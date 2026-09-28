// Command hakaishield-train fits a decision model to labelled traffic and
// writes it out for the proxy to load in shadow mode.
//
// Input is one JSON object per line, in the shape the evidence trail
// already records:
//
//	{"signals":["ua_mismatch","header_anomaly"],"automated":true}
//	{"signals":[],"automated":false}
//
// The label is the part that has to be earned. For production evaluation,
// "automated" must come from independent review, not a forgeable challenge
// solve or a possible browser prefetch. Labelling with the current
// rule score would only teach the model to repeat the guesses it exists
// to improve on, and the resulting model would look excellent against the
// very data that misled it.
//
// This runs offline. Nothing here is in the request path.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/decide"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// labelled is one line of the input file.
type labelled struct {
	Signals   []string `json:"signals"`
	Automated *bool    `json:"automated"`
}

func main() {
	in := flag.String("in", "", "labelled traffic, one JSON object per line (default: stdin, unless -db-url is set)")
	dbURL := flag.String("db-url", os.Getenv("DATABASE_URL"), "read labelled traffic collected by the proxy's -collect-labels instead of a file. Falls back to $DATABASE_URL.")
	tenant := flag.String("tenant", "", "train on one tenant's traffic only; empty trains a model shared across tenants")
	maxSamples := flag.Int("max-samples", 200_000, "most recent samples to read when training from the database")
	allowUnverifiedLabels := flag.Bool("allow-unverified-labels", false, "experimental: train from automatically collected candidate labels; output is for shadow analysis only")
	out := flag.String("out", "", "where to write the trained model (default: stdout)")
	holdout := flag.Float64("holdout", 0.2, "share of the data held back to score the model on traffic it was not trained on")
	blockAt := flag.Float64("block-at", decide.DefaultOptions().BlockAt, "probability at or above which the model would block")
	challengeAt := flag.Float64("challenge-at", decide.DefaultOptions().ChallengeAt, "probability at or above which the model would challenge")
	approvedBy := flag.String("approved-by", "", "name of the person accepting the held-out evaluation; empty saves an unapproved model that stays shadow-only")
	flag.Parse()

	if err := run(runOptions{
		in:                    *in,
		out:                   *out,
		dbURL:                 *dbURL,
		tenant:                *tenant,
		maxSamples:            *maxSamples,
		allowUnverifiedLabels: *allowUnverifiedLabels,
		holdout:               *holdout,
		challengeAt:           *challengeAt,
		blockAt:               *blockAt,
		approvedBy:            *approvedBy,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "hakaishield-train: %v\n", err)
		os.Exit(1)
	}
}

// runOptions is what the command was asked to do. It is a struct rather
// than eight positional arguments because most of them are numbers and
// swapping two would be silent.
type runOptions struct {
	in                    string
	out                   string
	dbURL                 string
	tenant                string
	maxSamples            int
	allowUnverifiedLabels bool
	holdout               float64
	challengeAt           float64
	blockAt               float64
	approvedBy            string
}

func run(opts runOptions) error {
	if opts.in == "" && opts.dbURL != "" && !opts.allowUnverifiedLabels {
		return errors.New("automatically collected labels are unverified; supply curated -in data, or use -allow-unverified-labels for shadow-only experiments")
	}
	if math.IsNaN(opts.holdout) || math.IsInf(opts.holdout, 0) || opts.holdout < 0 || opts.holdout >= 1 {
		return fmt.Errorf("holdout %v must be in [0,1)", opts.holdout)
	}
	if opts.approvedBy != "" && (opts.allowUnverifiedLabels || opts.holdout == 0) {
		return errors.New("approved model requires curated labels and a nonzero holdout; candidate-label or unevaluated models stay unapproved")
	}

	features := signals.FeatureNames()

	var samples []decide.Sample
	var err error
	if opts.in == "" && opts.dbURL != "" {
		samples, err = readFromDatabase(opts)
	} else {
		samples, err = readFromFile(opts.in, features)
	}
	if err != nil {
		return err
	}

	// The holdout is the tail of the input rather than a random slice, so
	// two runs on the same data split it the same way and their scores
	// can be compared. Whoever produces the input is responsible for it
	// not being ordered by label - readFromDatabase shuffles for exactly
	// that reason, since rows come back ordered by time.
	split := len(samples) - int(float64(len(samples))*opts.holdout)
	train, test := samples[:split], samples[split:]
	if opts.approvedBy != "" {
		humans := 0
		for _, sample := range test {
			if !sample.Automated {
				humans++
			}
		}
		if len(test) < 50 || humans < 5 || len(test)-humans < 5 {
			return fmt.Errorf("approved model needs a holdout with at least 50 samples, 5 humans and 5 bots; got %d samples, %d humans, %d bots", len(test), humans, len(test)-humans)
		}
	}

	trainOpts := decide.DefaultOptions()
	trainOpts.ChallengeAt = opts.challengeAt
	trainOpts.BlockAt = opts.blockAt

	model, err := decide.Train(features, train, trainOpts)
	if err != nil {
		return err
	}

	report(os.Stderr, "training set", model.Score(train))
	var heldOut decide.Quality
	if len(test) > 0 {
		// The held-out score is the one worth believing. A model that
		// does well on the data it was fitted to and poorly here has
		// memorised the sample, not learned the traffic.
		heldOut = model.Score(test)
		report(os.Stderr, "held-out set", heldOut)
	} else {
		fmt.Fprintln(os.Stderr, "no held-out data: the training score below is not evidence the model generalises")
	}

	// Stamp the artifact with what produced it: the check-list version,
	// a hash of the labelled rows, the training options and the held-out
	// evaluation. Load refuses an artifact whose stamp contradicts its
	// own feature list, so the origin record cannot silently rot.
	model, err = model.WithProvenance(decide.Provenance{
		FeatureVersion: signals.FeatureVersion(),
		DatasetHash:    datasetHash(samples),
		TrainedWith:    fmt.Sprintf("iterations=%d lr=%v l2=%v challenge_at=%v block_at=%v", trainOpts.Iterations, trainOpts.LearningRate, trainOpts.L2, trainOpts.ChallengeAt, trainOpts.BlockAt),
		EvalSummary:    fmt.Sprintf("held-out samples=%d logloss=%.4f accuracy=%.3f fp=%d fn=%d", heldOut.Samples, heldOut.LogLoss, heldOut.Accuracy, heldOut.FalsePositives, heldOut.FalseNegatives),
		ApprovedBy:     opts.approvedBy,
	})
	if err != nil {
		return err
	}

	if opts.out == "" {
		return model.Save(os.Stdout)
	}

	// Operator-supplied flag; writing where the command line says is the
	// point of the command.
	f, err := os.Create(opts.out) // #nosec G304 -- operator-supplied -out flag
	if err != nil {
		return err
	}
	return saveModel(model, f)
}

// datasetHash fingerprints the labelled rows in order, so two artifacts
// claiming the same data can be told apart. It is a record-keeping hash,
// not a secret.
func datasetHash(samples []decide.Sample) string {
	h := sha256.New()
	for _, s := range samples {
		// A hash.Hash Write never returns an error.
		_, _ = fmt.Fprintf(h, "%d:%d\n", s.Fired, boolInt(s.Automated))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// saveModel writes the model and reports the close error rather than
// deferring it away. A write that only fails when the file is flushed
// would otherwise leave a truncated model on disk while this command
// exited successfully, and the next thing to read it would be the proxy
// at startup.
func saveModel(model *decide.Model, f io.WriteCloser) error {
	if err := model.Save(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// readFromFile reads labelled traffic from a JSONL file, or stdin when
// no path is given.
func readFromFile(inPath string, features []string) ([]decide.Sample, error) {
	src := io.Reader(os.Stdin)
	if inPath != "" {
		// Operator-supplied flag; reading the file named on the command
		// line is what this command is for.
		f, err := os.Open(inPath) // #nosec G304 -- operator-supplied -in flag
		if err != nil {
			return nil, err
		}
		// Nothing is written to it, so a close error says nothing useful.
		defer func() { _ = f.Close() }()
		src = f
	}
	return readSamples(src, features)
}

// readFromDatabase reads labelled traffic the proxy collected.
//
// It filters on this build's feature version, because the fired mask is
// positional: rows captured before a check was added or reordered
// describe different signals, and training across the boundary would be
// training on noise.
func readFromDatabase(opts runOptions) ([]decide.Sample, error) {
	if err := db.Init(opts.dbURL); err != nil {
		return nil, err
	}

	version := signals.FeatureVersion()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stored, err := db.ReadSamples(ctx, opts.tenant, version, opts.maxSamples)
	if err != nil {
		return nil, err
	}
	if len(stored) == 0 {
		return nil, fmt.Errorf("no labelled traffic stored for check list %s%s - is the proxy running with -collect-labels?",
			version, tenantSuffix(opts.tenant))
	}

	out := make([]decide.Sample, len(stored))
	for i, s := range stored {
		out[i] = decide.Sample{Fired: s.Fired, Automated: s.Automated}
	}

	// Rows come back newest first, so the holdout would otherwise be the
	// oldest traffic rather than a sample of it. The shuffle is
	// deliberately deterministic - seeded only by the row count - so two
	// runs on the same rows produce the same split and the same model,
	// which is what makes a drop in detection quality distinguishable
	// from training noise. It is ordering, not secrecy, so a
	// cryptographic source would buy nothing here.
	rng := rand.New(rand.NewSource(int64(len(out)))) // #nosec G404 -- reproducible shuffle, not a secret
	rng.Shuffle(len(out), func(a, b int) { out[a], out[b] = out[b], out[a] })

	fmt.Fprintf(os.Stderr, "read %d labelled requests from the database (check list %s%s)\n",
		len(out), version, tenantSuffix(opts.tenant))
	return out, nil
}

func tenantSuffix(tenant string) string {
	if tenant == "" {
		return ", all tenants"
	}
	return ", tenant " + tenant
}

// readSamples parses the labelled file. A bad line fails the run rather
// than being skipped: a training set silently missing its malformed
// entries is a different training set from the one the operator thinks
// they produced.
func readSamples(r io.Reader, features []string) ([]decide.Sample, error) {
	var samples []decide.Sample

	scanner := bufio.NewScanner(r)
	// Lines hold a handful of short signal names; the default 64KiB
	// buffer is already far more than one needs.
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}

		// Decode rather than Unmarshal, for DisallowUnknownFields: a
		// misspelled field name would otherwise be accepted silently and
		// train the model on a file that is not the one the operator wrote.
		var l labelled
		dec := json.NewDecoder(bytes.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&l); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if l.Signals == nil {
			return nil, fmt.Errorf("line %d: missing signals array", line)
		}
		if l.Automated == nil {
			return nil, fmt.Errorf("line %d: missing automated label", line)
		}
		var extra any
		if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("line %d: extra content after sample: %w", line, err)
		}

		fired, err := decide.Vector(features, l.Signals)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		samples = append(samples, decide.Sample{Fired: fired, Automated: *l.Automated})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, errors.New("no labelled traffic on the input")
	}
	return samples, nil
}

func report(w io.Writer, label string, q decide.Quality) {
	// Writing the report to stderr is best-effort: a failure here must
	// not stop a model that trained correctly from being saved.
	_, _ = fmt.Fprintf(w, "%-12s  samples=%-7d logloss=%.4f  accuracy=%.3f  false-positives=%d  false-negatives=%d\n",
		label, q.Samples, q.LogLoss, q.Accuracy, q.FalsePositives, q.FalseNegatives)
}
