package decide

import (
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// evalModel separates one feature near-perfectly: fires bit0 -> bot
// (p≈1), else human (p≈0). Weight 20 keeps the bot's probability far
// above the block bar while a lone fired bit is still only challenged —
// the no-lone-signal rule.
func evalModel() *Model {
	return &Model{
		features:    []string{"f0", "f1", "f2"},
		weights:     []float64{20, 0, 0},
		bias:        -10,
		challengeAt: 0.5,
		blockAt:     0.9,
	}
}

func evalSamples() []EvalSample {
	return []EvalSample{
		{Sample: Sample{Fired: 1, Automated: true}, Identity: "g1", At: 100},
		{Sample: Sample{Fired: 1, Automated: true}, Identity: "g1", At: 110},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g1", At: 120},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g2", At: 130},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g2", At: 140},
		// g1 straddles the time boundary: the split must move the whole
		// identity to one side or the evaluation would grade the model on
		// a client it trained on.
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g1", At: 150},
	}
}

// The point of the split: a model must be judged on traffic from a time
// after everything it trained on, so memorising recent traffic cannot
// inflate the score. (No identity straddles the boundary here, so the
// time property can be checked strictly.)
func TestSplitTimeBasedLeavesHoldoutStrictlyLater(t *testing.T) {
	samples := []EvalSample{
		{Sample: Sample{Fired: 1, Automated: true}, Identity: "g1", At: 100},
		{Sample: Sample{Fired: 1, Automated: true}, Identity: "g1", At: 110},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g1", At: 120},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g2", At: 130},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "g2", At: 140},
	}
	train, holdout := SplitLeakageSafe(samples, 0.4)
	for _, s := range holdout {
		for _, tr := range train {
			if s.At < tr.At {
				t.Fatalf("holdout sample at %d trains on later sample at %d: not a locked later holdout", s.At, tr.At)
			}
		}
	}
	if len(holdout) == 0 || len(train) == 0 {
		t.Fatalf("split produced empty sides: train=%d holdout=%d", len(train), len(holdout))
	}
}

func TestSplitKeepsLateRepeatIdentityOutOfEarlierHoldout(t *testing.T) {
	samples := []EvalSample{
		{Identity: "repeat", At: 1},
		{Identity: "other", At: 2},
		{Identity: "fresh", At: 3},
		{Identity: "repeat", At: 4},
		{Identity: "later", At: 5},
	}
	train, holdout := SplitLeakageSafe(samples, 0.6)
	if len(train)+len(holdout) != len(samples) || len(holdout) == 0 {
		t.Fatalf("split must retain all samples and a usable holdout: train=%d holdout=%d", len(train), len(holdout))
	}
	for _, tr := range train {
		for _, h := range holdout {
			if tr.At >= h.At {
				t.Fatalf("training time %d is not before holdout time %d", tr.At, h.At)
			}
			if tr.Identity != "" && tr.Identity == h.Identity {
				t.Fatalf("identity %q occurs on both sides", tr.Identity)
			}
		}
	}
}

// A repeated identity must advance the time cut without losing samples.
func TestSplitKeepsIdentitiesWhole(t *testing.T) {
	samples := []EvalSample{
		{Identity: "g1", At: 1}, {Identity: "g1", At: 2},
		{Identity: "g2", At: 3}, {Identity: "g1", At: 4},
		{Identity: "g3", At: 5}, {Identity: "g3", At: 6},
	}
	train, holdout := SplitLeakageSafe(samples, 0.5)
	if len(holdout) != 2 {
		t.Fatalf("holdout = %d samples, want the two later g3 samples", len(holdout))
	}
	for _, h := range holdout {
		for _, tr := range train {
			if h.Identity == tr.Identity && h.Identity != "" {
				t.Fatalf("identity %q appears on both sides of the split", h.Identity)
			}
			if h.At <= tr.At {
				t.Fatalf("holdout time %d is not later than training time %d", h.At, tr.At)
			}
		}
		if h.Identity != "g3" {
			t.Errorf("unexpected holdout identity %q", h.Identity)
		}
	}
	if len(train)+len(holdout) != len(samples) {
		t.Fatalf("split lost samples: %d train + %d holdout != %d", len(train), len(holdout), len(samples))
	}
}

func TestSplitResultsDoNotShareBackingArray(t *testing.T) {
	samples := []EvalSample{{At: 1}, {At: 2}, {At: 3}, {At: 4}}
	train, holdout := SplitLeakageSafe(samples, 0.5)
	train = append(train, EvalSample{At: 99})
	if len(train) != 3 {
		t.Fatalf("grown training set = %d samples, want 3", len(train))
	}
	if holdout[0].At != 3 {
		t.Fatalf("appending training data changed the holdout to time %d", holdout[0].At)
	}
}

// A holdout fraction of 0 or 1 cannot produce two usable sides.
func TestSplitRefusesImpossibleHoldout(t *testing.T) {
	if _, holdout := SplitLeakageSafe(evalSamples(), 0); len(holdout) != 0 {
		t.Error("holdout 0 must hold out nothing")
	}
	if _, holdout := SplitLeakageSafe(evalSamples(), 1); len(holdout) == 0 {
		t.Error("holdout 1 must hold out everything")
	}
}

// Recall here means what the plan defines: bots stopped by any
// intervention (block, challenge, deceive, rate-limit), not only hard
// blocks.
func TestEvalMetricsCountInterventionRecall(t *testing.T) {
	m := evalModel() // bit0 -> p~1 -> block (needs >=2 fired) ... single bit only challenges
	samples := []EvalSample{
		{Sample: Sample{Fired: 1, Automated: true}, Identity: "b1", At: 10},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "h1", At: 11},
		{Sample: Sample{Fired: 0, Automated: false}, Identity: "h2", At: 12},
	}
	got := Evaluate(m, samples)
	// The lone fired bit is challenged (never blocked alone) — that is
	// still an intervention and must count as recalled.
	if got.Recall != 1 {
		t.Errorf("Recall = %v, want 1 (the bot was challenged)", got.Recall)
	}
	if got.Samples != 3 || got.Bots != 1 || got.Humans != 2 {
		t.Errorf("counts wrong: %+v", got)
	}
	// Brier: prediction for the human is p≈0 (perfect), for the bot p≈1
	// (perfect) -> near 0.
	if got.Brier > 0.01 {
		t.Errorf("Brier = %v, want near 0 for a separating model", got.Brier)
	}
}

// The plan's hard-block human FPR is the number that ends pilots: it
// must be visible separately from challenge burden.
func TestEvalMetricsSeparateHardBlockFromChallenge(t *testing.T) {
	m := evalModel()
	samples := []EvalSample{
		// Two fired bits clear the block bar for this model.
		{Sample: Sample{Fired: 3, Automated: false}, Identity: "h1", At: 10},
		{Sample: Sample{Fired: 1, Automated: false}, Identity: "h2", At: 11},
	}
	got := Evaluate(m, samples)
	if got.HumanHardBlockFPR != 0.5 {
		t.Errorf("HumanHardBlockFPR = %v, want 0.5 (1 of 2 humans hard-blocked)", got.HumanHardBlockFPR)
	}
	if got.HumanChallengeBurden != 1.0 {
		t.Errorf("HumanChallengeBurden = %v, want 1.0 (every human got a non-allow decision)", got.HumanChallengeBurden)
	}
}

// Calibration buckets: a bucket's average predicted probability should
// approach its observed bot rate. With one perfectly separated bucket we
// can only pin the structure, not calibration quality.
func TestCalibrationBucketsShape(t *testing.T) {
	m := evalModel()
	samples := evalSamples()
	buckets := CalibrationBuckets(m, samples, 5)
	if len(buckets) != 5 {
		t.Fatalf("got %d buckets, want 5", len(buckets))
	}
	total := 0
	for _, b := range buckets {
		if b.Count < 0 || b.AvgProbability < 0 || b.AvgProbability > 1 {
			t.Errorf("bucket out of range: %+v", b)
		}
		total += b.Count
	}
	if total != len(samples) {
		t.Errorf("buckets hold %d samples, want %d", total, len(samples))
	}
}

// The rule baseline must reproduce the live scorer exactly: rules sum
// the fixed weights of fired checks and decide on the same thresholds
// the proxy uses. If this drifts from signals.Evaluate, the promotion
// comparison is against a scorer the proxy does not run.
func TestRuleDecisionsMatchSignalScoring(t *testing.T) {
	names := signals.FeatureNames()
	// Build a fired mask with scripting_tool (weight 100 -> hard block).
	fired, err := Vector(names, []string{"scripting_tool"})
	if err != nil {
		t.Fatalf("Vector: %v", err)
	}
	if got := ruleDecide(fired); got != signals.DecisionBlock {
		t.Errorf("ruleDecide(scripting_tool) = %v, want block at weight 100", got)
	}
	// One mid-weight signal challenges; nothing allows cleanly.
	fired2, _ := Vector(names, []string{"header_anomaly"})
	if got := ruleDecide(fired2); got != signals.DecisionChallenge {
		t.Errorf("ruleDecide(header_anomaly) = %v, want challenge", got)
	}
	if got := ruleDecide(0); got != signals.DecisionAllow {
		t.Errorf("ruleDecide(0) = %v, want allow", got)
	}
}

// Promotion test: the plan's gate is recall and human harm against the
// rules, not a single accuracy number.
func TestPromotionReportComparesAgainstRules(t *testing.T) {
	m := evalModel()
	samples := evalSamples()
	rep := RunPromotion(m, samples)
	if rep.Rules.Holdout == nil || rep.Model.Holdout == nil {
		t.Fatalf("promotion report missing holdout metrics: %+v", rep)
	}
	if rep.Decision == "" {
		t.Error("PromotionReport must always state a decision")
	}
	// The separating model beats the rules on this set (rules block
	// nothing here since these samples fire no heavy signals... except
	// the model's behaviour is measured on the same holdout).
	if rep.Model.Holdout.Recall < rep.Rules.Holdout.Recall && rep.Decision == promote {
		t.Errorf("model recalled less than rules but was promoted: %+v", rep)
	}
}

func TestPromotionRefusesATinyHoldout(t *testing.T) {
	rep := RunPromotion(evalModel(), []EvalSample{
		{Sample: Sample{Fired: 1, Automated: true}, Identity: "g1", At: 10},
	})
	if rep.Decision != hold {
		t.Errorf("decision = %q, want hold on a tiny holdout", rep.Decision)
	}
}

func TestPromotionRefusesHoldoutWithoutHumans(t *testing.T) {
	samples := make([]EvalSample, 250)
	for i := range samples {
		samples[i] = EvalSample{Sample: Sample{Fired: 1, Automated: true}, At: int64(i)}
	}
	rep := RunPromotion(evalModel(), samples)
	if rep.Decision != hold {
		t.Fatalf("decision = %q with %d holdout humans, want hold", rep.Decision, rep.Model.Holdout.Humans)
	}
}

func TestCalibrationBucketsRefusesUnsupportedCount(t *testing.T) {
	if got := CalibrationBuckets(evalModel(), []EvalSample{{Sample: Sample{Fired: 1}, At: 1}}, 65); got != nil {
		t.Fatalf("65 buckets = %d entries, want nil instead of indexing past the 64-bucket cap", len(got))
	}
}
