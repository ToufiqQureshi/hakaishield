package signals

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// Verified captures (2026-09-29, see family.go).
const (
	chromeUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"
	edgeUA    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36 Edg/154.0.0.0"
	firefoxUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:142.0) Gecko/20100101 Firefox/142.0"
	chromeH2  = "1:65536;2:0;4:6291456;6:262144|15663105||masp"
	firefoxH2 = "1:65536;2:0;4:131072;5:16384|12517377||mpas"
	goH2      = "2:0;4:4194304;5:16384;6:10485760|1073741824||amps"
)

func TestH2FamilyMismatch(t *testing.T) {
	cases := []struct {
		name, ua, h2 string
		want         bool
	}{
		{"chrome greets like chrome", chromeUA, chromeH2, false},
		{"edge greets like chrome", edgeUA, chromeH2, false},
		{"firefox greets like firefox", firefoxUA, firefoxH2, false},
		{"go client wearing chrome UA", chromeUA, goH2, true},
		{"chrome UA on firefox stack", chromeUA, firefoxH2, true},
		{"firefox UA on chrome stack", firefoxUA, chromeH2, true},
		{"HTTP/1.1, nothing to compare", chromeUA, "", false},
		{"iOS chrome is WebKit underneath", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/153.0 Mobile/15E148 Safari/604.1", goH2, false},
		// Some iOS in-app browsers append a Chrome/ token; the stack is
		// still WebKit, so the Chromium comparison must not apply.
		{"iOS in-app UA with Chrome token", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Chrome/153.0.0.0 Safari/604.1", "2:0;3:100;4:2097152|10485760||msap", false},
		{"safari not captured yet", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15", goH2, false},
		{"honest tool is not a browser claim", "curl/8.6.0", chromeH2, false},
		{"greeting without pseudo order", chromeUA, "2:0|0||", false},
	}
	for _, tc := range cases {
		if got := h2FamilyMismatch(tc.ua, tc.h2); got != tc.want {
			t.Errorf("%s: h2FamilyMismatch = %v, want %v", tc.name, got, tc.want)
		}
	}
}

const asnFixture = "1.0.0.0\t1.0.0.255\t13335\tUS\tCLOUDFLARENET\n" +
	"3.0.0.0\t3.0.255.255\t16509\tUS\tAMAZON-02\n" +
	"not an ip\t3.1.0.0\t16509\tUS\tbroken\n" +
	"5.9.0.0\t5.9.255.255\t24940\tDE\tHETZNER\n" +
	"81.0.0.0\t81.0.0.255\t3320\tDE\tDTAG consumer\n" +
	"2600:1f00::\t2600:1fff:ffff:ffff:ffff:ffff:ffff:ffff\t16509\tUS\tAMAZON-02\n"

func loadASNFixture(t *testing.T, data string) int {
	t.Helper()
	prev := hostingRanges
	t.Cleanup(func() { hostingRanges = prev })
	n, err := LoadHostingASNDB(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHostingIPLookup(t *testing.T) {
	if n := loadASNFixture(t, asnFixture); n != 3 {
		t.Fatalf("kept %d hosting ranges, want 3 (AWS v4, Hetzner, AWS v6)", n)
	}
	cases := map[string]bool{
		"3.0.0.0":         true,  // range start
		"3.0.255.255":     true,  // range end
		"3.1.0.0":         false, // one past the end
		"2.255.255.255":   false, // one before the start
		"5.9.10.20":       true,
		"::ffff:5.9.10.2": true,  // IPv4-mapped form of a hosting IP
		"1.0.0.1":         false, // Cloudflare is not a hosting ASN here
		"81.0.0.7":        false, // consumer ISP
		"2600:1f14::1":    true,
		"2a00::1":         false,
		"garbage":         false,
		"":                false,
	}
	for ip, want := range cases {
		if got := isHostingIP(ip); got != want {
			t.Errorf("isHostingIP(%q) = %v, want %v", ip, got, want)
		}
	}
}

func TestHostingDBRejectsEmptyAndUnloaded(t *testing.T) {
	prev := hostingRanges
	t.Cleanup(func() { hostingRanges = prev })
	hostingRanges = nil
	if isHostingIP("3.0.0.1") {
		t.Fatal("no database loaded must mean no signal")
	}
	if _, err := LoadHostingASNDB(strings.NewReader("<html>not a tsv</html>\n")); err == nil {
		t.Fatal("a file with no valid rows must be refused, not load as an empty table")
	}
}

func TestShadowSignalsDatacenterSkipsVerifiedBots(t *testing.T) {
	loadASNFixture(t, asnFixture)
	f := RequestFacts{IP: "3.0.0.9", UA: chromeUA, Header: http.Header{}}
	if !slices.Contains(ShadowSignals(f), "datacenter_ip") {
		t.Fatal("browser session from AWS must record datacenter_ip")
	}
	f.VerifiedGoodBot = true
	if slices.Contains(ShadowSignals(f), "datacenter_ip") {
		t.Fatal("a DNS-verified crawler on its own cloud must not be flagged")
	}
}

func browserReq(tenant, ip, method, path, dest string) RequestFacts {
	h := http.Header{}
	if dest != "" {
		h.Set("Sec-Fetch-Dest", dest)
	}
	return RequestFacts{Tenant: tenant, IP: ip, UA: chromeUA, Method: method, Path: path, Header: h}
}

func TestNoSubresourcesSignal(t *testing.T) {
	newTestRedis(t)
	var last []string
	for i := 0; i < minDocsWithoutSubresources; i++ {
		last = sessionSignals(browserReq("t1", "9.9.9.9", "GET", "/p", "document"))
		if i < minDocsWithoutSubresources-1 && slices.Contains(last, "no_subresources") {
			t.Fatalf("fired after only %d page loads", i+1)
		}
	}
	if !slices.Contains(last, "no_subresources") {
		t.Fatalf("%d page loads and no asset: got %v, want no_subresources", minDocsWithoutSubresources, last)
	}

	// Same shape from a real browser that fetched one image: never fires.
	sessionSignals(browserReq("t1", "8.8.4.4", "GET", "/logo.png", "image"))
	for i := 0; i < minDocsWithoutSubresources+2; i++ {
		if got := sessionSignals(browserReq("t1", "8.8.4.4", "GET", "/p", "document")); slices.Contains(got, "no_subresources") {
			t.Fatal("a session that loaded a subresource must not fire no_subresources")
		}
	}
}

func TestDirectSensitivePost(t *testing.T) {
	newTestRedis(t)
	// Form submission straight to /login with browser headers.
	if got := sessionSignals(browserReq("t1", "7.7.7.7", "POST", "/login", "document")); !slices.Contains(got, "direct_sensitive_post") {
		t.Fatalf("login POST with no page load: got %v, want direct_sensitive_post", got)
	}
	// A person loads the login page first.
	sessionSignals(browserReq("t1", "6.6.6.6", "GET", "/login", "document"))
	if got := sessionSignals(browserReq("t1", "6.6.6.6", "POST", "/login", "document")); slices.Contains(got, "direct_sensitive_post") {
		t.Fatal("POST after loading the form page must not fire")
	}
	// Non-sensitive routes and reads never fire.
	for _, f := range []RequestFacts{
		browserReq("t1", "5.5.5.5", "POST", "/comments", "empty"),
		browserReq("t1", "5.5.5.6", "GET", "/checkout", "document"),
	} {
		if slices.Contains(sessionSignals(f), "direct_sensitive_post") {
			t.Fatalf("%s %s must not fire direct_sensitive_post", f.Method, f.Path)
		}
	}
	// A tenant route label marks a custom path as login.
	f := browserReq("t1", "4.4.4.4", "POST", "/members/enter", "empty")
	f.RouteClass = ClassLogin
	if !slices.Contains(sessionSignals(f), "direct_sensitive_post") {
		t.Fatal("tenant-labelled login route must be treated as sensitive")
	}
}

func TestSessionIsTenantAndClientScoped(t *testing.T) {
	newTestRedis(t)
	// A page load on tenant A must not excuse the same IP+UA on tenant B.
	sessionSignals(browserReq("tenant-a", "3.3.3.3", "GET", "/login", "document"))
	if !slices.Contains(sessionSignals(browserReq("tenant-b", "3.3.3.3", "POST", "/login", "document")), "direct_sensitive_post") {
		t.Fatal("session state leaked across tenants")
	}
	// Another User-Agent behind the same IP (NAT) is another session.
	sessionSignals(browserReq("tenant-a", "2.2.2.2", "GET", "/login", "document"))
	other := browserReq("tenant-a", "2.2.2.2", "POST", "/login", "document")
	other.UA = firefoxUA
	if !slices.Contains(sessionSignals(other), "direct_sensitive_post") {
		t.Fatal("different User-Agent behind one IP must be a different session")
	}
}

func TestSessionSignalsSkipNonBrowsersAndOutages(t *testing.T) {
	newTestRedis(t)
	tool := browserReq("t1", "1.1.1.1", "POST", "/login", "")
	tool.UA = "python-requests/2.31"
	if got := sessionSignals(tool); got != nil {
		t.Fatalf("declared tools are left to other checks: got %v", got)
	}
	prev := rdb
	rdb = nil
	defer func() { rdb = prev }()
	if got := sessionSignals(browserReq("t1", "1.1.1.2", "POST", "/login", "document")); got != nil {
		t.Fatalf("no Redis must mean no session signal: got %v", got)
	}
	if RecordBeacon("t1", "1.1.1.2", chromeUA, Beacon{}) {
		t.Fatal("RecordBeacon must report failure without Redis")
	}
}

func TestBeaconSignals(t *testing.T) {
	newTestRedis(t)
	doc := func(ip string) []string { return sessionSignals(browserReq("t1", ip, "GET", "/p", "document")) }

	// Before any beacon reaches the tenant, missing beacons mean nothing:
	// the tenant may not run the script.
	for i := 0; i < minDocsWithoutBeacon+1; i++ {
		if slices.Contains(doc("10.0.0.1"), "beacon_missing") {
			t.Fatal("beacon_missing fired on a tenant that never sent a beacon")
		}
	}

	// A human session: beacons with input.
	for i := 0; i < minBeaconsWithoutInteraction; i++ {
		if !RecordBeacon("t1", "10.0.0.2", chromeUA, Beacon{Scroll: true}) {
			t.Fatal("RecordBeacon failed")
		}
	}
	if got := doc("10.0.0.2"); slices.Contains(got, "beacon_no_interaction") || slices.Contains(got, "beacon_webdriver") {
		t.Fatalf("interacting session flagged: %v", got)
	}

	// The tenant now runs the script, so the silent session is suspicious.
	if !slices.Contains(doc("10.0.0.1"), "beacon_missing") {
		t.Fatal("session with many page loads and no beacon on a beacon tenant must record beacon_missing")
	}

	// A bot that runs the script but never touches the page.
	for i := 0; i < minBeaconsWithoutInteraction; i++ {
		RecordBeacon("t1", "10.0.0.3", chromeUA, Beacon{Webdriver: true})
	}
	got := doc("10.0.0.3")
	if !slices.Contains(got, "beacon_no_interaction") || !slices.Contains(got, "beacon_webdriver") {
		t.Fatalf("got %v, want beacon_no_interaction and beacon_webdriver", got)
	}
}

// One IP rotating its User-Agent must not mint unbounded Redis keys.
func TestSessionKeysPerIPAreBounded(t *testing.T) {
	keys := map[string]bool{}
	for i := 0; i < 5000; i++ {
		keys[sessionKey("t1", "9.9.9.9", fmt.Sprintf("Mozilla/5.0 Chrome/%d.0", i))] = true
	}
	if len(keys) > uaBuckets {
		t.Fatalf("5000 UAs from one IP made %d session keys, want at most %d", len(keys), uaBuckets)
	}
	if sessionKey("t1", "9.9.9.9", chromeUA) == sessionKey("t1", "9.9.9.9", firefoxUA) {
		t.Fatal("test fixture UAs collide; pick another pair so the NAT test stays meaningful")
	}
}
