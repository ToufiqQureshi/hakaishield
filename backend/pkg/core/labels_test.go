package core_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/labels"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// captureWriter records what the collector actually persisted, so these
// tests assert on stored samples rather than on a call happening.
type captureWriter struct {
	mu  sync.Mutex
	got []labels.Sample
}

func (w *captureWriter) WriteSamples(_ context.Context, s []labels.Sample) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.got = append(w.got, s...)
	return nil
}

func (w *captureWriter) samples() []labels.Sample {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]labels.Sample(nil), w.got...)
}

// labelFixture builds a guard with label collection on, in front of a
// real origin.
func labelFixture(t *testing.T, policy config.PolicyMode) (*core.Guard, *challenge.Challenge, *captureWriter, *labels.Recorder) {
	t.Helper()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("origin"))
	}))
	t.Cleanup(origin.Close)

	store := tenant.NewStore()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("NewChallenge() error: %v", err)
	}
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatalf("NewOriginProxy() error: %v", err)
	}
	store.Add("default", tenant.TenantConfig{
		Target: origin.URL,
		Mode:   config.ModeEnforce,
		Policy: policy,
	}, []string{"example.com"}, proxy)

	w := &captureWriter{}
	recorder := labels.NewRecorder(w)
	guard := core.NewGuard(store, c).WithLabelRecorder(recorder)

	return guard, c, w, recorder
}

// The whole point of the collector: a request that was challenged and
// then solved becomes a human-labelled sample carrying the checks that
// fired on the request that caused the challenge.
func TestSolvedChallengeBecomesAHumanLabel(t *testing.T) {
	guard, c, w, recorder := labelFixture(t, config.PolicyBalanced)

	// A browser missing its fetch-metadata headers fires header_anomaly,
	// so it is challenged rather than allowed.
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "__hakaishield") {
		t.Fatalf("the request was not challenged, so there is nothing to label: %d", rec.Code)
	}
	solveFromPage(t, c, body, "example.com")
	recorder.Close()

	got := w.samples()
	if len(got) != 1 {
		t.Fatalf("solving the challenge produced %d samples, want 1: %+v", len(got), got)
	}
	if got[0].Automated {
		t.Error("a solved challenge was labelled automated")
	}
	if got[0].Source != labels.SourceChallengeSolved {
		t.Errorf("Source = %q, want %q", got[0].Source, labels.SourceChallengeSolved)
	}
	if got[0].TenantID != "default" {
		t.Errorf("TenantID = %q, want the tenant that was served", got[0].TenantID)
	}
	if got[0].FeatureVersion != signals.FeatureVersion() {
		t.Errorf("FeatureVersion = %q, want %q", got[0].FeatureVersion, signals.FeatureVersion())
	}

	// The mask has to describe the request that was challenged, not an
	// empty one, or the sample teaches the model nothing.
	header, ok := signals.FeatureBit("header_anomaly")
	if !ok {
		t.Fatal("this build has no header_anomaly check")
	}
	if got[0].Fired&header == 0 {
		t.Errorf("Fired = %b, want header_anomaly set: the mask must describe the challenged request", got[0].Fired)
	}
}

// A challenge that is never solved says nothing about the client, so it
// must not produce a label in either direction.
func TestUnsolvedChallengeProducesNoLabel(t *testing.T) {
	guard, _, w, recorder := labelFixture(t, config.PolicyBalanced)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	guard.ServeHTTP(httptest.NewRecorder(), req)
	recorder.Close()

	if got := w.samples(); len(got) != 0 {
		t.Fatalf("an unsolved challenge produced %+v, want nothing", got)
	}
}

// A caller that followed the invisible trap link is labelled automated,
// and the honeypot bit is cleared: leaving it in would make the model
// learn the label back instead of learning from the other checks.
// trapSeq gives every honeypot test run its own caller. Honeypot trips are
// remembered process-wide, so a fixed IP stops being a "first trip" when
// the test runs again under -count or in a shuffled order.
var trapSeq atomic.Int32

func freshTrapAddr() string {
	n := trapSeq.Add(1)
	return fmt.Sprintf("198.18.%d.%d:5555", n/250, n%250+1)
}

func TestHoneypotTripBecomesAnAutomatedLabelWithoutItsOwnBit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error: %v", err)
	}
	defer mr.Close()
	signals.InitRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	guard, _, w, recorder := labelFixture(t, config.PolicyBalanced)

	trap := httptest.NewRequest("GET", "http://example.com"+signals.HoneypotPath, nil)
	trap.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	addr := freshTrapAddr()
	trap.RemoteAddr = addr
	guard.ServeHTTP(httptest.NewRecorder(), trap)

	// A trap hit must be captured even if this caller never requests
	// another page. A subsequent request must not duplicate its label.
	next := httptest.NewRequest("GET", "http://example.com/products", nil)
	next.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	next.RemoteAddr = addr
	guard.ServeHTTP(httptest.NewRecorder(), next)
	recorder.Close()

	got := w.samples()
	if len(got) != 1 {
		t.Fatalf("trap plus subsequent request produced %d labels, want 1: %+v", len(got), got)
	}
	honeypot, ok := signals.FeatureBit(signals.FeatureHoneypotTrap)
	if !ok {
		t.Fatal("this build has no honeypot_trap check")
	}
	for _, s := range got {
		if s.Source != labels.SourceHoneypotTrap {
			continue
		}
		if !s.Automated {
			t.Error("a honeypot trip was labelled human")
		}
		if s.Fired&honeypot != 0 {
			t.Errorf("Fired = %b still carries honeypot_trap: the model would learn the label back", s.Fired)
		}
		return
	}
	t.Fatalf("no honeypot-sourced sample among %+v", got)
}

func TestHoneypotTripWithoutFollowupProducesALabel(t *testing.T) {
	guard, _, w, recorder := labelFixture(t, config.PolicyBalanced)
	trap := httptest.NewRequest("GET", "http://example.com"+signals.HoneypotPath, nil)
	trap.RemoteAddr = freshTrapAddr()
	guard.ServeHTTP(httptest.NewRecorder(), trap)
	recorder.Close()
	got := w.samples()
	if len(got) != 1 || got[0].Source != labels.SourceHoneypotTrap || !got[0].Automated {
		t.Fatalf("single trap hit produced %+v, want one automated honeypot sample", got)
	}
}

// With no recorder attached — the default — nothing is collected and
// nothing about the request changes.
func TestNoRecorderCollectsNothing(t *testing.T) {
	guard, c, w, recorder := labelFixture(t, config.PolicyBalanced)
	guard.WithLabelRecorder(nil)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "__hakaishield") {
		solveFromPage(t, c, rec.Body.String(), "example.com")
	}
	recorder.Close()

	if got := w.samples(); len(got) != 0 {
		t.Fatalf("collection was off but produced %+v", got)
	}
}

// solveFromPage completes the challenge shown on a served page, through
// the same handler a browser would post to.
func solveFromPage(t *testing.T, c *challenge.Challenge, body, host string) {
	t.Helper()

	tm := regexp.MustCompile(`token", "([^"]+)"`).FindStringSubmatch(body)
	nm := regexp.MustCompile(`encode\("([^"]+)"`).FindStringSubmatch(body)
	dm := regexp.MustCompile(`var difficulty =\s*(\d+)`).FindStringSubmatch(body)
	if tm == nil || nm == nil || dm == nil {
		t.Fatalf("could not extract token/nonce/difficulty from the challenge page")
	}
	difficulty := 0
	for _, ch := range dm[1] {
		difficulty = difficulty*10 + int(ch-'0')
	}

	form := url.Values{}
	form.Set("token", tm[1])
	form.Set("answer", solvePoW(nm[1], difficulty))
	form.Set("canvas", testCanvasProof())

	post := httptest.NewRequest(http.MethodPost, "/__hakaishield/verify", strings.NewReader(form.Encode()))
	post.Host = host
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, post)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("verify returned %d, want a redirect: %s", rec.Code, rec.Body.String())
	}
}
