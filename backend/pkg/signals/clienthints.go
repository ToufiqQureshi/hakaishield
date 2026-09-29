package signals

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	chromiumUAMajor   = regexp.MustCompile(`(?:Chrome|Chromium)/([0-9]{1,3})`)
	chromiumHintMajor = regexp.MustCompile(`"(?:Chromium|Google Chrome|Microsoft Edge)";v="([0-9]{1,3})"`)
	// hintBrandEntry captures every brand name in a Sec-CH-UA list, real or
	// GREASE, unlike chromiumHintMajor which only matches the known real ones.
	hintBrandEntry = regexp.MustCompile(`"([^"]*)"\s*;\s*v="[0-9]+"`)
	// greaseBrand matches Chromium's randomized GREASE brand (e.g.
	// "Not/A)Brand", "Not;A=Brand"). The words are fixed, the punctuation
	// between them is deliberately randomized per spec, so match on the
	// words alone.
	greaseBrand = regexp.MustCompile(`(?i)not[^a-z0-9]*a[^a-z0-9]*brand`)
)

// clientHintSignals reports Chromium client-hint contradictions. Client
// hints can expose a forged Chromium version, but browser variants and
// privacy tools need real traffic review before this can affect enforcement.
func clientHintSignals(f RequestFacts) []string {
	if !claimsBrowser(f.UA) || len(f.UA) > 512 {
		return nil
	}
	uaMatch := chromiumUAMajor.FindStringSubmatch(f.UA)
	if len(uaMatch) != 2 {
		return nil
	}
	uaMajor, _ := strconv.Atoi(uaMatch[1])
	if uaMajor == 0 {
		return nil
	}
	var found []string
	if hint := f.Header.Get("Sec-CH-UA"); hint != "" && len(hint) <= 512 {
		for _, match := range chromiumHintMajor.FindAllStringSubmatch(hint, 4) {
			major, _ := strconv.Atoi(match[1])
			if major != uaMajor {
				found = append(found, "client_hint_major_mismatch")
				break
			}
		}
		// Every Chromium build since v90 injects a randomized GREASE brand
		// into this list so servers cannot hardcode the set (see
		// GREASE_BRAND_PATTERN prior art in browser fingerprinting tooling).
		// A hand-built Sec-CH-UA header that names only real brands is a
		// forged client hint, not a real Chromium build.
		brands := hintBrandEntry.FindAllStringSubmatch(hint, 8)
		if len(brands) > 0 {
			hasGrease := false
			for _, b := range brands {
				if greaseBrand.MatchString(b[1]) {
					hasGrease = true
					break
				}
			}
			if !hasGrease {
				found = append(found, "client_hint_missing_grease_brand")
			}
		}
	}
	if hint := f.Header.Get("Sec-CH-UA-Platform"); len(hint) <= 64 {
		claimed := chromiumUAPlatform(f.UA)
		observed := normalizeHintPlatform(hint)
		if claimed != "" && observed != "" && claimed != observed {
			found = append(found, "client_hint_platform_mismatch")
		}
	}
	if mobile := f.Header.Get("Sec-CH-UA-Mobile"); len(mobile) <= 4 {
		switch mobile {
		case "?1":
			if p := chromiumUAPlatform(f.UA); p == "windows" || p == "macos" || p == "chromeos" || p == "linux" {
				found = append(found, "client_hint_mobile_mismatch")
			}
		case "?0":
			if strings.Contains(f.UA, "Mobile") || strings.Contains(f.UA, "iPhone") || strings.Contains(f.UA, "iPod") {
				found = append(found, "client_hint_mobile_mismatch")
			}
		}
	}
	return found
}

func chromiumUAPlatform(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT"):
		return "windows"
	case strings.Contains(ua, "Android"):
		return "android"
	case strings.Contains(ua, "CrOS"):
		return "chromeos"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"):
		return "ios"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		return "macos"
	case strings.Contains(ua, "Linux"):
		return "linux"
	default:
		return ""
	}
}

func normalizeHintPlatform(hint string) string {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(hint), `"`)) {
	case "windows":
		return "windows"
	case "android":
		return "android"
	case "chrome os":
		return "chromeos"
	case "ios":
		return "ios"
	case "macos":
		return "macos"
	case "linux":
		return "linux"
	default:
		return ""
	}
}
