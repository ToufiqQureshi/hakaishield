package signals

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
)

// h2Frame builds one HTTP/2 frame: 9-byte header (RFC 7540 §4.1) plus
// payload.
func h2Frame(typ http2.FrameType, flags uint8, stream uint32, payload []byte) []byte {
	out := make([]byte, 9, 9+len(payload))
	out[0] = byte(len(payload) >> 16)
	out[1] = byte(len(payload) >> 8)
	out[2] = byte(len(payload))
	out[3] = byte(typ)
	out[4] = flags
	binary.BigEndian.PutUint32(out[5:9], stream)
	return append(out, payload...)
}

func h2Setting(id http2.SettingID, val uint32) []byte {
	b := make([]byte, 6)
	binary.BigEndian.PutUint16(b[:2], uint16(id))
	binary.BigEndian.PutUint32(b[2:6], val)
	return b
}

// h2Priority encodes one PRIORITY payload: the exclusive bit belongs to
// the dependency, while the full final byte holds weight minus one.
func h2Priority(stream uint32, exclusive bool, dep uint32, weight uint16) []byte {
	b := make([]byte, 5)
	binary.BigEndian.PutUint32(b[:4], dep)
	if exclusive {
		b[0] |= 0x80
	}
	b[4] = byte(weight - 1)
	return b
}

// goGreeting reconstructs the Go Transport greeting with configurable
// pieces so tests can prove the parser reads what was actually written.
// goGreeting reconstructs the verified Go Transport capture with
// configurable extra frames so tests can prove the parser reads what was
// actually written. The base shape matches the seed entry in http2fp.go:
// SETTINGS 2:0;4:4194304;5:16384;6:10485760, WINDOW_UPDATE 1073741824,
// then the caller's frames (PRIORITY, or a HEADERS ending the greeting).
func goGreeting(t *testing.T, extra ...[]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString(http2ClientPreface)
	b.Write(h2Frame(http2.FrameSettings, 0, 0, append(
		append(h2Setting(2, 0), h2Setting(4, 4194304)...),
		append(h2Setting(5, 16384), h2Setting(6, 10485760)...)...,
	)))
	b.Write(h2Frame(http2.FrameWindowUpdate, 0, 0, func() []byte {
		p := make([]byte, 4)
		binary.BigEndian.PutUint32(p, 1073741824)
		return p
	}()))
	for _, e := range extra {
		b.Write(e)
	}
	return b.Bytes()
}

func TestFingerprintHTTP2GreetingGoShape(t *testing.T) {
	// The Go Transport capture ends with its first request's HEADERS
	// frame carrying the pseudo-header order :method :scheme
	// :authority :path.
	headers := h2Frame(http2.FrameHeaders, 0x4 /*END_HEADERS*/, 1, []byte{0x82, 0x86, 0x81, 0x84})
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(append(goGreeting(t), headers...)))
	if err != nil {
		t.Fatalf("greeting rejected: %v", err)
	}
	// The shape must match the seeded verified capture exactly: the
	// greeting proper ends before the first request's HEADERS, which
	// supplies the pseudo-header section in the full capture.
	want := "2:0;4:4194304;5:16384;6:10485760|1073741824||msap"
	if fp != want {
		t.Fatalf("fingerprint = %q, want %q", fp, want)
	}
}

func TestFingerprintHTTP2GreetingPrioritySection(t *testing.T) {
	// Exclusive and weight occupy separate wire fields; all 256 weight
	// values remain possible with either exclusive value.
	prio := h2Frame(http2.FramePriority, 0, 3, h2Priority(3, true, 0, 128))
	prio2 := h2Frame(http2.FramePriority, 0, 5, h2Priority(5, false, 3, 256))
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(append(goGreeting(t), append(prio, prio2...)...)))
	if err != nil {
		t.Fatalf("greeting rejected: %v", err)
	}
	want := "2:0;4:4194304;5:16384;6:10485760|1073741824|3:1:0:128,5:0:3:256|"
	if fp != want {
		t.Fatalf("fingerprint = %q, want %q", fp, want)
	}
}

func TestFingerprintHTTP2GreetingEndsAtHeadersWithPseudoOrder(t *testing.T) {
	// A HEADERS frame carrying a complete HPACK block ends the greeting
	// and records pseudo-header order from the HPACK static-table
	// indexes: :method (2), :scheme https (6), :authority (1), :path (4).
	block := []byte{0x82, 0x86, 0x81, 0x84}
	hdrs := h2Frame(http2.FrameHeaders, 0x4 /*END_HEADERS*/, 1, block)
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(append(goGreeting(t), hdrs...)))
	if err != nil {
		t.Fatalf("greeting rejected: %v", err)
	}
	if !strings.HasSuffix(fp, "|msap") {
		t.Fatalf("fingerprint = %q, want pseudo section \"msap\"", fp)
	}
}

func TestFingerprintHTTP2GreetingAcceptsEmptySettings(t *testing.T) {
	in := append([]byte(http2ClientPreface), h2Frame(http2.FrameSettings, 0, 0, nil)...)
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(in))
	if err != nil || fp != "|0||" {
		t.Fatalf("empty SETTINGS greeting: fp=%q err=%v, want |0||", fp, err)
	}
}

func TestFingerprintHTTP2GreetingIgnoresStreamWindowUpdate(t *testing.T) {
	in := append([]byte(http2ClientPreface), h2Frame(http2.FrameSettings, 0, 0, nil)...)
	in = append(in, h2Frame(http2.FrameWindowUpdate, 0, 0, []byte{0, 0, 0, 7})...)
	in = append(in, h2Frame(http2.FrameWindowUpdate, 0, 1, []byte{0, 0, 0, 9})...)
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(in))
	if err != nil || fp != "|7||" {
		t.Fatalf("stream window update: fp=%q err=%v, want |7||", fp, err)
	}
}

func TestFingerprintHTTP2GreetingRejectsBadInput(t *testing.T) {
	cases := map[string][]byte{
		"empty":            {},
		"short preface":    []byte(http2ClientPreface[:10]),
		"wrong preface":    append([]byte("GET / HTTP/1.1\r\n\r\n"), h2Frame(http2.FrameSettings, 0, 0, h2Setting(2, 0))...),
		"truncated frame":  goGreeting(t)[:len(http2ClientPreface)+5],
		"settings stream1": append([]byte(http2ClientPreface), h2Frame(http2.FrameSettings, 0, 1, h2Setting(2, 0))...),
		"settings not %6":  append([]byte(http2ClientPreface), h2Frame(http2.FrameSettings, 0, 0, []byte{1, 2, 3, 4, 5})...),
		"oversize frame": func() []byte {
			return append([]byte(http2ClientPreface), []byte{0x00, 0x40, 0x00, byte(http2.FrameSettings), 0, 0, 0, 0, 0}...)
		}(),
		"window_update_len3": append([]byte(http2ClientPreface), h2Frame(http2.FrameWindowUpdate, 0, 0, []byte{1, 2, 3})...),
		"priority_len4":      append([]byte(http2ClientPreface), h2Frame(http2.FramePriority, 0, 3, []byte{1, 2, 3, 4})...),
	}
	for name, in := range cases {
		fp, err := FingerprintHTTP2Greeting(bytes.NewReader(in))
		if err == nil {
			t.Errorf("%s: expected error, got fingerprint %q", name, fp)
		}
		if fp != "" {
			t.Errorf("%s: expected empty fingerprint on error, got %q", name, fp)
		}
	}
}

func TestFingerprintHTTP2GreetingTruncatedPayloadIsError(t *testing.T) {
	full := goGreeting(t)
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(full[:len(full)-2]))
	if err == nil || fp != "" {
		t.Fatalf("truncated greeting: got (%q, %v), want error", fp, err)
	}
}

// startHTTP2ToolSyncTest points StartHTTP2ToolSync at the given
// miniredis through the package-level rdb the same way velocity tests
// do, cleaning up the background poller when the test ends.
func startHTTP2ToolSyncTest(t *testing.T, mr *miniredis.Miniredis) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	prevAddr := rdb.Options().Addr
	rdb = newRedisClientAt(t, mr.Addr())
	t.Cleanup(func() { rdb = newRedisClientAt(t, prevAddr) })
	StartHTTP2ToolSync(ctx, rdb)
}

func TestIsKnownToolHTTP2SeededGo(t *testing.T) {
	// The exact fingerprint this repo's own test client produces must be
	// recognized: it is the verified capture the seed was built from.
	ok, tool := IsKnownToolHTTP2("2:0;4:4194304;5:16384;6:10485760|1073741824||amps")
	if !ok || tool != "go-http2" {
		t.Fatalf("Go greeting not recognized: ok=%v tool=%q", ok, tool)
	}
	if ok, _ := IsKnownToolHTTP2(""); ok {
		t.Fatal("empty fingerprint must never be a tool")
	}
	if ok, _ := IsKnownToolHTTP2("1:999|0||"); ok {
		t.Fatal("unknown fingerprint must not be a tool")
	}
}

func TestStartHTTP2ToolSyncMergesAndSurvivesRedisOutage(t *testing.T) {
	mr := miniredis.RunT(t)
	newTestRedis(t)
	startHTTP2ToolSyncTest(t, mr)

	if err := rdb.HSet(context.Background(), "h2:tools", map[string]interface{}{
		"9:9|0||": "evil-scrapestack",
	}).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	StartHTTP2ToolSync(context.Background(), rdb)

	if ok, tool := IsKnownToolHTTP2("9:9|0||"); !ok || tool != "evil-scrapestack" {
		t.Fatalf("redis entry not loaded: ok=%v tool=%q", ok, tool)
	}
	// Seeded captures survive the merge.
	if ok, _ := IsKnownToolHTTP2("2:0;4:4194304;5:16384;6:10485760|1073741824||amps"); !ok {
		t.Fatal("seeded Go fingerprint lost on sync")
	}

	// An outage after a good sync keeps the last map.
	mr.Close()
	StartHTTP2ToolSync(context.Background(), rdb)
	if ok, _ := IsKnownToolHTTP2("9:9|0||"); !ok {
		t.Fatal("redis outage wiped the last good tool map")
	}
}

func TestStartHTTP2ToolSyncOverCapKeepsExisting(t *testing.T) {
	mr := miniredis.RunT(t)
	newTestRedis(t)
	startHTTP2ToolSyncTest(t, mr)

	over := make(map[string]interface{}, h2FingerprintMax+1)
	for i := 0; i <= h2FingerprintMax; i++ {
		over[itoa(i)+"|0||"] = "tool"
	}
	if err := rdb.HSet(context.Background(), "h2:tools", over).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	StartHTTP2ToolSync(context.Background(), rdb)

	if ok, _ := IsKnownToolHTTP2("0|0||"); ok {
		t.Fatal("over-cap feed was loaded; must keep existing list")
	}
	if ok, _ := IsKnownToolHTTP2("2:0;4:4194304;5:16384;6:10485760|1073741824||amps"); !ok {
		t.Fatal("seeded fingerprint lost when refusing over-cap feed")
	}
}

func TestHTTP2GreetingFingerprintRenderNil(t *testing.T) {
	var g *HTTP2Greeting
	if s := g.Fingerprint(); s != "" {
		t.Fatalf("nil greeting fingerprint = %q, want empty", s)
	}
}

func TestHTTP2GreetingFrameCapStopsReading(t *testing.T) {
	// More than maxGreetingFrames SETTINGS frames without a request:
	// parsing must stop (return) rather than loop or read forever. The
	// result is whatever arrived — a fingerprint is optional.
	var b bytes.Buffer
	b.WriteString(http2ClientPreface)
	for i := 0; i < maxGreetingFrames+4; i++ {
		b.Write(h2Frame(http2.FrameSettings, 0, 0, h2Setting(http2.SettingID(i+8), uint32(i))))
	}
	fp, err := FingerprintHTTP2Greeting(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("cap run should not error: %v", err)
	}
	if strings.Count(fp, ";") < maxGreetingFrames-2 {
		t.Fatalf("expected most frames parsed before the cap, got %q", fp)
	}
}

// newRedisClientAt builds a client for a specific address; the tests
// swap the package-level rdb between miniredis instances and restore it
// afterwards, matching how the velocity tests manage the package var.
func newRedisClientAt(t *testing.T, addr string) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: addr})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
