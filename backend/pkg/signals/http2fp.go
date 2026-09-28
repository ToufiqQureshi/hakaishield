package signals

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// HTTP/2 fingerprinting reads the connection preface a client sends
// before its first request: the SETTINGS frame, any connection-level
// WINDOW_UPDATE, and the greeting's PRIORITY frames. Browsers, HTTP
// libraries and impersonation clients each greet differently, and unlike
// headers these values come from the client's HTTP/2 stack — application
// code cannot set them the way it sets a User-Agent.
//
// The output is the Akamai fingerprint string:
//
//	S1:V1;S2:V2|WU|P1:1:EX:DEP:W,...|PS
//
// which is the format the public HTTP/2 fingerprint databases publish,
// so a maintained feed can be loaded without conversion.
//
// The parser is deliberately read-only and bounded: it consumes only the
// greeting (preface -> SETTINGS -> WINDOW_UPDATE -> PRIORITY, ending at
// the first HEADERS frame), never request data, and refuses lengths past
// its cap. A malformed greeting yields no fingerprint, never an error to
// the visitor: fingerprinting must not be able to break serving.

const (
	// http2ClientPreface is the 24-byte magic every HTTP/2 client sends first.
	http2ClientPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

	// maxGreetingFrames bounds one greeting read. A real client sends a
	// handful of frames here; anything near the cap is not a browser
	// greeting and stops being fingerprinted rather than holding the
	// connection.
	maxGreetingFrames = 16

	// maxH2FrameLen is the largest frame payload the parser accepts. The
	// HTTP/2 default SETTINGS_MAX_FRAME_SIZE is 16384 and the greeting
	// never carries anything near it.
	maxH2FrameLen = 1 << 14
)

// errBadGreeting marks input that is not a usable HTTP/2 greeting.
var errBadGreeting = errors.New("http2fp: malformed or truncated greeting")

// HTTP2Priority is one greeting PRIORITY frame, in Akamai render order.
// Weight is the wire value plus one (RFC 7540 section 5.3.2), matching
// the published fingerprint format.
type HTTP2Priority struct {
	StreamID  uint32
	Exclusive bool
	StreamDep uint32
	Weight    uint32
}

// HTTP2Greeting is the parsed connection preface.
type HTTP2Greeting struct {
	Settings     []http2.Setting
	WindowUpdate uint32
	Priorities   []HTTP2Priority
	// PseudoHeaders is the pseudo-header order of the first request's
	// HEADERS frame (for example ["method", "scheme", "authority",
	// "path"]), captured when the greeting ends at that frame.
	PseudoHeaders []string
}

// FingerprintHTTP2Greeting reads one greeting from r and renders it in
// Akamai format. It reads only up to the first non-greeting frame; the
// caller owns the connection afterwards and any bytes it consumed are
// exactly the preface frames it needed.
func FingerprintHTTP2Greeting(r io.Reader) (string, error) {
	preface := make([]byte, len(http2ClientPreface))
	if _, err := io.ReadFull(r, preface); err != nil {
		return "", errBadGreeting
	}
	if string(preface) != http2ClientPreface {
		return "", errBadGreeting
	}

	g := &HTTP2Greeting{}
	for i := 0; i < maxGreetingFrames; i++ {
		f, err := readGreetingFrame(r)
		if errors.Is(err, io.EOF) {
			// Input exhausted between frames: the greeting is simply
			// over. This is the normal end for a capture of a client
			// that sends its request on another stream.
			return g.Fingerprint(), nil
		}
		if err != nil {
			return "", err
		}
		switch f.typ {
		case http2.FrameSettings:
			// An ACK (empty, flag 0x1) answers our own preface; it
			// says nothing about the client's stack, so skip it.
			if f.flags&0x1 != 0 && f.length == 0 {
				continue
			}
			if f.stream != 0 || f.length%6 != 0 {
				return "", errBadGreeting
			}
			for p := f.payload; len(p) >= 6; p = p[6:] {
				g.Settings = append(g.Settings, http2.Setting{
					ID:  http2.SettingID(binary.BigEndian.Uint16(p[:2])),
					Val: binary.BigEndian.Uint32(p[2:6]),
				})
			}
		case http2.FrameWindowUpdate:
			if f.length != 4 {
				return "", errBadGreeting
			}
			inc := binary.BigEndian.Uint32(f.payload) & 0x7fffffff
			if f.stream != 0 {
				// The format records the connection-level increment
				// only; a stream-scoped update ends the greeting.
				return g.Fingerprint(), nil
			}
			g.WindowUpdate = inc
		case http2.FramePriority:
			if f.length != 5 {
				return "", errBadGreeting
			}
			g.Priorities = append(g.Priorities, HTTP2Priority{
				StreamID:  f.stream,
				Exclusive: f.payload[0]&0x80 != 0,
				StreamDep: binary.BigEndian.Uint32(f.payload[:4]) & 0x7fffffff,
				Weight:    uint32(f.payload[4]) + 1,
			})
		case http2.FrameHeaders:
			// The first request has begun. Record its pseudo-header
			// order when the header block is complete in this frame,
			// then stop: everything after is request traffic.
			if f.flags&0x4 != 0 {
				g.PseudoHeaders = decodePseudoNames(headersBlock(f))
			}
			return g.Fingerprint(), nil
		case http2.FramePing:
			// Some stacks ping before the first request; it carries no
			// fingerprint value, so keep reading.
			continue
		default:
			// DATA, RST_STREAM, GOAWAY, ...: greeting over.
			return g.Fingerprint(), nil
		}
	}
	// The cap ran out without a request starting. Fingerprint what
	// arrived and stop reading; the connection still gets served.
	return g.Fingerprint(), nil
}

// headersBlock strips the priority fields of a HEADERS frame that
// carries the PRIORITY flag (0x20), returning the HPACK block.
func headersBlock(f *greetingFrame) []byte {
	if f.flags&0x20 == 0 || len(f.payload) < 5 {
		return f.payload
	}
	return f.payload[5:]
}

// decodePseudoNames decodes a complete HPACK header block and returns the
// pseudo-header names in wire order. A decode failure or a block with no
// pseudo headers yields nil: header order is an extra section, never a
// reason to lose the rest of the fingerprint.
func decodePseudoNames(block []byte) []string {
	if len(block) == 0 || len(block) > maxH2FrameLen {
		return nil
	}
	var names []string
	dec := hpack.NewDecoder(4096, func(hf hpack.HeaderField) {
		if len(hf.Name) >= 2 && hf.Name[0] == ':' && len(names) < 8 {
			names = append(names, hf.Name[1:2])
		}
	})
	if _, err := dec.Write(block); err != nil {
		return nil
	}
	return names
}

// greetingFrame is one decoded frame: the fixed 9-byte header of RFC 7540
// section 4.1 plus its payload.
type greetingFrame struct {
	typ     http2.FrameType
	flags   uint8
	length  int
	stream  uint32
	payload []byte
}

// readGreetingFrame reads exactly one frame, refusing lengths past the
// cap and truncated payloads. io.EOF (clean, zero bytes of the header
// read) means the input ended between frames; every other error means
// the greeting is malformed or truncated.
func readGreetingFrame(r io.Reader) (*greetingFrame, error) {
	var hdr [9]byte
	n, err := io.ReadFull(r, hdr[:])
	if err != nil {
		if err == io.EOF && n == 0 {
			return nil, io.EOF
		}
		return nil, errBadGreeting
	}
	length := int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2])
	if length > maxH2FrameLen {
		return nil, errBadGreeting
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		// A declared length followed by fewer bytes is truncation.
		return nil, errBadGreeting
	}
	return &greetingFrame{
		typ:     http2.FrameType(hdr[3]),
		flags:   hdr[4],
		length:  length,
		stream:  binary.BigEndian.Uint32(hdr[5:9]) & 0x7fffffff,
		payload: payload,
	}, nil
}

// Fingerprint renders the Akamai-format string "S|WU|P|PS" for this
// greeting.
func (g *HTTP2Greeting) Fingerprint() string {
	if g == nil {
		return ""
	}
	var b strings.Builder
	for i, s := range g.Settings {
		if i != 0 {
			b.WriteByte(';')
		}
		b.WriteString(strconv.FormatUint(uint64(s.ID), 10))
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(uint64(s.Val), 10))
	}
	b.WriteByte('|')
	b.WriteString(strconv.FormatUint(uint64(g.WindowUpdate), 10))
	b.WriteByte('|')
	for i, p := range g.Priorities {
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatUint(uint64(p.StreamID), 10))
		b.WriteByte(':')
		if p.Exclusive {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(uint64(p.StreamDep), 10))
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(uint64(p.Weight), 10))
	}
	b.WriteByte('|')
	for _, n := range g.PseudoHeaders {
		b.WriteString(n)
	}
	return b.String()
}

// Verified H/2 tool fingerprints, same rule as badJA4Hashes: every entry
// must come from a captured connection of the named tool, and an entry
// shared with a real browser family must never ship. Seeded from local
// verified captures; browser greetings arrive through the maintained
// Redis feed rather than being guessed at here.
var badHTTP2Fingerprints = map[string]string{
	// Verified capture: Go x/net/http2 Transport as configured by
	// net/http (Go 1.26), re-verified against this repo's end-to-end
	// test: the test client's greeting is exactly
	// "2:0;4:4194304;5:16384;6:10485760|1073741824||amps". Older Go
	// stacks greet differently; add captures as they are observed.
	"2:0;4:4194304;5:16384;6:10485760|1073741824||amps": "go-http2",
	// Verified capture: curl 8.x with nghttp2 (SETTINGS 1:65536;
	// 3:1000;4:6291456;2:0, no WINDOW_UPDATE, no PRIORITY, first
	// request :method :scheme :authority :path).
	"1:65536;3:1000;4:6291456;2:0|0||msap": "curl-nghttp2",
}

const http2ToolRedisKey = "h2:tools"

// h2FingerprintMax caps the in-memory H2 tool map for the same reason
// maxScraperJA4s caps the JA4 one: the list is operator/Redis fed, and a
// runaway writer must not be copied into every node's heap on sync.
const h2FingerprintMax = 100_000

var (
	h2Mu        sync.RWMutex
	h2ToolNames = badHTTP2Fingerprints
)

// IsKnownToolHTTP2 reports whether the fingerprint belongs to a captured
// automation stack, and names it. An empty fingerprint is never a tool.
func IsKnownToolHTTP2(fp string) (bool, string) {
	if fp == "" {
		return false, ""
	}
	h2Mu.RLock()
	defer h2Mu.RUnlock()
	tool, ok := h2ToolNames[fp]
	return ok, tool
}

// AddKnownToolHTTP2 registers a verified capture. Primarily for tests and
// operator tooling; the normal path is the Redis feed.
func AddKnownToolHTTP2(fp, tool string) {
	if fp == "" {
		return
	}
	h2Mu.Lock()
	defer h2Mu.Unlock()
	if _, exists := h2ToolNames[fp]; !exists && len(h2ToolNames) >= h2FingerprintMax {
		return
	}
	h2ToolNames[fp] = tool
}

// StartHTTP2ToolSync mirrors StartJA4Sync: load once, then poll Redis so
// every node converges on the same maintained feed without a request-path
// lookup. A Redis error keeps the last good map.
func StartHTTP2ToolSync(ctx context.Context, rdb *redis.Client) {
	if rdb == nil {
		return
	}
	syncHTTP2Tools(ctx, rdb)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncHTTP2Tools(ctx, rdb)
			}
		}
	}()
}

func syncHTTP2Tools(ctx context.Context, rdb *redis.Client) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	fps, err := rdb.HGetAll(timeoutCtx, http2ToolRedisKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Printf("hakaishield: error syncing h2 tools from redis: %v", err)
		return
	}
	if len(fps) > h2FingerprintMax {
		log.Printf("hakaishield: redis %s has %d entries, over the %d cap; keeping the existing list",
			http2ToolRedisKey, len(fps), h2FingerprintMax)
		return
	}
	h2Mu.Lock()
	defer h2Mu.Unlock()
	merged := make(map[string]string, len(fps)+len(badHTTP2Fingerprints))
	for fp, tool := range badHTTP2Fingerprints {
		merged[fp] = tool
	}
	for fp, tool := range fps {
		merged[fp] = tool
	}
	h2ToolNames = merged
}
