package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"golang.org/x/net/http2"
)

// HTTP/2 support. The capture listener advertises h2 again now that the
// client greeting can be fingerprinted: net/http dispatches a negotiated
// h2 connection to TLSNextProto["h2"], which reads the greeting, stores
// the fingerprint, and hands the connection to the maintained
// golang.org/x/net/http2 server so every existing handler (guard,
// challenge, dashboard API) works unchanged.
//
// The fingerprint travels in the connection's base context, the same
// mechanism the JA4 capture uses, so request handlers read it with
// HTTP2FromContext.

type ctxKeyHTTP2 struct{}

// HTTP2FromContext returns the HTTP/2 greeting fingerprint of the
// connection this request arrived on, or "" when the request was not
// HTTP/2 or the greeting could not be read. An empty value never blocks
// anything on its own, mirroring JA4FromContext.
func HTTP2FromContext(ctx context.Context) string {
	if fp, ok := ctx.Value(ctxKeyHTTP2{}).(string); ok {
		return fp
	}
	return ""
}

// fingerprintHTTP2Conn reads the client's HTTP/2 greeting from the
// freshly negotiated connection, returning the fingerprint and the bytes
// consumed so the HTTP/2 server can be replayed them. It must run before
// the server reads the same bytes.
//
// A malformed or truncated greeting costs the client its fingerprint and
// nothing else: the connection is still served, and the visitor sees no
// error from this path.
func fingerprintHTTP2Conn(r io.Reader) (string, *bytes.Buffer) {
	var buf bytes.Buffer
	fp, err := signals.FingerprintHTTP2Greeting(io.TeeReader(r, &buf))
	if err != nil {
		observability.Inc("http2_greeting_unreadable_total")
		// Replay even on parse errors. The fingerprint parser can be
		// narrower than the HTTP/2 server; only the server should decide
		// whether a connection is valid.
		return "", &buf
	}
	observability.Inc("http2_greeting_captured_total")
	return fp, &buf
}

// replayConn replays the bytes consumed during fingerprinting before
// reading live from the connection, so the HTTP/2 server sees the full
// preface exactly once. It forwards ConnectionState so the server keeps
// verifying the TLS version/cipher requirements of RFC 9.2 and requests
// keep their req.TLS state (SNI, TLS version).
type replayConn struct {
	net.Conn
	replay *bytes.Buffer
}

func (c *replayConn) Read(p []byte) (int, error) {
	if c.replay != nil && c.replay.Len() > 0 {
		return c.replay.Read(p)
	}
	c.replay = nil
	return c.Conn.Read(p)
}

func (c *replayConn) ConnectionState() tls.ConnectionState {
	if tc, ok := c.Conn.(*tls.Conn); ok {
		return tc.ConnectionState()
	}
	return tls.ConnectionState{}
}

// WireHTTP2 installs the TLSNextProto handler. Call once during setup,
// before the server starts accepting.
//
// One http2.Server is shared by every connection and registered through
// http2.ConfigureServer, so srv.Shutdown sends GOAWAY to open h2
// connections and waits for their in-flight requests. A per-connection
// http2.Server is invisible to Shutdown and holds it until its timeout.
func WireHTTP2(srv *http.Server) error {
	h2 := &http2.Server{ //nolint:staticcheck // ServeTLS would re-run TLS and destroy the ClientHello capture
		// Explicit because they bound per-connection memory: a
		// browser opens ~100 streams for assets; more than 250
		// concurrent streams on one connection is not a browser.
		MaxConcurrentStreams: 250,
		MaxReadFrameSize:     1 << 20,
	}
	// ConfigureServer registers the graceful-shutdown hook and installs
	// its own "h2" handler, which is replaced below with one that reads
	// the greeting first. It also fills srv.TLSConfig, which is unused
	// here: TLS is terminated by the capture listener's own config.
	if err := http2.ConfigureServer(srv, h2); err != nil { //nolint:staticcheck // see above
		return err
	}
	srv.TLSNextProto["h2"] = func(hs *http.Server, tlsConn *tls.Conn, handler http.Handler) {
		// The connection is ours to close when serving ends; a close
		// error on an already-dead TLS connection says nothing useful.
		defer func() { _ = tlsConn.Close() }()
		// The stdlib clears its TLS-handshake deadline before calling
		// TLSNextProto. Bound this preface read separately; otherwise an
		// idle h2 client can hold a server goroutine indefinitely.
		prefaceTimeout := hs.ReadHeaderTimeout
		if prefaceTimeout <= 0 {
			prefaceTimeout = hs.ReadTimeout
		}
		if prefaceTimeout <= 0 {
			prefaceTimeout = 10 * time.Second
		}
		deadline := time.Now().Add(prefaceTimeout)
		if err := tlsConn.SetReadDeadline(deadline); err != nil {
			return
		}
		fp, replay := fingerprintHTTP2Conn(tlsConn)
		// A client that never finished the 24-byte preface in time is
		// dropped here. One that sent it but no request yet (a browser
		// preconnect) is served; the h2 server's IdleTimeout bounds it.
		// Closing those would cost real visitors a fresh TLS handshake.
		if replay.Len() < len(http2.ClientPreface) {
			return
		}
		if err := tlsConn.SetReadDeadline(time.Time{}); err != nil {
			return
		}

		// Every h2 request context derives from this base. net/http
		// passes its per-connection context (already carrying the
		// *tls.Conn from ConnContext, so JA4FromContext keeps working)
		// through the handler's BaseContext method, the same way
		// http2.ConfigureServer's own handler reads it.
		base := context.Background()
		if bc, ok := handler.(interface{ BaseContext() context.Context }); ok {
			base = bc.BaseContext()
		}
		if _, ok := base.Value(ctxKeyConn{}).(*tls.Conn); !ok {
			base = context.WithValue(base, ctxKeyConn{}, tlsConn)
		}
		if fp != "" {
			base = context.WithValue(base, ctxKeyHTTP2{}, fp)
		}

		// http2.ServeConn is marked deprecated in favour of
		// http.Server.ServeTLS, but ServeTLS would run its own TLS
		// handshake and destroy the ClientHello capture this product
		// exists for. Serving the already-negotiated connection through
		// TLSNextProto is the mechanism net/http itself uses.
		opts := &http2.ServeConnOpts{ //nolint:staticcheck // only supported option shape for an already-negotiated conn
			BaseConfig: hs,
			Handler:    handler,
			Context:    base,
		}
		// Blocks for the life of the connection, which is the
		// TLSNextProto contract.
		h2.ServeConn(&replayConn{Conn: tlsConn, replay: replay}, opts) //nolint:staticcheck // see above
	}
	return nil
}
