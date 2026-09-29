package signals

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// A session is one (tenant, IP, User-Agent) seen within sessionIdle. It
// counts what kind of requests arrive, which a real browser cannot help
// but reveal: a page load pulls its CSS, scripts and images, and a login
// form is loaded before it is submitted.
//
// Each session is one small Redis hash of integer counters that expires
// after sessionIdle without traffic. Redis runs with noeviction, so this
// state must stay small and self-expiring.
const (
	sessionIdle = 15 * time.Minute
	// minDocsWithoutSubresources is how many page loads with no
	// subresource at all it takes to look like a scraper that blocks
	// assets. A returning visitor with a warm cache still triggers some
	// image, font or fetch request within a few pages.
	minDocsWithoutSubresources = 8
	// minBeaconsWithoutInteraction: pages a person viewed without a single
	// pointer, scroll, key or touch event.
	minBeaconsWithoutInteraction = 3
	// minDocsWithoutBeacon: page loads on a tenant that runs the beacon
	// script, with no beacon ever arriving.
	minDocsWithoutBeacon = 5
	// beaconTenantTTL: a tenant counts as running the beacon script for a
	// day after the last beacon from anyone.
	beaconTenantTTL = 24 * time.Hour
)

// Session hash fields.
const (
	fieldDoc       = "doc"  // page navigations
	fieldSub       = "sub"  // subresource and fetch requests
	fieldBeacon    = "bcn"  // beacons received
	fieldInteract  = "bint" // beacons reporting any user input
	fieldWebdriver = "bwd"  // beacons reporting navigator.webdriver
)

// uaBuckets caps sessions per IP. Keying on the full User-Agent would let
// one IP mint a new Redis key per request by rotating its UA, and Redis
// runs with noeviction, so filling it would also break challenge replay
// protection. Sixteen buckets still separate the few browsers behind a
// shared NAT; a rare collision only merges two sessions' counts.
const uaBuckets = 16

func sessionKey(tenant, ip, ua string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(ua))
	return fmt.Sprintf("sess:t:%d:%s:ip:%s:ua:%d", len(tenant), tenant, ip, h.Sum32()%uaBuckets)
}

func beaconTenantKey(tenant string) string {
	return fmt.Sprintf("bcn:t:%d:%s", len(tenant), tenant)
}

// requestKind classifies a request as a page navigation ("doc"), a
// subresource ("sub"), or neither (""). Sec-Fetch-Dest is sent by every
// current browser; the path extension covers assets without it.
func requestKind(f RequestFacts) string {
	if isStaticAsset(f.Path) {
		return fieldSub
	}
	// A form submission is a navigation too, but it is not a page load:
	// counting it would let a direct POST excuse itself.
	navigation := f.Method == "" || strings.EqualFold(f.Method, "GET") || strings.EqualFold(f.Method, "HEAD")
	switch dest := f.Header.Get("Sec-Fetch-Dest"); dest {
	case "document":
		if navigation {
			return fieldDoc
		}
		return ""
	case "":
		if navigation && strings.Contains(f.Header.Get("Accept"), "text/html") {
			return fieldDoc
		}
		return ""
	default:
		return fieldSub
	}
}

// sessionSignals counts this request into its session and reports the
// session's shape. Only browser-claiming clients are counted: scripts
// that admit what they are are already caught by other checks, and this
// targets automation that looks like a browser on the wire.
func sessionSignals(f RequestFacts) []string {
	if f.Tenant == "" || f.IP == "" || f.VerifiedGoodBot || !claimsBrowser(f.UA) || !redisRequestAllowed() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	key := sessionKey(f.Tenant, f.IP, f.UA)
	kind := requestKind(f)
	pipe := rdb.Pipeline()
	if kind != "" {
		pipe.HIncrBy(ctx, key, kind, 1)
	}
	pipe.Expire(ctx, key, sessionIdle)
	counts := pipe.HMGet(ctx, key, fieldDoc, fieldSub, fieldBeacon, fieldInteract, fieldWebdriver)
	beaconTenant := pipe.Exists(ctx, beaconTenantKey(f.Tenant))
	if _, err := pipe.Exec(ctx); err != nil {
		redisHealth.failure(time.Now())
		return nil
	}
	redisHealth.success()

	n := make([]int64, 5)
	for i, v := range counts.Val() {
		if s, ok := v.(string); ok {
			_, _ = fmt.Sscan(s, &n[i])
		}
	}
	doc, sub, beacons, interacted, webdriver := n[0], n[1], n[2], n[3], n[4]

	var found []string
	if kind == fieldDoc && doc >= minDocsWithoutSubresources && sub == 0 {
		found = append(found, "no_subresources")
	}
	if doc == 0 && isSensitiveSubmit(f) {
		found = append(found, "direct_sensitive_post")
	}
	if webdriver > 0 {
		found = append(found, "beacon_webdriver")
	}
	if beacons >= minBeaconsWithoutInteraction && interacted == 0 {
		found = append(found, "beacon_no_interaction")
	}
	if beaconTenant.Val() > 0 && doc >= minDocsWithoutBeacon && beacons == 0 {
		found = append(found, "beacon_missing")
	}
	return found
}

// isSensitiveSubmit reports a state-changing request to a login or
// checkout route. A person loads the form page before submitting it;
// credential stuffers and checkout bots post directly.
func isSensitiveSubmit(f RequestFacts) bool {
	if f.Method == "" || strings.EqualFold(f.Method, "GET") || strings.EqualFold(f.Method, "HEAD") || strings.EqualFold(f.Method, "OPTIONS") {
		return false
	}
	class := f.RouteClass
	if class == "" {
		p, ok := NormalizePath(f.Path)
		if !ok {
			return false
		}
		class = Classify(p, f.Method)
	}
	return class == ClassLogin || class == ClassCheckout
}

// Beacon is what the opt-in page script reports when a page is left: only
// whether each kind of input happened, never what or where, plus the
// navigator.webdriver flag.
type Beacon struct {
	Webdriver bool
	Pointer   bool
	Scroll    bool
	Key       bool
	Touch     bool
}

// RecordBeacon adds one beacon to the sender's session and marks the
// tenant as running the script. A beacon is client-supplied and trivially
// forged, so it can only ever clear a client of the beacon signals or
// add evidence against it; it never grants trust. Returns false when
// Redis was unavailable.
func RecordBeacon(tenant, ip, ua string, b Beacon) bool {
	if tenant == "" || ip == "" || !claimsBrowser(ua) || !redisRequestAllowed() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	key := sessionKey(tenant, ip, ua)
	pipe := rdb.Pipeline()
	pipe.HIncrBy(ctx, key, fieldBeacon, 1)
	if b.Pointer || b.Scroll || b.Key || b.Touch {
		pipe.HIncrBy(ctx, key, fieldInteract, 1)
	}
	if b.Webdriver {
		pipe.HIncrBy(ctx, key, fieldWebdriver, 1)
	}
	pipe.Expire(ctx, key, sessionIdle)
	pipe.Set(ctx, beaconTenantKey(tenant), 1, beaconTenantTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		redisHealth.failure(time.Now())
		return false
	}
	redisHealth.success()
	return true
}
