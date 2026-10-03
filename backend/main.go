// Command hakaishield runs the hakaishield reverse proxy.
package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/api"
	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/decide"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/labels"
	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/ToufiqQureshi/hakaishield/pkg/policyprovider"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenantpolicy"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// loadDotEnv reads KEY=VALUE lines from a local .env file (if present)
// into the process environment, without overwriting a variable the
// shell already set — so a real deployment's env vars always win over
// a stray .env left in the working directory. There's no standard
// library .env parser and pulling in a dependency for ~15 lines of
// "split on the first '=', trim quotes" isn't worth it.
func loadDotEnv(path string) {
	// The path is this program's own, not anything a request supplies.
	f, err := os.Open(path) // #nosec G304 -- fixed .env path chosen by the operator
	if err != nil {
		return // no .env file; nothing to load, not an error
	}
	// Read-only, so a close error says nothing useful.
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, alreadySet := os.LookupEnv(key); !alreadySet {
			// Setenv only fails on a key the OS rejects, such as one
			// containing "=". Skipping that line is the right outcome
			// and there is nowhere useful to report it this early.
			_ = os.Setenv(key, value)
		}
	}
	// A read error (such as a line over the 64 KiB scanner limit) stops
	// the loop early. Starting with half the secrets loaded is worse than
	// not starting.
	if err := scanner.Err(); err != nil {
		log.Fatalf("hakaishield: reading %s: %v", path, err)
	}
}

// redactCredentials removes URL userinfo passwords before logging database
// or Redis endpoints. Deployment logs may leave the host, so credentials
// must never be printed there.
func redactCredentials(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "(unparseable connection string, not logging it)"
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "REDACTED")
		}
	}
	return u.String()
}

// matchingDefaultOwner binds policy ownership only when the dashboard row
// describes the same live host and origin as this pilot process.
func matchingDefaultOwner(rowHost, rowTarget, rowStatus, owner, host, target string) (string, error) {
	if rowHost != host || rowTarget != target || rowStatus != tenant.StatusActive {
		return "", errors.New("default tenant database row does not match the active pilot host/origin")
	}
	return owner, nil
}

// lookupDefaultOwner finds the managed pilot row by its configured hostname.
// Database tenant IDs may be UUIDs, so a synthetic "default" ID is invalid.
func lookupDefaultOwner(ctx context.Context, host, target string, loader tenant.TenantLoader) (string, error) {
	_, rowTarget, _, _, rowStatus, owner, err := loader(ctx, host)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return matchingDefaultOwner(host, rowTarget, rowStatus, owner, host, target)
}

func main() {
	loadDotEnv(".env")

	addr := flag.String("addr", ":8080", "address to listen on")
	target := flag.String("target", "", "origin server to protect, e.g. https://example.com")
	originHostFromTarget := flag.Bool("origin-host-from-target", false, "send the target URL host to the origin when its virtual host differs from the protected hostname")
	host := flag.String("host", "", "public hostname for the default tenant; empty permits any Host for local development")
	certFile := flag.String("tls-cert", "", "TLS certificate file; enables TLS + JA4 fingerprinting")
	keyFile := flag.String("tls-key", "", "TLS private key file, required with -tls-cert")
	challengeSecret := flag.String("challenge-secret", "", "Shared secret for stateless JS challenges. If empty, a random one is generated.")
	evidenceToken := flag.String("evidence-token", os.Getenv("EVIDENCE_TOKEN"), "bearer token for the per-request evidence endpoint; unset leaves the endpoint off")
	observabilityToken := flag.String("observability-token", os.Getenv("HAKAISHIELD_OBSERVABILITY_TOKEN"), "bearer token for aggregate operational counters; unset leaves the endpoint off")
	modeFlag := flag.String("mode", "enforce", `"enforce" acts on scores; "shadow" only records what it would have done`)
	themeFlag := flag.String("theme", "ghost", `challenge page theme: "ghost", "branded", or "default"`)
	policyFlag := flag.String("policy", "balanced", `policy strategy: "balanced" (allow clean score 0, challenge suspicious) or "strict" (mandatory challenge)`)
	deceptionFlag := flag.Bool("deception", false, "enable deception mode (forwards high-confidence bots to origin with X-HakaiShield-Decision: deceive instead of 403)")
	trustedProxyCIDRs := flag.String("trusted-proxy-cidrs", "", "comma-separated proxy CIDRs allowed to supply X-Forwarded-For; leave empty to trust only direct peers")
	redisURL := flag.String("redis-url", "redis://localhost:6379", "Redis connection URL for distributed rate limiting")
	dbURL := flag.String("db-url", os.Getenv("DATABASE_URL"), "PostgreSQL URL for the Supabase project's database (Project Settings > Database in the Supabase dashboard). Falls back to $DATABASE_URL (including from a local .env file) if unset.")
	collectLabels := flag.Bool("collect-labels", false, "collect candidate observations from solved challenges and honeypot hits. Requires -db-url. Off by default; see docs/ARCHITECTURE.md.")
	sampleRetentionDays := flag.Int("sample-retention-days", 30, "delete training samples older than this many days, 1-365; background cleanup requires -db-url")
	durableEvidence := flag.Bool("durable-evidence", false, "persist the evidence trail to -db-url so it survives a restart. Off by default: it adds a database write per decision (batched off the request path).")
	evidenceRetentionDays := flag.Int("evidence-retention-days", 30, "delete durable evidence records older than this many days, 1-365; used only with -durable-evidence")
	asnDBPath := flag.String("asn-db", "", "ip2asn TSV (optionally .gz) from iptoasn.com; enables the evidence-only datacenter_ip signal. Unset leaves it off.")
	modelPath := flag.String("model", "", "trained decision model (pkg/decide) to score alongside the rules in shadow; it never affects a decision. Unset leaves it off.")
	supabaseURL := flag.String("supabase-url", os.Getenv("SUPABASE_URL"), "Supabase project URL (e.g. https://xxxx.supabase.co); used to verify dashboard session JWTs against the project's published JWKS. Required, with -db-url, to enable the domains/rules/settings API. Falls back to $SUPABASE_URL (including from a local .env file) if unset.")
	flag.Parse()
	if *sampleRetentionDays < 1 || *sampleRetentionDays > 365 {
		log.Fatal("hakaishield: -sample-retention-days must be between 1 and 365")
	}
	if *evidenceRetentionDays < 1 || *evidenceRetentionDays > 365 {
		log.Fatal("hakaishield: -evidence-retention-days must be between 1 and 365")
	}

	mode, err := config.ParseMode(*modeFlag)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	policy, err := config.ParsePolicy(*policyFlag)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	// SENTRY_DSN is an env var, not a flag: flags show up in `ps aux`
	// output on shared hosts, which a DSN (while not a secret that
	// grants access to customer data) still has no reason to leak into.
	if err := observability.Init(os.Getenv("SENTRY_DSN")); err != nil {
		log.Printf("hakaishield: warning: sentry init failed: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	if *target == "" {
		log.Fatal("hakaishield: -target is required")
	}

	secretStr := *challengeSecret
	if secretStr == "" {
		secretStr = os.Getenv("HAKAISHIELD_CHALLENGE_SECRET")
	}
	secret := []byte(secretStr)
	if len(secret) == 0 {
		// Fallback to random if not provided, sufficient for single-node.
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatalf("hakaishield: generating random secret: %v", err)
		}
	}

	challengeHandler, err := challenge.NewChallenge(secret, *themeFlag)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	// Initialize Redis for global rate limiting
	opt, err := redis.ParseURL(*redisURL)
	if err != nil {
		log.Fatalf("hakaishield: invalid redis url: %v", err)
	}
	// Request-path Redis operations have their own bounded context. Disable
	// command retries and permit one dial attempt: during an outage, retries
	// add latency and multiply load before the signals package can open its
	// circuit. go-redis uses -1 to disable command retries and treats zero
	// dial attempts as its default, so the dial count must be explicit.
	opt.MaxRetries = -1
	opt.DialerRetries = 1
	rdb := redis.NewClient(opt)
	ctxRdb, cancelRdb := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRdb()
	if err := rdb.Ping(ctxRdb).Err(); err != nil {
		log.Printf("hakaishield: warning: could not connect to redis at %s: %v (falling back to open)", redactCredentials(*redisURL), err)
	} else {
		log.Printf("hakaishield: connected to redis at %s", redactCredentials(*redisURL))
	}
	signals.InitRedis(rdb)
	challengeHandler.SetNonceStore(challenge.NewRedisNonceStore(rdb, ""))

	// Start dynamic JA4 synchronization from Redis
	signals.StartJA4Sync(context.Background(), rdb)
	// And the HTTP/2 tool fingerprint feed, same pattern.
	signals.StartHTTP2ToolSync(context.Background(), rdb)

	if *dbURL != "" {
		if err := db.Init(*dbURL); err != nil {
			log.Fatalf("hakaishield: initializing postgres db: %v", err)
		}
		log.Printf("hakaishield: connected to postgres at %s", redactCredentials(*dbURL))
	}

	store := tenant.NewStore()
	// Durable evidence is opt-in and additive: reads keep coming from the
	// in-memory ring, and Record queues a copy to Postgres without blocking
	// the request. A database is required, so an unset -db-url leaves it off.
	var evidenceWriter *evidence.Writer
	if *durableEvidence {
		if *dbURL == "" {
			log.Fatal("hakaishield: -durable-evidence needs -db-url: there is nowhere durable to write")
		}
		evidenceWriter = evidence.NewWriter(db.EvidenceStore{}, evidence.WriterOptions{})
		store.EvidenceWriter = evidenceWriter
		store.EvidenceSink = db.EvidenceStore{}
	}
	challengeHandler.SetOutcomeRecorder(func(host string, solved bool) {
		// The verify route is unauthenticated; like the shadow recorder,
		// only an already-loaded cached tenant is charged, so a forged
		// Host can never create a database lookup or touch another
		// tenant's numbers.
		if tn := store.GetCachedByHost(host); tn != nil {
			if solved {
				tn.Stats.RecordChallengeSolved()
			} else {
				tn.Stats.RecordChallengeFailed()
			}
		}
	})
	challengeHandler.SetShadowRecorder(func(host string, signals []string) {
		// The verify route is unauthenticated. Only use a tenant already
		// resolved by its original request; never trigger a new DB lookup here.
		if tn := store.GetCachedByHost(host); tn != nil {
			tn.Trail.Record(evidence.Evidence{
				ShadowSignals: signals,
				Decision:      "allow",
				Enforced:      true,
			})
		}
	})
	// Dashboard/database-created tenant origins are customer-controlled input.
	// Keep the default -target dev path flexible, but require lazy-loaded SaaS
	// origins to be public and rechecked on dial to close the SSRF path.
	store.ProxyFactory = core.NewPublicOriginProxy

	var originProxy *httputil.ReverseProxy
	if *originHostFromTarget {
		originProxy, err = core.NewOriginProxyWithTargetHost(*target)
	} else {
		originProxy, err = core.NewOriginProxy(*target)
	}
	if err != nil {
		log.Fatalf("hakaishield: creating origin proxy: %v", err)
	}

	defaultHosts := []string{"*"}
	if *host != "" {
		defaultHosts = []string{*host}
	} else {
		log.Print("hakaishield: warning: -host unset; default origin accepts any hostname not claimed by a database tenant")
	}
	defaultOwner := ""
	if *dbURL != "" && *host != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defaultOwner, err = lookupDefaultOwner(ctx, *host, *target, db.GetTenant)
		cancel()
		if err != nil {
			log.Fatalf("hakaishield: reading default tenant owner: %v", err)
		}
	}
	err = store.Add("default", tenant.TenantConfig{
		Target:        *target,
		Mode:          mode,
		Policy:        policy,
		EvidenceToken: *evidenceToken,
		Deception:     *deceptionFlag,
		OwnerUserID:   defaultOwner,
	}, defaultHosts, originProxy)

	if err != nil {
		log.Fatalf("hakaishield: provisioning default tenant: %v", err)
	}

	clientIPResolver, err := core.NewClientIPResolver(strings.Split(*trustedProxyCIDRs, ","))
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}
	guard := core.NewGuardWithClientIPResolver(store, challengeHandler, clientIPResolver)

	// Labelled traffic accumulates in the background so a model can be
	// trained later. Collecting decides nothing and changes no response;
	// it needs a database because the evidence trail is a small in-memory
	// ring buffer, not a place to accumulate anything.
	if *collectLabels {
		if *dbURL == "" {
			log.Fatal("hakaishield: -collect-labels needs -db-url: there is nowhere to put the samples")
		}
		recorder := labels.NewRecorder(db.SampleStore{})
		guard.WithLabelRecorder(recorder)
		defer recorder.Close()
		log.Printf("hakaishield: collecting candidate labels (check list %s); it records traffic and decides nothing", signals.FeatureVersion())
	}

	// A model scores alongside the rules and is recorded, never acted on.
	// Same rule for the ASN table: a path that does not load is fatal, so a
	// typo cannot silently switch the datacenter signal off.
	if *asnDBPath != "" {
		n, err := loadASNDB(*asnDBPath)
		if err != nil {
			log.Fatalf("hakaishield: -asn-db: %v", err)
		}
		log.Printf("hakaishield: ASN table loaded from %s: %d hosting ranges (datacenter_ip is evidence only)", *asnDBPath, n)
	}

	// A bad model file is fatal rather than ignored: starting anyway would
	// look like the operator's model was running when it was not.
	if *modelPath != "" {
		model, err := loadShadowModel(*modelPath)
		if err != nil {
			log.Fatalf("hakaishield: -model: %v", err)
		}
		guard.WithShadowModel(model)
		// Approval state is visible at startup so an operator can tell a
		// shadow-analysis artifact from one a person signed off. Either
		// way it decides nothing here; enforcement is a separate gate.
		if model.Approved() {
			log.Printf("hakaishield: shadow model loaded from %s (trained on %d requests, approved by %s); it records opinions and decides nothing", *modelPath, model.TrainedOn(), model.Provenance().ApprovedBy)
		} else {
			log.Printf("hakaishield: shadow model loaded from %s (trained on %d requests, UNAPPROVED - shadow analysis only); it records opinions and decides nothing", *modelPath, model.TrainedOn())
		}
	}

	// The proxy loads versioned tenant policy asynchronously. Legacy account
	// rules remain shadow-only; activated tenant revisions may enforce.
	// This only needs Postgres, not dashboard Supabase auth.
	var provider *policyprovider.Provider
	if *dbURL != "" {
		provider = policyprovider.New(store, rules.NewStore(db.DB), settings.NewStore(db.DB)).WithTenantPolicies(tenantpolicy.NewStore(db.DB))
		guard.WithPolicyProvider(provider.ForTenant)
		log.Print("hakaishield: tenant policy provider enabled")
	}

	mux := http.NewServeMux()
	mountChallengeRoutes(mux, challengeHandler)
	mux.Handle("/", guard)

	if *evidenceToken != "" {
		mux.Handle("/api/v1/dashboard/evidence", api.DashboardEvidenceHandler(store))
	} else {
		log.Print("hakaishield: -evidence-token not set, evidence endpoint disabled")
	}
	if *observabilityToken != "" {
		mux.Handle("/__hakaishield/observability", observability.CountersHandler(*observabilityToken))
	} else {
		log.Print("hakaishield: -observability-token not set, aggregate counters endpoint disabled")
	}

	// The domains/rules/settings dashboard API needs both a database
	// (the only durable store any of it has) and a Supabase project URL
	// (to verify session JWTs Supabase Auth issued, against that
	// project's published JWKS — see pkg/auth). Requiring both
	// explicitly means a misconfigured deployment fails loudly at
	// startup instead of silently accepting no sessions at all.
	if *dbURL != "" && *supabaseURL != "" {
		verifier, err := auth.NewVerifier(*supabaseURL)
		if err != nil {
			log.Fatalf("hakaishield: %v", err)
		}
		rulesStore := rules.NewStore(db.DB)
		settingsStore := settings.NewStore(db.DB)

		// No method prefix on any of these patterns: net/http's
		// ServeMux would reject a browser's CORS preflight OPTIONS
		// request at the routing layer before it ever reached a
		// handler's own OPTIONS short-circuit, breaking every one of
		// these from a browser (found via an end-to-end Playwright
		// run — see docs/PROGRESS.md). Every handler below already
		// checks r.Method itself (directly, or via RequireAuth), so
		// the mux doesn't need to gate on method too.
		//
		// Signup/signin/session-management are no longer this
		// backend's job — the frontend talks to Supabase Auth
		// directly (see dashboard/src/lib/supabaseClient.ts). This
		// backend only verifies the JWT Supabase already issued.
		mux.HandleFunc("/api/v1/domains", api.DomainsHandler(verifier))
		mux.HandleFunc("/api/v1/domains/{id}/verify", api.DomainVerifyHandler(verifier))
		mux.Handle("/api/v1/dashboard/stats", api.DashboardStatsHandler(store, verifier))
		mux.HandleFunc("/api/v1/rules", api.RulesListHandler(rulesStore, verifier))
		mux.HandleFunc("/api/v1/rules/custom", api.CreateRuleHandler(rulesStore, verifier))
		mux.HandleFunc("/api/v1/rules/{id}/toggle", api.ToggleRuleHandler(rulesStore, verifier))
		policyStore := tenantpolicy.NewStore(db.DB)
		mux.HandleFunc("/api/v1/domains/{id}/policy", api.TenantPolicyHandler(policyStore, provider, verifier))
		mux.HandleFunc("/api/v1/domains/{id}/policy/history", api.TenantPolicyHistoryHandler(policyStore, verifier))
		mux.HandleFunc("/api/v1/domains/{id}/policy/history/{version}", api.TenantPolicyVersionHandler(policyStore, verifier))
		mux.HandleFunc("/api/v1/domains/{id}/policy/rollback", api.TenantPolicyRollbackHandler(policyStore, provider, verifier))
		mux.HandleFunc("/api/v1/domains/{id}/policy/preview", api.TenantPolicyPreviewHandler(policyStore, verifier))
		mux.HandleFunc("/api/v1/domains/{id}/policy/shadow", api.TenantPolicyShadowHandler(policyStore, store, verifier))
		mux.HandleFunc("/api/v1/domains/{id}/policy/activate", api.TenantPolicyActivateHandler(policyStore, provider, store, verifier))
		mux.HandleFunc("/api/v1/settings/protection", api.ProtectionSettingsHandler(settingsStore, verifier))
		mux.HandleFunc("/api/v1/dashboard/top-offenders", api.TopOffendersHandler(store, verifier))
		mux.HandleFunc("/api/v1/dashboard/evidence-logs", api.EvidenceLogsHandler(store, verifier))
		log.Print("hakaishield: domains/rules/settings API enabled (Supabase-authenticated)")
	} else {
		log.Print("hakaishield: -db-url and/or -supabase-url not set, domains/rules/settings API disabled")
	}

	srv := &http.Server{
		Handler:           observability.Middleware(mux),
		ConnContext:       core.ConnContext,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	// Serve negotiated h2 connections with the maintained x/net/http2
	// server after their greeting has been fingerprinted. HTTP/1.1
	// serving is untouched.
	if err := core.WireHTTP2(srv); err != nil {
		log.Fatalf("hakaishield: configuring HTTP/2: %v", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("hakaishield: loading TLS cert/key: %v", err)
		}
		// TLS 1.0/1.1 are deprecated and, for a product whose own
		// detection logic reads TLS version to spot automation
		// (UAMismatch in pkg/signals), accepting them here would also
		// undermine that signal for real visitors on old clients.
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		ln = core.NewCaptureListener(ln, tlsConfig)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *dbURL != "" {
		go runSampleRetention(ctx, *sampleRetentionDays)
	}
	if evidenceWriter != nil {
		go evidenceWriter.Run(ctx)
		hydrateCtx, cancelHydrate := context.WithTimeout(ctx, 5*time.Second)
		store.HydrateEvidence(hydrateCtx)
		cancelHydrate()
		go runEvidenceRetention(ctx, *evidenceRetentionDays)
		log.Printf("hakaishield: durable evidence enabled (batched write-behind; the in-memory ring still serves reads)")
	}

	go func() {
		log.Printf("hakaishield: listening on %s, protecting %s", *addr, *target)
		if mode == config.ModeShadow {
			log.Print("hakaishield: SHADOW MODE - scoring and recording only, NOTHING will be blocked or challenged")
		}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("hakaishield: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("hakaishield: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("hakaishield: shutdown error: %v", err)
	}
}

// mountChallengeRoutes exposes verification without a public puzzle issuer.
// Guard issues puzzles only after resolving the customer and scoring traffic.
func mountChallengeRoutes(mux *http.ServeMux, c *challenge.Challenge) {
	mux.Handle("/__hakaishield/challenge", http.NotFoundHandler())
	mux.Handle("/__hakaishield/verify", c.Handler())
}

// loadShadowModel reads a trained model and checks it against the checks
// this binary actually runs.
func loadShadowModel(path string) (*decide.Model, error) {
	// The path is an operator-supplied flag; pointing the process at a
	// file is the whole feature, and nothing a visitor sends reaches it.
	f, err := os.Open(path) // #nosec G304 -- operator-supplied -model flag
	if err != nil {
		return nil, err
	}
	// Nothing was written, so a close error says nothing useful.
	defer func() { _ = f.Close() }()
	return decide.Load(f, signals.FeatureNames())
}

// loadASNDB opens the operator's ip2asn file, decompressing it when the
// name ends in .gz, as iptoasn.com publishes it.
func loadASNDB(path string) (int, error) {
	f, err := os.Open(path) // #nosec G304 -- operator-chosen startup flag
	if err != nil {
		return 0, err
	}
	// Read-only, so a close error says nothing useful.
	defer func() { _ = f.Close() }()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	return signals.LoadHostingASNDB(r)
}
