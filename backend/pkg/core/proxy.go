// Package proxy is the reverse proxy that sits in front of a
// client's origin server. It is the integration point every
// detection layer (fingerprint, score, challenge) plugs into.
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/deception"
	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

type ctxKeyDecision struct{}
type ctxKeyScore struct{}
type ctxKeyClientIP struct{}

// WithClientIP carries the already-validated visitor identity from Guard to
// the origin proxy. The proxy must not re-parse an untrusted forwarding header
// after the scoring layer has established the trusted-proxy boundary.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyClientIP{}, ip)
}

// ClientIPFromContext returns the identity Guard resolved for this request.
func ClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(ctxKeyClientIP{}).(string)
	return ip
}

// WithDecision attaches the hakaishield policy decision and score to the request context.
func WithDecision(ctx context.Context, decision string, score int) context.Context {
	ctx = context.WithValue(ctx, ctxKeyDecision{}, decision)
	return context.WithValue(ctx, ctxKeyScore{}, score)
}

// DecisionFromContext retrieves the policy decision from context.
func DecisionFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(ctxKeyDecision{}).(string); ok {
		return s
	}
	return ""
}

// ScoreFromContext retrieves the bot risk score from context.
func ScoreFromContext(ctx context.Context) int {
	if s, ok := ctx.Value(ctxKeyScore{}).(int); ok {
		return s
	}
	return 0
}

// ja4Header is the header we attach to the forwarded request so the
// origin (and later, our own scoring code) can see the caller's JA4
// fingerprint without re-capturing it themselves.
const ja4Header = "X-HakaiShield-JA4"

// uaMismatchHeader tells the origin the caller's declared browser
// doesn't match its TLS handshake. Set to "true" only when we caught
// one — absence means nothing suspicious was found, not "unknown".
const uaMismatchHeader = "X-HakaiShield-UA-Mismatch"
const decisionHeader = "X-HakaiShield-Decision"
const scoreHeader = "X-HakaiShield-Score"
const signalsHeader = "X-HakaiShield-Signals"

// h2Header carries the visitor's HTTP/2 greeting fingerprint to the
// origin alongside the JA4 header, under the same set-by-us rule.
const h2Header = "X-HakaiShield-HTTP2"

// realIPHeader is the client-IP header we set ourselves. nginx, Rails
// and Laravel apps commonly read this one.
const realIPHeader = "X-Real-IP"

// clientIPHeaders are the "who is calling" headers a visitor could
// send to pass themselves off as another IP. net/http only strips the
// X-Forwarded-* family, so we clear these ourselves — an origin that
// trusts any one of them would otherwise log, allowlist or rate-limit
// an address the caller picked.
var clientIPHeaders = []string{
	realIPHeader,
	"True-Client-IP",
	"CF-Connecting-IP",
	"X-Client-IP",
	"Fastly-Client-IP",
	"X-Cluster-Client-IP",
}

// DefaultOriginTransport is a production-tuned HTTP transport with connection pooling and explicit timeouts.
var DefaultOriginTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          1000,
	MaxIdleConnsPerHost:   200,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 15 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func publicOriginTransport() *http.Transport {
	t := DefaultOriginTransport.Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok && blockedOriginIP(tcp.IP) {
			_ = conn.Close()
			return nil, fmt.Errorf("origin resolved to non-public address %s", tcp.IP.String())
		}
		return conn, nil
	}
	return t
}

// ValidatePublicOrigin rejects origins that would let a dashboard user make
// hakaishield fetch local/cloud-internal services. Hostnames are rechecked at
// dial time by NewPublicOriginProxy to cover DNS rebinding.
func ValidatePublicOrigin(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("origin scheme must be http or https")
	}
	if u.Host == "" {
		return errInvalidTarget
	}
	host := u.Hostname()
	if host == "" {
		return errInvalidTarget
	}
	if ip := net.ParseIP(host); ip != nil && blockedOriginIP(ip) {
		return fmt.Errorf("origin host %s is not public", ip.String())
	}
	return nil
}

func blockedOriginIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	ip = ip.To16()
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 0 || ip4[0] == 127 || ip4[0] >= 224 || (ip4[0] == 255 && ip4[1] == 255 && ip4[2] == 255 && ip4[3] == 255)
	}
	return false
}

// maxDeceptionBodyBytes caps how much of an origin response we will hold
// in memory to rewrite it. The old version of this hook called
// io.ReadAll on the origin body, which meant one deceived request for a
// large file pinned that whole file in heap — and enough of them at once
// would take the process down in front of every customer on the node.
// Real HTML documents sit an order of magnitude below this, and anything
// larger is streamed through untouched rather than buffered or
// truncated. The cap is what a chunked response (one that declares no
// length) is allowed to cost us, so it is deliberately not generous.
const maxDeceptionBodyBytes = 512 << 10 // 512 KiB

// initialDeceptionBufBytes is the starting buffer for a response that
// declares no length (a chunked one, which is what dynamic pages
// usually are). It comfortably holds a typical HTML document, so the
// common case is a single allocation well under the cap.
const initialDeceptionBufBytes = 64 << 10 // 64 KiB

// readBody fills buf from the response body. A short read is the end of
// the document and is not an error; a real failure closes the body and
// is returned, because a partially drained body can no longer be
// forwarded intact and the visitor must get ErrorHandler's page rather
// than a silently truncated one.
func readBody(resp *http.Response, buf []byte) (int, error) {
	n, err := io.ReadFull(resp.Body, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		_ = resp.Body.Close()
		return n, err
	}
	return n, nil
}

// deceiveResponse injects the deception payload into HTML served to
// traffic the guard decided to deceive. Every early return leaves the
// response exactly as the origin sent it: failing to poison a scraper
// is a missed opportunity, while corrupting a real response is an
// outage for the customer.
func deceiveResponse(resp *http.Response) error {
	if resp.Request == nil {
		return nil
	}
	if DecisionFromContext(resp.Request.Context()) != signals.DecisionDeceive.String() {
		return nil
	}
	// Only whole, uncompressed 200s. A 206 range or a 304 revalidation
	// carries no complete document to rewrite, and rewriting one would
	// break the client's reassembly or its cache.
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	if enc := resp.Header.Get("Content-Encoding"); enc != "" && enc != "identity" {
		return nil
	}
	// Check the declared type before touching the body: this is what
	// keeps a deceived request for a video or a ZIP from being read
	// into memory only for the payload injector to decline it.
	if !deception.IsInjectableContentType(resp.Header.Get("Content-Type")) {
		return nil
	}
	// A declared length past the cap lets us skip the read entirely.
	if resp.ContentLength > maxDeceptionBodyBytes {
		return nil
	}

	// Size the buffer from the declared length when the origin gave us
	// one, so an 80 KiB page costs 80 KiB and not the whole cap. A
	// chunked response declares nothing, so it starts at a realistic
	// page size and grows only if it has to.
	//
	// io.ReadAll is deliberately not used: it starts at 512 bytes and
	// doubles, which measured at roughly 10 MB of churn per over-cap
	// response — on a hook that runs in front of customer traffic.
	// Reading one byte past the cap is what makes an over-long body
	// detectable without buffering all of it.
	size := initialDeceptionBufBytes
	if resp.ContentLength >= 0 && resp.ContentLength <= maxDeceptionBodyBytes {
		size = int(resp.ContentLength) + 1
	}

	buf := make([]byte, size)
	n, err := readBody(resp, buf)
	if err != nil {
		return err
	}

	if n == size && size <= maxDeceptionBodyBytes {
		// Filled the starting buffer without reaching the cap: either a
		// chunked response larger than a typical page, or an origin that
		// under-declared its length. Grow once, straight to the cap,
		// rather than doubling our way there.
		grown := make([]byte, maxDeceptionBodyBytes+1)
		copy(grown, buf)
		more, err := readBody(resp, grown[n:])
		if err != nil {
			return err
		}
		buf, n, size = grown, n+more, maxDeceptionBodyBytes+1
	}
	body := buf[:n]

	if n == size {
		// Too large to rewrite: hand back what we read followed by the
		// rest of the stream, so the visitor still gets the real
		// response byte for byte.
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(body), resp.Body), resp.Body}
		return nil
	}
	_ = resp.Body.Close()

	transformed := deception.InjectPayload(body)

	resp.Body = io.NopCloser(bytes.NewReader(transformed))
	resp.ContentLength = int64(len(transformed))
	resp.Header.Set("Content-Length", strconv.Itoa(len(transformed)))
	// The body no longer matches the origin's validator, so leaving it
	// would let a cache or browser revalidate into the unpoisoned copy.
	resp.Header.Del("Etag")
	return nil
}

// NewOriginProxy sets up the HTTP proxy to the origin server.
// It forces TLS since a WAF that doesn't protect the origin connection
// is just security theater.
func NewOriginProxy(target string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		observability.Inc("origin_proxy_invalid_target_total")
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		observability.Inc("origin_proxy_invalid_target_total")
		return nil, &url.Error{Op: "parse", URL: target, Err: errInvalidTarget}
	}

	// Rewrite, not the older Director: net/http strips the client's
	// own X-Forwarded-* headers before calling it, so a visitor can't
	// fake the IP that our rate limiting and geo checks will rely on.
	// Director leaves those headers untouched.
	p := &httputil.ReverseProxy{
		Transport: DefaultOriginTransport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			observability.Inc("origin_proxy_error_total")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>502 Bad Gateway</title><style>body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#c9d1d9;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;}h1{font-size:2rem;color:#f85149;}p{color:#8b949e;}</style></head><body><div style="text-align:center;"><h1>502 Bad Gateway</h1><p>Origin server connection failed or timed out.</p><small style="color:#484f58;">Protected by HakaiShield</small></div></body></html>`))
		},
		ModifyResponse: deceiveResponse,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			// SetURL would point Host at the origin; the origin serves
			// the visitor's domain, so it needs the original Host.
			r.Out.Host = r.In.Host
			// Fills in the real client IP for the origin's logs.
			r.SetXForwarded()
			for _, h := range clientIPHeaders {
				r.Out.Header.Del(h)
			}
			ip := ClientIPFromContext(r.In.Context())
			if ip == "" {
				ip = canonicalIP(r.In.RemoteAddr)
			}
			if ip != "" {
				r.Out.Header.Set(realIPHeader, ip)
				r.Out.Header.Set("X-Forwarded-For", ip)
			}

			// Only we get to say what the fingerprint is. A request
			// with none is forwarded as normal — a fingerprint we
			// couldn't read is never a reason to block someone.
			r.Out.Header.Del(ja4Header)
			ja4 := JA4FromContext(r.In.Context())
			if ja4 != "" {
				r.Out.Header.Set(ja4Header, ja4)
			}
			r.Out.Header.Del(h2Header)
			if h2fp := HTTP2FromContext(r.In.Context()); h2fp != "" {
				r.Out.Header.Set(h2Header, h2fp)
			}

			// Same rule for the UA-mismatch flag: a visitor doesn't
			// get to set this themselves, and it's a signal for
			// scoring, not a block, per CLAUDE.md Section 6.
			r.Out.Header.Del(uaMismatchHeader)
			if signals.UAMismatch(r.In.UserAgent(), ja4) {
				r.Out.Header.Set(uaMismatchHeader, "true")
			}

			// Inbound requests cannot forge HakaiShield decision or score headers.
			r.Out.Header.Del(decisionHeader)
			r.Out.Header.Del(scoreHeader)
			r.Out.Header.Del(signalsHeader)
			if dec := DecisionFromContext(r.In.Context()); dec != "" {
				r.Out.Header.Set(decisionHeader, dec)
				r.Out.Header.Set(scoreHeader, strconv.Itoa(ScoreFromContext(r.In.Context())))
				if dec == signals.DecisionDeceive.String() {
					// Strip Accept-Encoding so origin returns uncompressed HTML that can be transformed.
					r.Out.Header.Del("Accept-Encoding")
				}
			}
		},
	}
	return p, nil
}

// NewPublicOriginProxy is used for customer-configured SaaS origins. It keeps
// local development flexible through NewOriginProxy while preventing dashboard
// tenants from turning the proxy into an internal-network fetcher.
func NewPublicOriginProxy(target string) (*httputil.ReverseProxy, error) {
	if err := ValidatePublicOrigin(target); err != nil {
		observability.Inc("origin_proxy_invalid_target_total")
		return nil, err
	}
	p, err := NewOriginProxy(target)
	if err != nil {
		return nil, err
	}
	p.Transport = publicOriginTransport()
	return p, nil
}
