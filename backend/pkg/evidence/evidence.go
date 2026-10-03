package evidence

import (
	"sync"
	"time"
)

// How much history the trail keeps. Both limits exist because this
// runs against live adversarial traffic: the size cap stops memory
// growing with request volume, and the age cap stops us holding
// visitor records longer than answering a complaint needs.
const (
	trailSize   = 1000
	trailMaxAge = 24 * time.Hour
)

// Evidence is the record of one decision: enough to answer "why was
// this request stopped?" days later, and nothing more about the
// visitor than the decision itself already used.
type Evidence struct {
	Time    time.Time `json:"time"`
	JA4     string    `json:"ja4"`
	Signals []string  `json:"signals"`
	// ShadowSignals are observed candidates that do not contribute to Score
	// or Decision until measured on real traffic and explicitly promoted.
	ShadowSignals []string `json:"shadowSignals,omitempty"`
	Score         int      `json:"score"`
	Decision      string   `json:"decision"`
	// Enforced is false when the decision was only recorded, not acted
	// on (shadow mode). Without it a reader cannot tell a real block
	// from one that never happened.
	Enforced bool `json:"enforced"`
	// Model is what the learned model (pkg/decide) would have decided,
	// present only when one is loaded. It never affects Decision: the
	// rule scorer above is what actually ran. Recording both is how a
	// model earns the right to enforce, by being compared against the
	// rules on real traffic first.
	Model *ModelOpinion `json:"model,omitempty"`
	// Policy records the matched tenant rule, proposed action, and whether
	// an activated revision changed the actual decision. Legacy account
	// rules remain shadow-only.
	Policy *PolicyOpinion `json:"policy,omitempty"`
}

// PolicyOpinion explains one request's policy evaluation. An empty RuleID means no rule
// matched.
type PolicyOpinion struct {
	Matched           bool   `json:"matched"`
	RuleID            string `json:"ruleId,omitempty"`
	RuleName          string `json:"ruleName,omitempty"`
	Action            string `json:"action,omitempty"`
	Version           int    `json:"version,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Class             string `json:"class,omitempty"`
	Method            string `json:"method,omitempty"`
	VerifiedAgent     string `json:"verifiedAgent,omitempty"`
	Allowlisted       bool   `json:"allowlisted"`
	BaselineDecision  string `json:"baselineDecision,omitempty"`
	ProposedDecision  string `json:"proposedDecision,omitempty"`
	EffectiveDecision string `json:"effectiveDecision,omitempty"`
	Enforced          bool   `json:"enforced"`
	SkippedReason     string `json:"skippedReason,omitempty"`
}

// ModelOpinion is the learned model's view of one request. It is a plain
// record rather than the decide.Prediction itself so the evidence trail
// stays a description of what happened and does not depend on the
// scoring package.
type ModelOpinion struct {
	Decision    string  `json:"decision"`
	Probability float64 `json:"probability"`
	Confidence  float64 `json:"confidence"`
	// Reasons is each fired check's push on the decision, strongest
	// first. This is what answers "why" for a model decision, the same
	// way Signals does for the rule score.
	Reasons []ModelReason `json:"reasons,omitempty"`
}

// ModelReason is one check's contribution to a model decision, in
// log-odds. Positive argued the request was automated.
type ModelReason struct {
	Feature string  `json:"feature"`
	Weight  float64 `json:"weight"`
}

// Trail holds the most recent decisions in a fixed-size ring buffer,
// oldest overwritten first. Reads always come from memory; when a Writer
// is attached the same records are also queued to a durable Sink, and
// Load seeds the buffer from it at startup so a restart does not empty
// the history a customer can see.
type Trail struct {
	mu     sync.Mutex
	buf    []Evidence
	next   int
	n      int
	maxAge time.Duration
	now    func() time.Time

	writer   *Writer
	tenantID string
}

func NewTrail() *Trail {
	return newTrail(trailSize, trailMaxAge)
}

// newTrail refuses a zero size rather than handing back a Trail that
// divides by zero on its first record — a panic here would be in the
// request path, so it fails at construction instead.
func newTrail(size int, maxAge time.Duration) *Trail {
	if size < 1 {
		size = 1
	}
	return &Trail{buf: make([]Evidence, size), maxAge: maxAge, now: time.Now}
}

// EnablePersistence attaches a durable Writer to this tenant's trail.
// It must be called before the tenant serves traffic.
func (t *Trail) EnablePersistence(tenantID string, w *Writer) {
	if t == nil || w == nil {
		return
	}
	t.mu.Lock()
	t.tenantID = tenantID
	t.writer = w
	t.mu.Unlock()
}

// Record stamps a decision with the time it happened and stores it,
// dropping the oldest record once the buffer is full. When a Writer is
// attached the record is also queued for durable storage, without
// blocking on the database.
func (t *Trail) Record(e Evidence) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e = cloneEvidence(e)
	e.Time = t.now()
	t.buf[t.next] = e
	t.next = (t.next + 1) % len(t.buf)
	if t.n < len(t.buf) {
		t.n++
	}
	if t.writer != nil {
		t.writer.enqueue(t.tenantID, e)
	}
}

// Load seeds the ring from durable storage, oldest record first, so a
// freshly started process shows the same history as before it restarted.
// It is meant to run once, before the tenant serves traffic; it does not
// restamp times or queue what it loads back to the store.
func (t *Trail) Load(records []Evidence) {
	if t == nil || len(records) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf = make([]Evidence, len(t.buf))
	t.next, t.n = 0, 0
	// Callers pass newest first, so insert from the back to keep the ring
	// in chronological order.
	for i := len(records) - 1; i >= 0; i-- {
		e := cloneEvidence(records[i])
		t.buf[t.next] = e
		t.next = (t.next + 1) % len(t.buf)
		if t.n < len(t.buf) {
			t.n++
		}
	}
}

// Recent returns the newest records first, skipping any past the
// retention window. limit <= 0 means "everything still held".
func (t *Trail) Recent(limit int) []Evidence {
	t.mu.Lock()
	defer t.mu.Unlock()

	if limit <= 0 || limit > t.n {
		limit = t.n
	}
	cutoff := t.now().Add(-t.maxAge)

	out := make([]Evidence, 0, limit)
	for i := 0; i < t.n && len(out) < limit; i++ {
		e := t.buf[(t.next-1-i+2*len(t.buf))%len(t.buf)]
		// Records sit in time order, so the first one past the window
		// means every older one is too.
		if e.Time.Before(cutoff) {
			break
		}
		out = append(out, cloneEvidence(e))
	}
	return out
}

func cloneEvidence(e Evidence) Evidence {
	e.Signals = append([]string(nil), e.Signals...)
	e.ShadowSignals = append([]string(nil), e.ShadowSignals...)
	if e.Policy != nil {
		p := *e.Policy
		e.Policy = &p
	}
	if e.Model != nil {
		m := *e.Model
		m.Reasons = append([]ModelReason(nil), m.Reasons...)
		e.Model = &m
	}
	return e
}
