package decide

import (
	"fmt"
	"sort"

	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// EvalSample is one labelled request plus the two fields leakage-safe
// evaluation needs and training deliberately ignores: which pseudonymous
// identity produced it, and when it was observed.
//
// The identity is the same capped (tenant, IP, JA4) string label
// collection uses — it is never persisted with training rows, and here
// it exists only so evaluation can keep one client's sessions on one
// side of the split (split by time and identity/session; see
// docs/ARCHITECTURE.md, Learned model).
type EvalSample struct {
	Sample
	Identity string
	// At is the observation time as any monotone counter (unix seconds
	// are fine). Only ordering matters.
	At int64
}

// Metrics is the evaluation report for one side of a split. It reports
// what the plan asks for instead of a single accuracy number: recall
// (bots receiving any intervention), hard-block human false-positive
// rate, and challenge burden are the three numbers a pilot lives or dies
// by.
type Metrics struct {
	Samples int
	Bots    int
	Humans  int
	// Recall is the intervention recall: bots stopped by any non-allow
	// decision. A challenge counts; only silent passage does not.
	Recall float64
	// HumanHardBlockFPR is the share of humans the evaluator would
	// refuse outright. This is the plan's hard harm number.
	HumanHardBlockFPR float64
	// HumanChallengeBurden is the share of humans receiving any
	// non-allow decision — challenges, rate limits, everything.
	HumanChallengeBurden float64
	// Brier is the mean squared error of the probability estimates, the
	// calibration-sensitive companion to log loss.
	Brier float64
}

// Evaluate scores one model against labelled requests. An empty set
// returns zeroed metrics; callers decide whether that is worth acting on
// (PromotionReport refuses to act on it).
func Evaluate(m *Model, samples []EvalSample) Metrics {
	mt := Metrics{Samples: len(samples)}
	if len(samples) == 0 {
		return mt
	}
	blocked := 0
	recalled := 0
	for _, s := range samples {
		p := m.Predict(s.Fired)
		brier := p.Probability
		if s.Automated {
			mt.Bots++
			brier = 1 - brier
			if p.Decision != signals.DecisionAllow {
				recalled++
			}
		} else {
			mt.Humans++
			if p.Decision == signals.DecisionBlock {
				blocked++
			}
			if p.Decision != signals.DecisionAllow {
				mt.HumanChallengeBurden += 1
			}
		}
		mt.Brier += brier * brier
	}
	if mt.Bots > 0 {
		mt.Recall = float64(recalled) / float64(mt.Bots)
	}
	if mt.Humans > 0 {
		mt.HumanHardBlockFPR = float64(blocked) / float64(mt.Humans)
		mt.HumanChallengeBurden /= float64(mt.Humans)
	}
	mt.Brier /= float64(len(samples))
	return mt
}

// CalibrationBucket is one slice of predicted probability with the
// observed bot rate inside it. A calibrated model's AvgProbability
// tracks its BotRate bucket by bucket; the plan requires these buckets
// before Confidence may be read as anything but a margin.
type CalibrationBucket struct {
	AvgProbability float64
	BotRate        float64
	Count          int
}

// CalibrationBuckets buckets samples by predicted probability in [0,1]
// into buckets equal-width slices and reports the average prediction and
// observed bot rate per bucket. Counts outside 1..64 return nil.
func CalibrationBuckets(m *Model, samples []EvalSample, buckets int) []CalibrationBucket {
	if buckets <= 0 || buckets > 64 || len(samples) == 0 {
		return nil
	}
	out := make([]CalibrationBucket, buckets)
	var probSum, botSum [64]float64 // supports up to 64 buckets
	for _, s := range samples {
		p := m.Predict(s.Fired).Probability
		idx := int(p * float64(buckets))
		if idx >= buckets {
			idx = buckets - 1
		}
		probSum[idx] += p
		botSum[idx] += label(s.Automated)
		out[idx].Count++
	}
	for i := range out {
		if out[i].Count > 0 {
			out[i].AvgProbability = probSum[i] / float64(out[i].Count)
			out[i].BotRate = botSum[i] / float64(out[i].Count)
		}
	}
	return out
}

// ruleDecide reproduces the live rule scorer's decision from a fired
// mask: the sum of fixed check weights against the same thresholds the
// proxy enforces. It exists so the promotion comparison pits the model
// against the scorer the proxy actually runs.
func ruleDecide(fired uint32) signals.Decision {
	score := signals.ScoreFromFired(fired)
	if score >= signals.HardBlockThreshold() {
		return signals.DecisionBlock
	}
	if score == 0 {
		return signals.DecisionAllow
	}
	return signals.DecisionChallenge
}

// RulesScored evaluates the rule scorer as if it were a model, using the
// same metrics, so PromotionReport compares like with like.
func RulesScored(samples []EvalSample) Metrics {
	mt := Metrics{Samples: len(samples)}
	if len(samples) == 0 {
		return mt
	}
	blocked := 0
	recalled := 0
	for _, s := range samples {
		d := ruleDecide(s.Fired)
		if s.Automated {
			mt.Bots++
			if d != signals.DecisionAllow {
				recalled++
			}
		} else {
			mt.Humans++
			if d == signals.DecisionBlock {
				blocked++
			}
			if d != signals.DecisionAllow {
				mt.HumanChallengeBurden += 1
			}
		}
	}
	if mt.Bots > 0 {
		mt.Recall = float64(recalled) / float64(mt.Bots)
	}
	if mt.Humans > 0 {
		mt.HumanHardBlockFPR = float64(blocked) / float64(mt.Humans)
		mt.HumanChallengeBurden /= float64(mt.Humans)
	}
	return mt
}

// SplitLeakageSafe divides samples into train and holdout with the two
// properties the plan demands: the holdout is strictly later than every
// training sample (a locked later holdout), and no identity appears on
// both sides (sessions of one client cannot be graded on a client the
// model saw). The holdout fraction applies to the whole set, taken from
// the latest samples.
func SplitLeakageSafe(samples []EvalSample, holdoutFrac float64) (train, holdout []EvalSample) {
	if len(samples) == 0 {
		return nil, nil
	}
	byTime := append([]EvalSample(nil), samples...)
	sort.SliceStable(byTime, func(a, b int) bool { return byTime[a].At < byTime[b].At })

	n := len(byTime)
	splitIdx := n - int(float64(n)*holdoutFrac)
	if splitIdx < 0 {
		splitIdx = 0
	}
	if splitIdx > n {
		splitIdx = n
	}
	// If an identity crosses the cut, advance the cut past its final
	// observation. Scanning the newly included rows too handles chains of
	// overlapping identities. This preserves every sample while keeping
	// the holdout strictly later than the training set.
	last := make(map[string]int, n)
	for i, s := range byTime {
		if s.Identity != "" {
			last[s.Identity] = i
		}
	}
	for i := 0; i < splitIdx; i++ {
		if end, ok := last[byTime[i].Identity]; ok && end >= splitIdx {
			splitIdx = end + 1
		}
		// Equal timestamps have no reliable before/after order. Any
		// rows this adds are scanned by the same loop for identity overlap.
		for splitIdx < n && byTime[splitIdx].At == byTime[splitIdx-1].At {
			splitIdx++
		}
	}
	// Return independent backing arrays: callers may grow either set
	// without silently changing the other one's evaluation rows.
	train = append([]EvalSample(nil), byTime[:splitIdx]...)
	holdout = append([]EvalSample(nil), byTime[splitIdx:]...)
	return train, holdout
}

// PromotionVerdict states the promotion gate's outcome.
const (
	promote = "promote"
	hold    = "hold"
)

// PromotionReport is the plan's promotion test result: on a locked
// later, identity-clean holdout, the model must recall at least as many
// bots as the rules with no more hard-block human false positives, on
// enough holdout traffic to say anything at all. Until it passes, the
// model's output stays shadow-only.
type PromotionReport struct {
	Model    SideReport
	Rules    SideReport
	Decision string `json:"decision"`
	Reason   string
}

// SideReport pairs one side's metrics with its identity, so a report can
// carry both a full-data and a holdout score.
type SideReport struct {
	Name    string
	Metrics *Metrics
	// Holdout is nil for the train side.
	Holdout *Metrics `json:",omitempty"`
}

// RunPromotion evaluates the model against the rules with a
// leakage-safe split. The verdict is hold whenever the evidence is too
// thin to promote — a tiny or one-sided holdout promotes nothing.
func RunPromotion(m *Model, samples []EvalSample) PromotionReport {
	const minHoldout = 50
	const minBots = 5
	const minHumans = 5

	train, holdout := SplitLeakageSafe(samples, 0.2)
	rep := PromotionReport{
		Model: SideReport{Name: "model"},
		Rules: SideReport{Name: "rules"},
	}

	trainMetrics := Evaluate(m, train)
	holdMetrics := Evaluate(m, holdout)
	rep.Model.Metrics = &trainMetrics
	rep.Model.Holdout = &holdMetrics

	rulesTrain := RulesScored(train)
	rulesHold := RulesScored(holdout)
	rep.Rules.Metrics = &rulesTrain
	rep.Rules.Holdout = &rulesHold

	if len(holdout) < minHoldout || holdMetrics.Bots < minBots || holdMetrics.Humans < minHumans {
		rep.Decision = hold
		rep.Reason = fmt.Sprintf("holdout too thin to judge: %d samples, %d bots, %d humans (need >= %d samples, >= %d bots, >= %d humans)",
			len(holdout), holdMetrics.Bots, holdMetrics.Humans, minHoldout, minBots, minHumans)
		return rep
	}

	switch {
	case holdMetrics.Recall < rulesHold.Recall:
		rep.Decision = hold
		rep.Reason = "model recalls fewer bots than the rules on the holdout"
	case holdMetrics.HumanHardBlockFPR > rulesHold.HumanHardBlockFPR:
		rep.Decision = hold
		rep.Reason = "model hard-blocks more humans than the rules on the holdout"
	default:
		rep.Decision = promote
		rep.Reason = fmt.Sprintf("model recall %.3f >= rules %.3f at hard-block human FPR %.3f <= %.3f",
			holdMetrics.Recall, rulesHold.Recall, holdMetrics.HumanHardBlockFPR, rulesHold.HumanHardBlockFPR)
	}
	return rep
}
