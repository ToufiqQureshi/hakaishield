package signals

import "strings"

// h2PseudoOrder is the HTTP/2 pseudo-header order each browser engine
// sends, the last field of the greeting fingerprint. It is set by the
// engine's network stack, not by page scripts or the User-Agent, so a
// client that claims one browser but greets like another is lying about
// its stack.
//
// Only verified captures are listed (2026-09-29, Windows, served by
// core.WireHTTP2): Chrome 153 headless and headful, Edge 154 and
// Patchright-driven Chrome all sent "masp"; Firefox 142 sent "mpas".
// Safari is absent because it has not been captured; a guessed value
// would flag every Safari user.
var h2PseudoOrder = map[string]string{
	"chromium": "masp",
	"firefox":  "mpas",
}

// h2FamilyMismatch reports a browser User-Agent whose HTTP/2 greeting
// belongs to a different engine, such as a Python or Go client wearing a
// Chrome UA. It cannot see a real browser driven by automation
// (Patchright's greeting is Chrome's); that needs behavioural evidence.
func h2FamilyMismatch(ua, h2 string) bool {
	if h2 == "" || !claimsBrowser(ua) {
		return false
	}
	want := h2PseudoOrder[uaEngine(ua)]
	if want == "" {
		return false
	}
	pseudo := h2[strings.LastIndexByte(h2, '|')+1:]
	return pseudo != "" && pseudo != want
}

// uaEngine names the network stack a User-Agent claims. Every iOS
// browser must use WebKit's stack whatever its brand, so iOS UAs are
// never compared.
func uaEngine(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"),
		strings.Contains(ua, "CriOS/"), strings.Contains(ua, "FxiOS/"), strings.Contains(ua, "EdgiOS/"):
		return ""
	case strings.Contains(ua, "Firefox/"):
		return "firefox"
	case chromiumUAMajor.MatchString(ua):
		return "chromium"
	default:
		return ""
	}
}
