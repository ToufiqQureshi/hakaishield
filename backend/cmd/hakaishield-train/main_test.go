package main

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/decide"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// labelledTraffic writes a training file using real check names, so this
// test breaks if the trainer and the live check list ever drift apart.
func labelledTraffic(t *testing.T, humans, bots int) string {
	t.Helper()

	names := signals.FeatureNames()
	if len(names) < 2 {
		t.Fatalf("this build runs only %d checks; the trainer needs at least 2", len(names))
	}

	var b strings.Builder
	for i := 0; i < humans; i++ {
		b.WriteString(`{"signals":[],"automated":false}` + "\n")
	}
	for i := 0; i < bots; i++ {
		b.WriteString(`{"signals":["` + names[0] + `","` + names[1] + `"],"automated":true}` + "\n")
	}

	path := filepath.Join(t.TempDir(), "labelled.jsonl")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return path
}

// The whole point of the command: labelled traffic in, a model the proxy
// can actually load out.
func TestTrainedModelLoadsBackIntoTheProxy(t *testing.T) {
	in := labelledTraffic(t, 300, 300)
	out := filepath.Join(t.TempDir(), "model.json")

	if err := run(runOptions{in: in, out: out, holdout: 0.2, challengeAt: 0.5, blockAt: 0.9}); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("the trainer wrote no model: %v", err)
	}
	defer f.Close()

	m, err := decide.Load(f, signals.FeatureNames())
	if err != nil {
		t.Fatalf("the proxy cannot load the model this trainer wrote: %v", err)
	}

	// It has to have learned the separation it was shown, or the file is
	// a model in shape only.
	names := signals.FeatureNames()
	botLike, err := decide.Vector(names, []string{names[0], names[1]})
	if err != nil {
		t.Fatalf("Vector() error: %v", err)
	}
	if got := m.Predict(botLike); got.Decision == signals.DecisionAllow {
		t.Errorf("the trained model allows the pattern it was shown as automated (p=%.4f)", got.Probability)
	}
	if got := m.Predict(0); got.Decision != signals.DecisionAllow {
		t.Errorf("the trained model does not allow the pattern it was shown as human: %v (p=%.4f)", got.Decision, got.Probability)
	}
}

// A malformed or foreign line means the file is not what the operator
// thinks it is. Skipping it would train on a quietly different set.
func TestRunRefusesUnusableInput(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"a truncated line", `{"signals":[],"automated":` + "\n", "line 1"},
		{"a misspelled field", `{"signal":[],"automated":false}` + "\n", "unknown field"},
		{"a missing label", `{"signals":[]}` + "\n", "missing automated"},
		{"a missing feature list", `{"automated":false}` + "\n", "missing signals"},
		{"two objects on one line", `{"signals":[],"automated":false}{"signals":[],"automated":true}` + "\n", "line 1"},
		{"a check from another build", `{"signals":["not_a_real_check"],"automated":true}` + "\n", "not_a_real_check"},
		{"an empty file", "", "no labelled traffic"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := filepath.Join(t.TempDir(), "labelled.jsonl")
			if err := os.WriteFile(in, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			out := filepath.Join(t.TempDir(), "model.json")
			err := run(runOptions{in: in, out: out, holdout: 0.2, challengeAt: 0.5, blockAt: 0.9})
			if err == nil {
				t.Fatalf("run(%s) returned no error, want one mentioning %q", tc.name, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("run(%s) error = %q, want it to mention %q", tc.name, err, tc.want)
			}
		})
	}
}

func TestRunRefusesAutomaticallyCollectedLabelsByDefault(t *testing.T) {
	err := run(runOptions{dbURL: "postgres://unused", holdout: 0.2, challengeAt: 0.5, blockAt: 0.9})
	if err == nil || !strings.Contains(err.Error(), "unverified") {
		t.Fatalf("run() error = %v, want an unverified-label gate before any database read", err)
	}
}

func TestRunRefusesApprovalOfUnverifiedOrUnevaluatedLabels(t *testing.T) {
	in := labelledTraffic(t, 100, 100)
	for _, tc := range []runOptions{
		{in: in, out: filepath.Join(t.TempDir(), "unverified.json"), holdout: 0.2, allowUnverifiedLabels: true, approvedBy: "owner", challengeAt: 0.5, blockAt: 0.9},
		{in: in, out: filepath.Join(t.TempDir(), "no-holdout.json"), holdout: 0, approvedBy: "owner", challengeAt: 0.5, blockAt: 0.9},
	} {
		if err := run(tc); err == nil {
			t.Fatalf("run(%+v) stamped an approved model without verified holdout evidence", tc)
		}
	}
}

func TestRunRefusesApprovalOfOneClassHoldout(t *testing.T) {
	// Input ordered by label puts only bots in the final holdout. A good
	// training score cannot measure human harm on that slice.
	in := labelledTraffic(t, 150, 150)
	err := run(runOptions{in: in, out: filepath.Join(t.TempDir(), "model.json"), holdout: 0.2, approvedBy: "owner", challengeAt: 0.5, blockAt: 0.9})
	if err == nil {
		t.Fatal("approved model accepted a holdout with no humans")
	}
}

// One-class data reaches the trainer as a perfectly accurate, perfectly
// useless model. It has to stop here, not in production.
func TestRunRefusesOneClassTraffic(t *testing.T) {
	in := labelledTraffic(t, 0, 200)
	if err := run(runOptions{in: in, out: filepath.Join(t.TempDir(), "model.json"), holdout: 0.2, challengeAt: 0.5, blockAt: 0.9}); err == nil {
		t.Fatal("run() accepted traffic labelled entirely automated")
	}
}

func TestRunRefusesAnImpossibleHoldout(t *testing.T) {
	in := labelledTraffic(t, 100, 100)
	for _, holdout := range []float64{-0.1, 1, 1.5, math.NaN(), math.Inf(1)} {
		if err := run(runOptions{in: in, out: filepath.Join(t.TempDir(), "model.json"), holdout: holdout, challengeAt: 0.5, blockAt: 0.9}); err == nil {
			t.Errorf("run(holdout=%v) returned no error", holdout)
		}
	}
}

// readSamples skips blank lines, which a hand-edited file will have,
// without treating them as an unlabelled request.
func TestReadSamplesSkipsBlankLines(t *testing.T) {
	names := signals.FeatureNames()
	content := "\n" + `{"signals":[],"automated":false}` + "\n\n" + `{"signals":["` + names[0] + `"],"automated":true}` + "\n\n"

	got, err := readSamples(bytes.NewReader([]byte(content)), names)
	if err != nil {
		t.Fatalf("readSamples() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("readSamples() returned %d samples, want 2: %+v", len(got), got)
	}
	if got[0].Automated || !got[1].Automated {
		t.Errorf("labels came back wrong: %+v", got)
	}
}

// A model that trained fine but could not be written is a failure, not a
// success with nothing on disk.
func TestRunReportsAnUnwritableOutput(t *testing.T) {
	in := labelledTraffic(t, 200, 200)

	// A directory cannot be opened for writing, so os.Create fails here.
	dir := t.TempDir()
	if err := run(runOptions{in: in, out: dir, holdout: 0.2, challengeAt: 0.5, blockAt: 0.9}); err == nil {
		t.Fatal("run() reported success writing the model to a directory")
	}
}

// The success path must leave a complete model, not a partial one.
func TestRunWritesACompleteModel(t *testing.T) {
	in := labelledTraffic(t, 300, 300)
	out := filepath.Join(t.TempDir(), "model.json")

	if err := run(runOptions{in: in, out: out, holdout: 0.2, challengeAt: 0.5, blockAt: 0.9}); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if _, err := decide.Load(bytes.NewReader(written), signals.FeatureNames()); err != nil {
		t.Fatalf("the file left on disk is not a loadable model: %v\n%s", err, written)
	}
}

// failOnClose accepts every write and fails only when closed - the shape
// of a buffered write that turns out to be short once it is flushed.
type failOnClose struct{ err error }

func (f failOnClose) Write(p []byte) (int, error) { return len(p), nil }
func (f failOnClose) Close() error                { return f.err }

// A model whose bytes never reached the disk is not saved, however well
// it trained. Deferring the close would report success here.
func TestSaveModelReportsACloseFailure(t *testing.T) {
	m, err := decide.Train(signals.FeatureNames(), []decide.Sample{
		{Fired: 0, Automated: false},
		{Fired: 1, Automated: true},
	}, decide.DefaultOptions())
	if err != nil {
		t.Fatalf("Train() error: %v", err)
	}

	want := errors.New("disk full on flush")
	if got := saveModel(m, failOnClose{err: want}); !errors.Is(got, want) {
		t.Fatalf("saveModel() = %v, want the close error %v", got, want)
	}

	if got := saveModel(m, failOnClose{}); got != nil {
		t.Errorf("saveModel() = %v on a clean close, want nil", got)
	}
}
