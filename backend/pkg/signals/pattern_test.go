package signals

import (
	"fmt"
	"testing"
)

func TestIsStaticAsset(t *testing.T) {
	assets := []string{"/static/app.js", "/a/b/style.CSS", "/img/logo.png", "/fonts/x.woff2", "/v.mp4"}
	for _, p := range assets {
		if !isStaticAsset(p) {
			t.Errorf("isStaticAsset(%q) = false, want true", p)
		}
	}
	pages := []string{"/", "/pricing", "/products/42", "/api/v1/search", ""}
	for _, p := range pages {
		if isStaticAsset(p) {
			t.Errorf("isStaticAsset(%q) = true, want false", p)
		}
	}
}

func TestCrawlPatternNoRedisFailsOpen(t *testing.T) {
	prev := rdb
	rdb = nil
	defer func() { rdb = prev }()

	if CrawlPatternSuspected(RequestFacts{Tenant: "test-tenant", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: "/a"}) {
		t.Fatal("CrawlPatternSuspected with nil rdb must fail open (false)")
	}
}

// An honest non-crawler HTTP client isn't claiming browser navigation
// behavior, so the crawl-shape check stays neutral for it.
func TestCrawlPatternExemptsNonBrowser(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxDistinctPaths+10; i++ {
		f := RequestFacts{Tenant: "test-tenant", IP: "1.2.3.4", UA: "curl/8.6.0", Path: fmt.Sprintf("/p/%d", i)}
		if CrawlPatternSuspected(f) {
			t.Fatal("a non-browser-claiming client must stay exempt from crawl detection")
		}
	}
}

func TestCrawlPatternCountsUnverifiedGoodBotClaim(t *testing.T) {
	newTestRedis(t)
	var last bool
	for i := 0; i < maxDistinctPaths+1; i++ {
		last = CrawlPatternSuspected(RequestFacts{
			Tenant: "test-tenant",
			IP:     "1.2.3.4",
			UA:     "Mozilla/5.0 (compatible; Googlebot/2.1) Chrome/120.0.0.0",
			Path:   fmt.Sprintf("/p/%d", i),
		})
	}
	if !last {
		t.Fatal("unverified Googlebot claim must not bypass server-observed crawl detection")
	}
}

func TestCrawlPatternCountsUnverifiedUnknownCrawlerClaim(t *testing.T) {
	newTestRedis(t)
	var last bool
	for i := 0; i < maxDistinctPaths+1; i++ {
		last = CrawlPatternSuspected(RequestFacts{
			Tenant: "test-tenant",
			IP:     "1.2.3.5",
			UA:     "ExampleSpider/1.0",
			Path:   fmt.Sprintf("/unknown/%d", i),
		})
	}
	if !last {
		t.Fatal("unverified crawler declaration must not bypass server-observed crawl detection")
	}
}

func TestCrawlPatternExemptsVerifiedGoodBot(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxDistinctPaths+10; i++ {
		f := RequestFacts{
			Tenant:          "test-tenant",
			IP:              "1.2.3.4",
			UA:              "Mozilla/5.0 (compatible; Googlebot/2.1) Chrome/120.0.0.0",
			Path:            fmt.Sprintf("/p/%d", i),
			VerifiedGoodBot: true,
		}
		if CrawlPatternSuspected(f) {
			t.Fatal("DNS-verified good bot must stay exempt from crawl-pattern scoring")
		}
	}
}

// TestCrawlPatternExemptsAssets: a real page load produces many asset
// requests, which must never count as distinct pages.
func TestCrawlPatternExemptsAssets(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxDistinctPaths+10; i++ {
		f := RequestFacts{Tenant: "test-tenant", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: fmt.Sprintf("/static/%d.js", i)}
		if CrawlPatternSuspected(f) {
			t.Fatal("asset requests must not count toward distinct page paths")
		}
	}
}

func TestCrawlPatternUnderLimit(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxDistinctPaths; i++ {
		f := RequestFacts{Tenant: "test-tenant", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: fmt.Sprintf("/p/%d", i)}
		if CrawlPatternSuspected(f) {
			t.Fatalf("page %d: fired before exceeding maxDistinctPaths=%d", i, maxDistinctPaths)
		}
	}
}

// TestCrawlPatternFiresOnManyDistinctPaths: a browser-claiming client
// walking dozens of distinct pages in the window is the crawler shape.
func TestCrawlPatternFiresOnManyDistinctPaths(t *testing.T) {
	newTestRedis(t)
	var last bool
	for i := 0; i < maxDistinctPaths+1; i++ {
		last = CrawlPatternSuspected(RequestFacts{Tenant: "test-tenant", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: fmt.Sprintf("/p/%d", i)})
	}
	if !last {
		t.Fatalf("want crawl pattern after exceeding maxDistinctPaths=%d", maxDistinctPaths)
	}
}

// TestCrawlPatternRepeatedSamePathDoesNotFire: hitting one page many
// times is a reload loop, not a crawl. HyperLogLog counts cardinality,
// not volume, so it must not fire.
func TestCrawlPatternRepeatedSamePathDoesNotFire(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxDistinctPaths*3; i++ {
		if CrawlPatternSuspected(RequestFacts{Tenant: "test-tenant", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: "/pricing"}) {
			t.Fatal("repeated requests to one path must not look like a crawl")
		}
	}
}

func TestCrawlPatternIsolatedPerIP(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxDistinctPaths+1; i++ {
		CrawlPatternSuspected(RequestFacts{Tenant: "test-tenant", IP: "9.9.9.9", UA: "Mozilla/5.0 Chrome/120.0", Path: fmt.Sprintf("/p/%d", i)})
	}
	if CrawlPatternSuspected(RequestFacts{Tenant: "test-tenant", IP: "1.1.1.1", UA: "Mozilla/5.0 Chrome/120.0", Path: "/p/0"}) {
		t.Fatal("a fresh IP must not inherit another IP's crawl count")
	}
}

func TestCrawlPatternIsolatedPerTenant(t *testing.T) {
	newTestRedis(t)
	var last bool
	for i := 0; i < maxDistinctPaths+1; i++ {
		last = CrawlPatternSuspected(RequestFacts{Tenant: "tenant-a", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: fmt.Sprintf("/p/%d", i)})
	}
	if !last {
		t.Fatal("tenant A's crawl must fire")
	}
	if CrawlPatternSuspected(RequestFacts{Tenant: "tenant-b", IP: "1.2.3.4", UA: "Mozilla/5.0 Chrome/120.0", Path: "/p/0"}) {
		t.Fatal("tenant B inherited tenant A's crawl counter")
	}
}
