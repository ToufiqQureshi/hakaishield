package signals

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"
)

// staticAssetExts are the subresource types a real browser fetches by
// itself after a page navigation. Classification is by URL path, which
// the server observes directly — a scraper can forge every header, but
// it cannot change which URLs the origin actually serves as assets.
var staticAssetExts = map[string]bool{
	".css": true, ".js": true, ".mjs": true, ".map": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".avif": true, ".svg": true, ".ico": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp4": true, ".webm": true, ".mp3": true, ".wasm": true,
}

// isStaticAsset reports whether a request path looks like a subresource
// rather than a page navigation. Used so the dozens of asset requests a
// single real page load produces are not counted against the strict
// navigation rate limit (CLAUDE.md Section 14 — a false positive is
// worse than a miss).
func isStaticAsset(p string) bool {
	if p == "" {
		return false
	}
	return staticAssetExts[strings.ToLower(path.Ext(p))]
}

// crawlWindowMs buckets distinct-path counting. A person reading a site
// touches a handful of pages a minute; a crawler walking it touches
// hundreds.
const (
	crawlWindowMs    = 60_000
	maxDistinctPaths = 60
)

// CrawlPatternSuspected reports whether one client has requested many
// distinct page paths inside the current window — the shape of a crawler
// walking a site, not a person reading it.
//
// It is server-observed (the URL path), so it survives every client-side
// spoof; the cost it imposes is that an attacker must slow down and
// diversify less to stay hidden. Browser-claiming clients and declared
// crawlers are counted. Only a supported crawler whose IP passed
// reverse/forward DNS verification is exempt; a User-Agent claim alone is
// not identity proof. Static assets are never counted because a real page
// load produces many of them.
//
// Counting uses a Redis HyperLogLog: cardinality is estimated rather
// than stored, so memory stays bounded (~12KB) no matter how many paths
// one IP throws at it (CLAUDE.md Section 15).
func CrawlPatternSuspected(f RequestFacts) bool {
	if f.Tenant == "" || f.IP == "" || f.VerifiedGoodBot ||
		(!claimsBrowser(f.UA) && !claimsCrawler(f.UA)) ||
		isStaticAsset(f.Path) || !redisRequestAllowed() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	window := time.Now().UnixMilli() / int64(crawlWindowMs)
	key := fmt.Sprintf("crawl:t:%d:%s:ip:%s:%d", len(f.Tenant), f.Tenant, f.IP, window)

	pipe := rdb.Pipeline()
	pipe.PFAdd(ctx, key, f.Path)
	pipe.Expire(ctx, key, time.Duration(2*crawlWindowMs)*time.Millisecond)
	count := pipe.PFCount(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		// Fail open on Redis errors, like every other rate check, so a
		// Redis outage never blocks real traffic.
		redisHealth.failure(time.Now())
		return false
	}
	redisHealth.success()
	return count.Val() > maxDistinctPaths
}
