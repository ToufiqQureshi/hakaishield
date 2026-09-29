package signals

import (
	"context"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis points the package-level rdb at an in-memory miniredis
// instance for the duration of the test, then restores whatever was
// there before — the velocity/pattern checks read the package var
// directly, so tests can't inject a client any other way.
func newTestRedis(t *testing.T) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	prev := rdb
	prevWindow := rateLimitMs
	// These tests assert counter isolation and thresholds, not a wall-clock
	// boundary. Keep CI load from splitting a burst across one-second buckets.
	rateLimitMs = 60_000
	rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// Dial before the test starts. Request-path Redis calls have a 50 ms
	// budget, and a cold TCP dial on a loaded runner can exceed it, which
	// fails the check open and flakes the test.
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping miniredis: %v", err)
	}
	redisHealth.reset()
	t.Cleanup(func() { rdb = prev; rateLimitMs = prevWindow })
}

func resetCommonBrowserPrefixes(t *testing.T) {
	t.Helper()
	ja4Mu.Lock()
	prev := append([]string(nil), browserPrefixes...)
	browserPrefixes = nil
	ja4Mu.Unlock()
	t.Cleanup(func() {
		ja4Mu.Lock()
		browserPrefixes = prev
		ja4Mu.Unlock()
	})
}

func TestCheckVelocitySpikeNoRedisFailsOpen(t *testing.T) {
	prev := rdb
	rdb = nil
	defer func() { rdb = prev }()

	if checkVelocitySpike("test-tenant", "1.2.3.4", "/", "GET") {
		t.Fatal("checkVelocitySpike with nil rdb must fail open (false)")
	}
}

func TestCheckVelocitySpikeUnderLimit(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxNavPerWindow; i++ {
		if checkVelocitySpike("test-tenant", "1.2.3.4", "/pricing", "GET") {
			t.Fatalf("request %d: spiked before exceeding maxNavPerWindow=%d", i, maxNavPerWindow)
		}
	}
}

func TestCheckVelocitySpikeOverLimit(t *testing.T) {
	newTestRedis(t)
	var lastSpiked bool
	for i := 0; i < maxNavPerWindow+1; i++ {
		lastSpiked = checkVelocitySpike("test-tenant", "5.6.7.8", "/pricing", "GET")
	}
	if !lastSpiked {
		t.Fatalf("request %d: want spike after exceeding maxNavPerWindow=%d", maxNavPerWindow+1, maxNavPerWindow)
	}
}

// TestCheckVelocitySpikeExemptsAssets is the false-positive guard: a real
// browser loading a single page fires dozens of asset requests, way past
// the navigation limit, and must never be rate-limited for it.
func TestCheckVelocitySpikeExemptsAssets(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxNavPerWindow*5; i++ {
		if checkVelocitySpike("test-tenant", "7.7.7.7", "/static/app.js", "GET") {
			t.Fatalf("asset request %d: assets must not count against the navigation limit", i)
		}
	}
}

func TestCheckVelocitySpikeIsolatedPerIP(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxNavPerWindow+1; i++ {
		checkVelocitySpike("test-tenant", "9.9.9.9", "/", "GET")
	}
	if checkVelocitySpike("test-tenant", "1.1.1.1", "/", "GET") {
		t.Fatal("a fresh IP must not be flagged by another IP's velocity")
	}
}

func TestVelocityBucketClassifies(t *testing.T) {
	if _, limit := velocityBucket("test-tenant", "1.2.3.4", "/static/app.js", "GET", 0); limit != maxAssetPerWindow {
		t.Errorf("asset path: want asset limit %d, got %d", maxAssetPerWindow, limit)
	}
	if _, limit := velocityBucket("test-tenant", "1.2.3.4", "/pricing", "GET", 0); limit != maxNavPerWindow {
		t.Errorf("navigation path: want nav limit %d, got %d", maxNavPerWindow, limit)
	}
}

func TestCheckJA4VelocitySpikeExemptsCommonBrowsers(t *testing.T) {
	newTestRedis(t)
	resetCommonBrowserPrefixes(t)
	AddCommonBrowserPrefix("t13d1516h2")
	for i := 0; i < maxJA4Requests+1; i++ {
		if checkJA4VelocitySpike("test-tenant", "t13d1516h2_8daaf6152771_e5627efa2ab1") {
			t.Fatal("a common browser JA4 prefix must never be flagged by aggregate velocity")
		}
	}
}

func TestCheckJA4VelocitySpikeFailsOpenWithoutBrowserPrefixes(t *testing.T) {
	newTestRedis(t)
	resetCommonBrowserPrefixes(t)
	for i := 0; i < maxJA4Requests+1; i++ {
		if checkJA4VelocitySpike("test-tenant", "t99d000000_deadbeefdead_deadbeefdead") {
			t.Fatal("empty browser-prefix database must fail open to avoid challenging real browser builds")
		}
	}
}

func TestCheckJA4VelocitySpikeOverLimit(t *testing.T) {
	newTestRedis(t)
	resetCommonBrowserPrefixes(t)
	AddCommonBrowserPrefix("t13d1516h2")
	var lastSpiked bool
	for i := 0; i < maxJA4Requests+1; i++ {
		lastSpiked = checkJA4VelocitySpike("test-tenant", "t99d000000_deadbeefdead_deadbeefdead")
	}
	if !lastSpiked {
		t.Fatalf("want spike after exceeding maxJA4Requests for a non-browser JA4")
	}
}

func TestVelocityExceededCombinesBothChecks(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxNavPerWindow+1; i++ {
		VelocityExceeded("test-tenant", "2.2.2.2", "", "/", "GET")
	}
	if !VelocityExceeded("test-tenant", "2.2.2.2", "", "/", "GET") {
		t.Fatal("VelocityExceeded must reflect an IP-only spike")
	}
	if VelocityExceeded("test-tenant", "3.3.3.3", "", "/", "GET") {
		t.Fatal("VelocityExceeded must not flag an unrelated, low-volume IP")
	}
}

func TestVelocitySignalIsolatedPerTenant(t *testing.T) {
	newTestRedis(t)
	var last Evaluation
	for i := 0; i < maxNavPerWindow+1; i++ {
		last = Evaluate(RequestFacts{Tenant: "tenant-a", IP: "2.2.2.2", Path: "/pricing"})
	}
	if !slices.Contains(last.Signals, "velocity_spike") {
		t.Fatal("tenant A's navigation burst must fire velocity_spike")
	}
	other := Evaluate(RequestFacts{Tenant: "tenant-b", IP: "2.2.2.2", Path: "/pricing"})
	if slices.Contains(other.Signals, "velocity_spike") {
		t.Fatal("tenant B inherited tenant A's velocity counter")
	}
}

func TestJA4VelocitySignalIsolatedPerTenant(t *testing.T) {
	newTestRedis(t)
	resetCommonBrowserPrefixes(t)
	AddCommonBrowserPrefix("t13d1516h2")
	const ja4 = "t99d000000_deadbeefdead_deadbeefdead"
	var last Evaluation
	for i := 0; i < maxJA4Requests+1; i++ {
		last = Evaluate(RequestFacts{Tenant: "tenant-a", JA4: ja4, Path: "/pricing"})
	}
	if !slices.Contains(last.Signals, "ja4_velocity_spike") {
		t.Fatal("tenant A's JA4 burst must fire ja4_velocity_spike")
	}
	other := Evaluate(RequestFacts{Tenant: "tenant-b", JA4: ja4, Path: "/pricing"})
	if slices.Contains(other.Signals, "ja4_velocity_spike") {
		t.Fatal("tenant B inherited tenant A's JA4 counter")
	}
}
