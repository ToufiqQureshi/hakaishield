package signals

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
)

// realBrowserHeaders is the header set every current real browser sends on
// a navigation: the Sec-Fetch-* fetch-metadata family plus a client-hint
// brand. A score test that intends "clean real browser" must pass these,
// or the header_anomaly check correctly fires on the missing headers.
func realBrowserHeaders() http.Header {
	h := http.Header{}
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-CH-UA", `"Chromium";v="120"`)
	return h
}

// facts builds a RequestFacts for a browser-claiming client with real
// browser headers, so only the signal under test varies.
func facts(ja4, ua string) RequestFacts {
	return RequestFacts{JA4: ja4, UA: ua, Header: realBrowserHeaders()}
}

func TestScoreNoSignals(t *testing.T) {
	// Real Chrome, modern TLS, real browser headers: nothing should fire.
	got := Score(facts("t13d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Chrome/120.0"))
	if got != 0 {
		t.Fatalf("Score() = %d, want 0", got)
	}
}

func TestEvaluateRunsStatefulChecksOnce(t *testing.T) {
	newTestRedis(t)
	// checkJA4VelocitySpike fails open until at least one common-browser
	// prefix is loaded (see docs/DECISIONS.md, "Audit P0 routing, JA4,
	// host-cache..."), so this test's JA4 counter assertion depended on
	// whichever earlier test in the package happened to leave
	// browserPrefixes populated — order-dependent and flaky in isolation
	// (confirmed: `go test -run TestEvaluateRunsStatefulChecksOnce` fails
	// on a fresh binary). Seed a prefix that does not match f.JA4 below,
	// so hasCommonBrowserPrefixes() is true and isCommonBrowserJA4(f.JA4)
	// stays false, deterministically, regardless of test run order.
	resetCommonBrowserPrefixes(t)
	AddCommonBrowserPrefix("t13d1516h2")

	f := RequestFacts{
		Tenant: "test-tenant",
		IP:     "203.0.113.20",
		JA4:    "t99d000000_deadbeefdead_deadbeefdead",
		UA:     "SomeUnknownClient/1.0",
		Header: http.Header{},
		Path:   "/",
	}
	window := time.Now().UnixMilli() / int64(rateLimitMs)
	evaluation := Evaluate(f)
	if evaluation.Score != 0 {
		t.Fatalf("Evaluate() score = %d, want 0", evaluation.Score)
	}
	if len(evaluation.Signals) != 0 {
		t.Fatalf("Evaluate() signals = %v, want none", evaluation.Signals)
	}

	ipKey, _ := velocityBucket(f.Tenant, f.IP, f.Path, f.Method, window)
	ipCount, err := rdb.Get(context.Background(), ipKey).Int64()
	if err != nil {
		t.Fatalf("read IP velocity counter: %v", err)
	}
	if ipCount != 1 {
		t.Fatalf("IP velocity counter = %d, want one increment", ipCount)
	}

	ja4Key := fmt.Sprintf("vel:t:%d:%s:ja4:%s:%d", len(f.Tenant), f.Tenant, f.JA4, window)
	ja4Count, err := rdb.Get(context.Background(), ja4Key).Int64()
	if err != nil {
		t.Fatalf("read JA4 velocity counter: %v", err)
	}
	if ja4Count != 1 {
		t.Fatalf("JA4 velocity counter = %d, want one increment", ja4Count)
	}
}

func TestScoreFragmentedOnly(t *testing.T) {
	// curl also trips scripting_tool, so isolate fragmented_handshake
	// with a UA that isn't a known scripting tool or browser claim.
	got := Score(RequestFacts{JA4: JA4Unreadable, UA: "SomeUnknownClient/1.0"})
	want := fragmentedWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScoreUAMismatchOnly(t *testing.T) {
	// Claims Firefox but negotiated TLS 1.0, so only UAMismatch fires.
	got := Score(facts("t10d1516h2_8daaf6152771_e5627efa2ab1", "Mozilla/5.0 Firefox/120.0"))
	want := uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScoreBothSignals(t *testing.T) {
	got := Score(facts(JA4Unreadable, "Mozilla/5.0 Chrome/120.0"))
	want := fragmentedWeight + uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScorePlainHTTPFailsOpen(t *testing.T) {
	// No TLS at all (ja4 == "") - can't fingerprint, must not penalize.
	got := Score(facts("", "Mozilla/5.0 Chrome/120.0"))
	if got != 0 {
		t.Fatalf("Score() = %d, want 0", got)
	}
}

func TestScoreJA4Blocklist(t *testing.T) {
	got := Score(RequestFacts{JA4: "t12d190800_4464c1bd5eb7_b3394627b738", UA: "SomeUnknownClient/1.0"})
	if got != 100 {
		t.Fatalf("Score() = %d, want 100", got)
	}
}

func TestScoreJA4BlocklistWithUAMismatch(t *testing.T) {
	// Seed the scraper JA4 here: the entry lives in the shared ja4db
	// state, and relying on another test to have populated it made this
	// test order-dependent (it failed under -run filters that skipped
	// that test). The hash is the verified python-requests capture.
	AddKnownScraperJA4("t12d190800_4464c1bd5eb7_b3394627b738", "python-requests")
	got := Score(facts("t12d190800_4464c1bd5eb7_b3394627b738", "Mozilla/5.0 Chrome/120.0"))
	want := 100 + uaMismatchWeight
	if got != want {
		t.Fatalf("Score() = %d, want %d", got, want)
	}
}

func TestScoreHeaderAnomaly(t *testing.T) {
	// A client claiming Chrome but sending none of the headers every real
	// browser sends is the evasion header_anomaly exists to catch.
	got := Score(RequestFacts{JA4: "t13d1516h2_8daaf6152771_e5627efa2ab1", UA: "Mozilla/5.0 Chrome/120.0", Header: http.Header{}})
	if got != headerAnomalyWeight {
		t.Fatalf("Score() = %d, want %d", got, headerAnomalyWeight)
	}
}

func TestScoreHeaderAnomalyUnknownClientStaysQuiet(t *testing.T) {
	got := Score(RequestFacts{JA4: JA4Unreadable, UA: "SomeUnknownClient/1.0", Header: http.Header{}})
	if got != fragmentedWeight {
		t.Fatalf("Score() = %d, want %d", got, fragmentedWeight)
	}
}

// TestDecideThresholds verifies outcome contracts for both PolicyBalanced and PolicyStrict modes.
func TestDecideThresholds(t *testing.T) {
	t.Run("PolicyBalanced", func(t *testing.T) {
		cases := []struct {
			score int
			want  Decision
		}{
			{0, DecisionAllow},
			{headerAnomalyWeight, DecisionChallenge},
			{fragmentedWeight, DecisionChallenge},
			{blockThreshold - 1, DecisionChallenge},
			{blockThreshold, DecisionBlock},
			{blockThreshold + 50, DecisionBlock},
		}
		for _, c := range cases {
			if got := DecideWithPolicy(c.score, config.PolicyBalanced); got != c.want {
				t.Errorf("DecideWithPolicy(%d, Balanced) = %v, want %v", c.score, got, c.want)
			}
		}
	})

	t.Run("PolicyStrict", func(t *testing.T) {
		cases := []struct {
			score int
			want  Decision
		}{
			{0, DecisionChallenge},
			{headerAnomalyWeight, DecisionChallenge},
			{fragmentedWeight, DecisionChallenge},
			{blockThreshold - 1, DecisionChallenge},
			{blockThreshold, DecisionBlock},
			{blockThreshold + 50, DecisionBlock},
		}
		for _, c := range cases {
			if got := DecideWithPolicy(c.score, config.PolicyStrict); got != c.want {
				t.Errorf("DecideWithPolicy(%d, Strict) = %v, want %v", c.score, got, c.want)
			}
		}
	})
}

// TestWeakSignalsNeverBlockAlone guards CLAUDE.md Section 6: no single
// signal may be the only thing between allow and block. The two
// mid-strength signals must never, on their own, produce a block.
func TestWeakSignalsNeverBlockAlone(t *testing.T) {
	for _, w := range []int{headerAnomalyWeight, fragmentedWeight, crawlPatternWeight} {
		if got := Decide(w); got == DecisionBlock {
			t.Fatalf("a single signal worth %d must not block, got %v", w, got)
		}
	}
}

func TestDecisionString(t *testing.T) {
	cases := map[Decision]string{
		DecisionAllow:     "allow",
		DecisionChallenge: "challenge",
		DecisionBlock:     "block",
		DecisionDeceive:   "deceive",
		Decision(99):      "unknown",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Decision(%d).String() = %q, want %q", d, got, want)
		}
	}
}
