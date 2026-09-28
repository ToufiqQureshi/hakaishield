package signals

import "strings"

// browserMarkers only appear in a real browser's own User-Agent.
var browserMarkers = []string{"Chrome/", "Firefox/", "Safari/", "Edg/"}

// crawlerMarkers mark a User-Agent that openly admits it's a bot
// (Googlebot, etc). That's not a lie, so it's not a mismatch target.
var crawlerMarkers = []string{"bot", "spider", "crawl"}

// scriptingMarkers match generic HTTP libraries, CLI tools, headless
// automation frameworks, load-test tools, and recon/vuln scanners —
// nothing a real browser's own UA string ever contains, so this list
// only grows by adding another honest self-declaration, never by
// guessing at a real browser variant (CLAUDE.md Section 14: a false
// entry here would hard-block that browser's users outright, since
// scripting_tool is one of the two checks allowed to score 100 alone).
//
// This is a blunt, high-confidence instrument: it only catches a tool
// that hasn't bothered to fake its User-Agent. A stealth automation
// stack (Patchright-class) that drives a real, unmodified browser
// engine sends a real browser's UA and is invisible to this check by
// construction — see docs/STATUS.md, "What a good bot still gets past". Closing that
// gap is behavioral/browser-integrity scoring (Phase 3), not a bigger
// list here.
var scriptingMarkers = []string{
	// generic HTTP client libraries
	"curl", "wget", "httpie",
	"python-urllib", "python-requests", "python-httpx", "aiohttp",
	"go-http-client", "go-resty",
	"okhttp", "java/", "apache-httpclient",
	"node-fetch", "axios/",
	"libwww-perl", "guzzlehttp", "urllib3", "libcurl",

	// API testing / load-testing tools
	"postman", "insomnia",
	"locust", "jmeter", "gatling", "k6", "vegeta", "wrk/", "artillery", "siege/", "apachebench",

	// headless browser / automation frameworks
	"headless", "phantomjs", "selenium", "webdriver",
	"playwright", "patchright", "puppeteer",
	"nightmare", "casperjs", "splash",

	// scraping frameworks
	"scrapy", "mechanize",

	// recon / vulnerability scanners (not a bot in the scraping sense,
	// but never a real visitor's browser either)
	"nmap", "nikto", "sqlmap", "nuclei", "masscan", "zgrab",
	"gobuster", "dirbuster", "wpscan", "acunetix", "nessus",
	"burpsuite", "zaproxy",
}

// claimsBrowser reports whether ua claims to be a real browser,
// rather than a script or a bot that's already honest about itself.
func claimsBrowser(ua string) bool {
	if claimsCrawler(ua) {
		return false
	}
	for _, m := range browserMarkers {
		if strings.Contains(ua, m) {
			return true
		}
	}
	return false
}

// claimsCrawler detects a crawler declaration without trusting it as proof
// that the request comes from an approved search engine.
func claimsCrawler(ua string) bool {
	lower := strings.ToLower(ua)
	for _, marker := range crawlerMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// IsScriptingTool returns true if the User-Agent identifies as a generic
// HTTP client or scripting language library.
func IsScriptingTool(ua string) bool {
	lower := strings.ToLower(ua)
	for _, m := range scriptingMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// UAMismatch flags a request that claims to be a browser but whose
// TLS handshake says otherwise. Real, current browsers only speak
// TLS 1.2/1.3 and never trigger the record-fragmentation evasion
// (JA4Unreadable) — so either one, from something calling itself
// Chrome or Firefox, means the User-Agent is faked.
func UAMismatch(ua, ja4 string) bool {
	if !claimsBrowser(ua) {
		return false // not claiming to be a browser, nothing to catch
	}
	if ja4 == "" {
		return false // no TLS to compare against - fail open
	}
	if ja4 == JA4Unreadable {
		return true
	}
	if len(ja4) < 3 {
		return false
	}
	version := ja4[1:3] // e.g. "13" in "t13d1516h2_..."
	if version == "10" || version == "11" {
		return true
	}
	// Browser impersonation: UA claims browser, but JA4 matches a known scraper library.
	if isScraper, _ := IsKnownScraperJA4(ja4); isScraper {
		return true
	}
	return false
}
