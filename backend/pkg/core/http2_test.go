package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestHTTP2FromContextEmpty(t *testing.T) {
	if got := HTTP2FromContext(context.Background()); got != "" {
		t.Fatalf("HTTP2FromContext(background) = %q, want empty", got)
	}
	const fp = "1:1|0||"
	if got := HTTP2FromContext(context.WithValue(context.Background(), ctxKeyHTTP2{}, fp)); got != fp {
		t.Fatalf("HTTP2FromContext(stored) = %q, want %q", got, fp)
	}
}

func TestReplayConnServesCapturedBytesThenLive(t *testing.T) {
	live := bytes.NewBufferString("LIVESTREAM")
	rc := &replayConn{
		replay: bytes.NewBufferString("PREFACE"),
		Conn:   &readerConn{r: live},
	}
	got := make([]byte, 16)
	n, err := io.ReadFull(rc, got[:7])
	if err != nil || string(got[:n]) != "PREFACE" {
		t.Fatalf("replay read = %q, %v", got[:n], err)
	}
	n, err = io.ReadFull(rc, got[:10])
	if err != nil || string(got[:n]) != "LIVESTREAM" {
		t.Fatalf("live read = %q, %v", got[:n], err)
	}
}

// readerConn is a net.Conn that only supports reading, enough for the
// replay test.
type readerConn struct{ r io.Reader }

func (c *readerConn) Read(p []byte) (int, error)       { return c.r.Read(p) }
func (c *readerConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *readerConn) Close() error                     { return nil }
func (c *readerConn) LocalAddr() net.Addr              { return nil }
func (c *readerConn) RemoteAddr() net.Addr             { return nil }
func (c *readerConn) SetDeadline(time.Time) error      { return nil }
func (c *readerConn) SetReadDeadline(time.Time) error  { return nil }
func (c *readerConn) SetWriteDeadline(time.Time) error { return nil }
func (c *readerConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{}
}

// TestWireHTTP2ServeHTTP2OverTLS drives a real HTTP/2-over-TLS client
// through a capture listener wired with WireHTTP2, asserting the handler
// saw both fingerprints. This is the end-to-end proof that greeting
// fingerprinting does not disturb serving.
func TestWireHTTP2ServeHTTP2OverTLS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-JA4", JA4FromContext(r.Context()))
		w.Header().Set("X-Seen-H2", HTTP2FromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Handler: handler, ConnContext: ConnContext}
	if err := WireHTTP2(srv); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	ln = NewCaptureListener(ln, &tls.Config{
		Certificates: []tls.Certificate{localhostCert(t)},
		NextProtos:   []string{"h2", "http/1.1"},
		MinVersion:   tls.VersionTLS12,
	})
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	addr := ln.Addr().String()

	// The Go HTTP/2 client, whose greeting the signals seed recognizes
	// as the go-http2 tool capture.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // self-signed test cert
			ForceAttemptHTTP2: true,
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://" + addr + "/")
	if err != nil {
		t.Fatalf("h2 request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.ProtoMajor != 2 {
		t.Fatalf("negotiated HTTP/%d, want 2", resp.ProtoMajor)
	}
	h2 := resp.Header.Get("X-Seen-H2")
	if h2 == "" {
		t.Fatal("handler saw no HTTP/2 fingerprint; context wiring broken")
	}
	if ok, tool := signals.IsKnownToolHTTP2(h2); !ok || tool != "go-http2" {
		t.Fatalf("Go client greeting %q not recognized as go-http2 (ok=%v tool=%q)", h2, ok, tool)
	}
	if ja4 := resp.Header.Get("X-Seen-JA4"); ja4 == "" || ja4 == signals.JA4Unreadable {
		t.Fatalf("JA4 lost over the h2 path: %q", ja4)
	}

	// HTTP/1.1 through the same listener keeps working.
	tlsConn, err := tls.Dial("tcp", addr, &tls.Config{
		InsecureSkipVerify: true, // self-signed test cert
		NextProtos:         []string{"http/1.1"},
	})
	if err != nil {
		t.Fatalf("tls dial http/1.1: %v", err)
	}
	defer tlsConn.Close()
	if got := tlsConn.ConnectionState().NegotiatedProtocol; got != "http/1.1" {
		t.Fatalf("negotiated %q, want http/1.1", got)
	}
	req := "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	if _, err := tlsConn.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	br := bufio.NewReader(tlsConn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !bytes.Contains([]byte(status), []byte(" 200 ")) {
		t.Fatalf("http/1.1 status = %q, want 200", status)
	}
}

func TestWireHTTP2TimesOutIdleClientPreface(t *testing.T) {
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		ReadHeaderTimeout: 200 * time.Millisecond,
	}
	if err := WireHTTP2(srv); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ln = NewCaptureListener(ln, &tls.Config{
		Certificates: []tls.Certificate{localhostCert(t)},
		NextProtos:   []string{"h2"},
		MinVersion:   tls.VersionTLS12,
	})
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	client, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		InsecureSkipVerify: true, // self-signed test cert
		NextProtos:         []string{"h2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, client); err != nil {
		t.Fatalf("idle preface connection stayed open past ReadHeaderTimeout: %v", err)
	}
}

// TestWireHTTP2ServesSlowPreconnect covers a browser preconnect: the
// client sends its preface and SETTINGS, then its first request only after
// the preface-read timeout. The connection must still be served, not
// closed, or every preconnect costs a visitor a new TLS handshake.
func TestWireHTTP2ServesSlowPreconnect(t *testing.T) {
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }),
		ConnContext:       ConnContext,
		ReadHeaderTimeout: 200 * time.Millisecond,
	}
	if err := WireHTTP2(srv); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln = NewCaptureListener(ln, &tls.Config{
		Certificates: []tls.Certificate{localhostCert(t)},
		MinVersion:   tls.VersionTLS12,
	})
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		InsecureSkipVerify: true, // self-signed test cert
		NextProtos:         []string{"h2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(http2.ClientPreface)); err != nil {
		t.Fatal(err)
	}
	fr := http2.NewFramer(conn, conn)
	if err := fr.WriteSettings(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(3 * srv.ReadHeaderTimeout) // idle past the preface-read deadline

	var block bytes.Buffer
	enc := hpack.NewEncoder(&block)
	for _, f := range []hpack.HeaderField{
		{Name: ":method", Value: "GET"}, {Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: "localhost"}, {Name: ":path", Value: "/"},
	} {
		if err := enc.WriteField(f); err != nil {
			t.Fatal(err)
		}
	}
	if err := fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID: 1, BlockFragment: block.Bytes(), EndStream: true, EndHeaders: true,
	}); err != nil {
		t.Fatalf("connection closed before the late request: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var status string
	dec := hpack.NewDecoder(4096, func(f hpack.HeaderField) {
		if f.Name == ":status" {
			status = f.Value
		}
	})
	for status == "" {
		f, err := fr.ReadFrame()
		if err != nil {
			t.Fatalf("no response to the late request: %v", err)
		}
		if h, ok := f.(*http2.HeadersFrame); ok && h.StreamID == 1 {
			if _, err := dec.Write(h.HeaderBlockFragment()); err != nil {
				t.Fatal(err)
			}
		}
	}
	if status != "418" {
		t.Fatalf(":status = %q, want 418 from the handler", status)
	}
}

// TestWireHTTP2GracefulShutdown proves srv.Shutdown reaches open h2
// connections. Without the shared, ConfigureServer-registered
// http2.Server an idle h2 connection stays "active" to net/http and
// Shutdown waits for its full deadline on every deploy.
func TestWireHTTP2GracefulShutdown(t *testing.T) {
	srv := &http.Server{
		Handler:     http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		ConnContext: ConnContext,
	}
	if err := WireHTTP2(srv); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln = NewCaptureListener(ln, &tls.Config{
		Certificates: []tls.Certificate{localhostCert(t)},
		MinVersion:   tls.VersionTLS12,
	})
	go func() { _ = srv.Serve(ln) }()

	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // self-signed test cert
		ForceAttemptHTTP2: true,
	}
	defer tr.CloseIdleConnections()
	resp, err := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Get("https://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("h2 request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.ProtoMajor != 2 {
		t.Fatalf("negotiated HTTP/%d, want 2", resp.ProtoMajor)
	}

	// The client keeps its idle h2 connection open. Shutdown must close
	// it with GOAWAY rather than wait out the deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown with an open h2 connection: %v after %v", err, time.Since(start))
	}
}

func TestFingerprintHTTP2ConnMalformedGreeting(t *testing.T) {
	// Even rejected bytes must be replayed: the HTTP/2 server, not the
	// fingerprint parser, owns the decision to reject a connection.
	input := []byte("NOT A PREFACE AT ALL............")
	fp, replay := fingerprintHTTP2Conn(bytes.NewReader(input))
	if fp != "" {
		t.Fatalf("fp = %q, want empty", fp)
	}
	if replay == nil || !bytes.Equal(replay.Bytes(), input[:24]) {
		t.Fatalf("replay = %v, want the consumed preface bytes", replay)
	}
	// A truncated preface behaves the same.
	input = []byte("PRI * HTTP/2.0\r")
	fp, replay = fingerprintHTTP2Conn(bytes.NewReader(input))
	if fp != "" || replay == nil || !bytes.Equal(replay.Bytes(), input) {
		t.Fatalf("truncated preface: fp=%q replay=%v, want empty and full replay", fp, replay)
	}
}

func localhostCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	return cert
}
