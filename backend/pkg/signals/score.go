package signals

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
)

// Decision is the outcome scoring produces for a request: challenge it
// (the default, and the only non-block outcome today), refuse it, or —
// when the tenant enables deception — forward it with a deceived flag.
// No single signal may pick DecisionBlock by itself (CLAUDE.md Section 6)
// — only combined evidence crosses that threshold.
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionChallenge
	DecisionBlock
	DecisionDeceive
	DecisionRateLimit
)

func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionChallenge:
		return "challenge"
	case DecisionBlock:
		return "block"
	case DecisionDeceive:
		return "deceive"
	case DecisionRateLimit:
		return "rate_limit"
	default:
		return "unknown"
	}
}

// Signal weights and thresholds. Fixed for now, not yet configurable
// per client — that's docs/STATUS.md item 11. fragmentedWeight and
// uaMismatchWeight are deliberately equal and additive: a fragmented
// handshake (fingerprint-layer anomaly, true regardless of what the
// client claims to be) and a UA mismatch (consistency-layer lie,
// which needs a browser claim to be a lie) are different findings
// from different layers, even though a fragmented handshake is one of
// UAMismatch's own inputs — see docs/DECISIONS.md for why that's not
// double-counting the same fact.
const (
	fragmentedWeight    = 50
	uaMismatchWeight    = 50
	headerAnomalyWeight = 25
	crawlPatternWeight  = 50
	honeypotWeight      = 50
	blockThreshold      = 100
)

// Known malicious JA4 fingerprints. Every entry must be verified against
// real captures before being added — blocking a JA4 shared with a real
// browser family would block every legitimate user on that browser
// (false positive, CLAUDE.md Section 14). Placeholder hashes without
// verified captures must never ship.
var badJA4Hashes = map[string]bool{
	"t12d190800_4464c1bd5eb7_b3394627b738": true, // Python requests (verified capture)
}

// RequestFacts is the server-observed view of one request that scoring
// uses. Keeping them in one struct avoids a growing parameter list, and
// makes explicit that every field is read without trusting a client
// beyond what it can actually control (path, TLS fingerprint and
// connection come from the connection, not the visitor).
type RequestFacts struct {
	IP     string
	JA4    string
	UA     string
	Header http.Header
	Path   string
	// Method is the HTTP method, used to classify the endpoint for
	// velocity buckets. It is client-controlled and only ever compared
	// case-insensitively, never trusted as a decision input by itself.
	Method string
	// RouteClass is a trusted, activated tenant-policy override for this
	// exact path. The visitor cannot set it; unknown values are ignored.
	RouteClass string
	// Tenant scopes per-customer state (currently the honeypot trap) so
	// one customer's traffic can never influence another's decisions.
	Tenant string
	// VerifiedGoodBot is set only after server-side reverse/forward DNS
	// verification. A claimed crawler UA alone never grants this exemption.
	VerifiedGoodBot bool
}

// Evaluation is the complete result of evaluating one request. Keeping the
// score and fired signal names together prevents request-path callers from
// running stateful checks twice and producing evidence that disagrees with the
// decision.
type Evaluation struct {
	Score   int
	Signals []string
	// Fired is the same result as Signals in machine-readable form: bit i
	// is set when checks[i] fired, in FeatureNames order. A learned model
	// (pkg/decide) reads this instead of the names so it scores exactly the
	// checks that ran, and a bitmask keeps that free of allocation in the
	// request path. It holds 32 checks; TestChecksFitFeatureMask fails if
	// the list ever outgrows that, since the 33rd check would otherwise
	// drop out of the evidence silently.
	Fired uint32
}

// checks is the single list every scoring check lives in, so a score
// and the explanation shown for it can never disagree — adding a check
// in one place and forgetting the other would make the evidence trail
// lie about why a request was stopped.
var checks = []struct {
	name   string
	weight int
	fired  func(f RequestFacts) bool
}{
	{"fragmented_handshake", fragmentedWeight, func(f RequestFacts) bool { return f.JA4 == JA4Unreadable }},
	{"ua_mismatch", uaMismatchWeight, func(f RequestFacts) bool { return UAMismatch(f.UA, f.JA4) }},
	// header_anomaly is a weaker consistency signal than ua_mismatch
	// (privacy tools and unusual-but-real clients can drop browser
	// headers), so it is weighted below the challenge/block bar. It can
	// never be the only reason for a block: by itself it scores 25, and
	// every path to 100 already fires stronger signals (CLAUDE.md §6).
	{"header_anomaly", headerAnomalyWeight, func(f RequestFacts) bool { return HeaderAnomaly(f.UA, f.Header) }},
	{"ja4_blocklist", 100, func(f RequestFacts) bool {
		isScraper, _ := IsKnownScraperJA4(f.JA4)
		return isScraper || badJA4Hashes[f.JA4]
	}},
	{"scripting_tool", 100, func(f RequestFacts) bool { return IsScriptingTool(f.UA) }},
	{"velocity_spike", 50, func(f RequestFacts) bool {
		return checkVelocitySpikeForClass(f.Tenant, f.IP, f.Path, f.Method, f.RouteClass)
	}},
	{"ja4_velocity_spike", 50, func(f RequestFacts) bool { return checkJA4VelocitySpike(f.Tenant, f.JA4) }},
	// crawl_pattern is a server-observed behaviour signal, not a client
	// claim: a real browser's requests per page look nothing like a
	// scraper walking many distinct URLs quickly with no subresources.
	{"crawl_pattern", crawlPatternWeight, func(f RequestFacts) bool { return CrawlPatternSuspected(f) }},
	// honeypot_trap fires for a caller that fetched the invisible trap
	// link (pkg/deception injects it; guard.go records the fetch).
	// Following a display:none, aria-hidden, nofollow link is strong
	// evidence of DOM-walking automation — but deliberately not 100.
	// A screen reader or an over-eager browser prefetch can reach a
	// hidden link too, and those are real people (CLAUDE.md Section
	// 14). At 50 a lone trip is challenged, which a human recovers
	// from, while a real scraper trips this *and* the handshake, UA or
	// crawl-pattern signals and crosses the block bar on evidence.
	{"honeypot_trap", honeypotWeight, func(f RequestFacts) bool {
		return HoneypotTripped(f.Tenant, f.IP, f.JA4)
	}},
}

// Evaluate runs every scoring check exactly once for one request. Some checks
// update bounded Redis counters, so callers that need both a score and an
// explanation must use this method rather than calling Score and Analyze
// separately.
func Evaluate(f RequestFacts) Evaluation {
	e := Evaluation{}
	for i, c := range checks {
		if c.fired(f) {
			e.Score += c.weight
			e.Signals = append(e.Signals, c.name)
			e.Fired |= 1 << i
		}
	}
	return e
}

// FeatureNames lists every check in the order Evaluation.Fired uses its
// bits. A model trained against one order must refuse a binary whose order
// differs, so this is the name list pkg/decide validates a saved model
// against — see decide.Load.
func FeatureNames() []string {
	names := make([]string, len(checks))
	for i, c := range checks {
		names[i] = c.name
	}
	return names
}

// Score combines a request's known signals into one risk score. Prefer
// Evaluate when the caller also needs the fired signal names.
func Score(f RequestFacts) int {
	return Evaluate(f).Score
}

// Analyze names the checks that fired for a request, so the evidence
// trail can answer "why was this stopped?" and not just "how much." Prefer
// Evaluate when the caller also needs the score.
func Analyze(f RequestFacts) []string {
	return Evaluate(f).Signals
}

// Decide turns a score into an outcome using the balanced policy by default.
func Decide(score int) Decision {
	return DecideWithPolicy(score, config.PolicyBalanced)
}

// DecideWithPolicy turns a score into an outcome according to the policy mode.
// PolicyBalanced: clean traffic (score 0) is allowed passively without delay,
// elevated risk (1-99) is challenged, and score >= 100 is blocked.
// PolicyStrict: all unscored traffic (< 100) receives the mandatory interstitial challenge.
func DecideWithPolicy(score int, policy config.PolicyMode) Decision {
	if score >= blockThreshold {
		return DecisionBlock
	}
	if policy == config.PolicyStrict {
		return DecisionChallenge
	}
	if score == 0 {
		return DecisionAllow
	}
	return DecisionChallenge
}

// HardBlockThreshold is the score at which scoring blocks a request
// outright, regardless of policy mode. Other packages that need to
// reason about "stricter than a block" (e.g. pkg/policy's DECEIVE
// guardrail) call this instead of hardcoding 100, so the two constants
// can never silently drift apart.
func HardBlockThreshold() int {
	return blockThreshold
}

// FeatureVersion identifies the check list this build runs, as a short
// stable hash of the names in FeatureNames order.
//
// A stored training sample is a positional bitmask, so it is meaningless
// without knowing which list produced it: reorder or rename a check and
// every older row silently starts describing different signals. Samples
// carry this string so a trainer can refuse the ones captured against a
// different build, the same way decide.Load refuses a mismatched model.
func FeatureVersion() string {
	sum := sha256.Sum256([]byte(strings.Join(FeatureNames(), "\n")))
	return hex.EncodeToString(sum[:6])
}

// Feature names that other packages need to refer to by name rather than
// by position. Using the constant keeps a rename from silently turning a
// lookup into a miss.
const FeatureHoneypotTrap = "honeypot_trap"

// FeatureBit returns the Evaluation.Fired bit for a named check.
//
// It reports false for a name this build does not run, so a caller that
// looks up a renamed check fails visibly instead of masking with zero
// and silently doing nothing.
func FeatureBit(name string) (uint32, bool) {
	for i, c := range checks {
		if c.name == name {
			return 1 << i, true
		}
	}
	return 0, false
}
