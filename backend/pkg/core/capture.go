package core

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/wi1dcard/fingerproxy/pkg/hack"
)

type ctxKeyConn struct{}

// NewCaptureListener wraps a plain TCP listener so each connection
// keeps a copy of its raw TLS handshake. Go normally throws those
// bytes away as soon as the handshake finishes, and the JA4
// fingerprint can only be built from them.
func NewCaptureListener(inner net.Listener, tlsConfig *tls.Config) net.Listener {
	cfg := tlsConfig.Clone()
	// h2 is offered so HTTP/2 clients are served and their greeting
	// fingerprinted (WireHTTP2 installs the handler). HTTP/1.1 stays
	// listed so plain browsers and non-h2 tools keep working. See
	// docs/DECISIONS.md for why h2 was deferred and what changed.
	cfg.NextProtos = []string{"h2", "http/1.1"}
	return &captureListener{Listener: inner, config: cfg}
}

type captureListener struct {
	net.Listener
	config *tls.Config
}

// Accept hands back a real *tls.Conn without handshaking it here.
// That matters: net/http only recognises a real *tls.Conn, and when
// it does it runs the handshake itself, with its own timeout, error
// handling and connection limits — all of which we would otherwise
// have to rewrite, worse.
func (l *captureListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return tls.Server(hack.NewHijackClientHelloConn(conn), l.config), nil
}

// ConnContext gives each request a way to reach the connection it
// arrived on, which is where the saved handshake lives. Wire this to
// http.Server.ConnContext.
func ConnContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, ctxKeyConn{}, c)
}

type ctxKeyJA4 struct{}

// WithJA4 returns a context containing an explicit JA4 fingerprint.
func WithJA4(ctx context.Context, ja4 string) context.Context {
	return context.WithValue(ctx, ctxKeyJA4{}, ja4)
}

// signals.JA4Unreadable marks a TLS connection whose handshake we could not
// read. A normal client never causes this; a bot splitting its
// handshake across TLS records to dodge fingerprinting does. So it is
// a signal to score later, and must never look the same as a plain
// HTTP request that simply has no fingerprint.

// JA4FromContext returns the JA4 fingerprint of the connection this
// request came in on, signals.JA4Unreadable if it was TLS but we couldn't
// read it, or "" if it wasn't TLS at all. None of these block a
// request on their own — that's the scoring layer's job.
func JA4FromContext(ctx context.Context) string {
	if ja4, ok := ctx.Value(ctxKeyJA4{}).(string); ok && ja4 != "" {
		return ja4
	}
	conn, _ := ctx.Value(ctxKeyConn{}).(*tls.Conn)
	if conn == nil {
		return "" // plain HTTP, or capture isn't set up
	}
	hijacked, ok := conn.NetConn().(*hack.HijackClientHelloConn)
	if !ok {
		return ""
	}
	raw, err := hijacked.GetClientHello()
	if err != nil {
		return signals.JA4Unreadable
	}
	fp, err := signals.ParseJA4(raw)
	if err != nil {
		return signals.JA4Unreadable
	}
	return fp
}
