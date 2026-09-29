package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const beaconUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"

// beaconGuard is a shadow-mode guard for example.com whose origin counts
// how often it is reached.
func beaconGuard(t *testing.T) (*core.Guard, *tenant.Store, *int) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	signals.InitRedis(client)
	t.Cleanup(func() { signals.InitRedis(nil) })

	originHits := new(int)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*originHits++
		_, _ = w.Write([]byte("origin"))
	}))
	t.Cleanup(origin.Close)
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	store := tenant.NewStore()
	if err := store.Add("tenant-a", tenant.TenantConfig{Target: origin.URL, Mode: config.ModeShadow, Policy: config.PolicyBalanced}, []string{"example.com"}, proxy); err != nil {
		t.Fatal(err)
	}
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatal(err)
	}
	return core.NewGuard(store, c), store, originHits
}

func serve(g *core.Guard, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:4444"
	req.Header.Set("User-Agent", beaconUA)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

func TestBeaconScriptIsServedByTheProxy(t *testing.T) {
	g, _, originHits := beaconGuard(t)
	rec := serve(g, http.MethodGet, "/__hakaishield/b.js", "", nil)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("script: status=%d type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "sendBeacon") || !strings.Contains(rec.Body.String(), "/__hakaishield/beacon") {
		t.Fatalf("script body does not post beacons: %q", rec.Body.String())
	}
	if rec := serve(g, http.MethodPost, "/__hakaishield/b.js", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST to script: status=%d, want 405", rec.Code)
	}
	if *originHits != 0 {
		t.Fatal("beacon script request reached the origin")
	}
}

func TestBeaconFeedsSessionEvidence(t *testing.T) {
	g, store, originHits := beaconGuard(t)
	for i := 0; i < 3; i++ {
		rec := serve(g, http.MethodPost, "/__hakaishield/beacon", `{"wd":1,"pm":0,"sc":0,"kd":0,"tc":0}`, map[string]string{"Content-Type": "text/plain;charset=UTF-8"})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("beacon %d: status=%d body=%q", i, rec.Code, rec.Body.String())
		}
	}
	if *originHits != 0 {
		t.Fatal("beacon reached the origin")
	}

	rec := serve(g, http.MethodGet, "/pricing", "", map[string]string{"Sec-Fetch-Dest": "document"})
	if rec.Code != http.StatusOK || *originHits != 1 {
		t.Fatalf("page after beacons: status=%d originHits=%d", rec.Code, *originHits)
	}
	ta, err := store.GetByHost("example.com")
	if err != nil {
		t.Fatal(err)
	}
	records := ta.Trail.Recent(1)
	if len(records) != 1 {
		t.Fatalf("want one evidence record, got %d", len(records))
	}
	got := records[0].ShadowSignals
	if !slices.Contains(got, "beacon_webdriver") || !slices.Contains(got, "beacon_no_interaction") {
		t.Fatalf("shadow signals = %v, want beacon_webdriver and beacon_no_interaction", got)
	}
	if records[0].Score != 0 {
		t.Fatalf("beacon evidence changed the score to %d; it must stay shadow-only", records[0].Score)
	}
}

func TestBeaconRejectsBadInput(t *testing.T) {
	g, _, _ := beaconGuard(t)
	cases := []struct {
		name, method, body string
		want               int
	}{
		{"GET", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"not JSON", http.MethodPost, "hello", http.StatusBadRequest},
		{"unknown field", http.MethodPost, `{"wd":0,"x":"y"}`, http.StatusBadRequest},
		{"oversized", http.MethodPost, `{"wd":0,"pm":` + strings.Repeat("1", 400) + `}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if rec := serve(g, tc.method, "/__hakaishield/beacon", tc.body, nil); rec.Code != tc.want {
			t.Errorf("%s: status=%d, want %d", tc.name, rec.Code, tc.want)
		}
	}
}

func TestBeaconForUnknownHostWritesNothing(t *testing.T) {
	g, _, _ := beaconGuard(t)
	req := httptest.NewRequest(http.MethodPost, "http://not-ours.example/__hakaishield/beacon", strings.NewReader(`{"wd":1}`))
	req.Header.Set("User-Agent", beaconUA)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("beacon to an unserved host: status=%d, want 421", rec.Code)
	}
}
