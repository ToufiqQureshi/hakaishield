package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/jackc/pgx/v5"
)

func TestRedactCredentials(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "password redacted, username kept",
			in:   "postgresql://postgres.abc123:supersecretpassword@aws-0-ap-south-1.pooler.supabase.com:5432/postgres",
			want: "postgresql://postgres.abc123:REDACTED@aws-0-ap-south-1.pooler.supabase.com:5432/postgres",
		},
		{
			name: "no credentials, untouched",
			in:   "postgresql://localhost:5432/hakaishield",
			want: "postgresql://localhost:5432/hakaishield",
		},
		{
			name: "username only, no password added",
			in:   "postgresql://postgres@localhost:5432/hakaishield",
			want: "postgresql://postgres@localhost:5432/hakaishield",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redactCredentials(c.in)
			if got != c.want {
				t.Errorf("redactCredentials(%q) = %q, want %q", c.in, got, c.want)
			}
			if got == c.in && c.name == "password redacted, username kept" {
				t.Error("password appeared unredacted in output")
			}
		})
	}
}

func TestDefaultTenantOwnerRequiresMatchingLiveRoute(t *testing.T) {
	owner, err := matchingDefaultOwner("client.example", "https://origin.example", "active", "user-1", "client.example", "https://origin.example")
	if err != nil || owner != "user-1" {
		t.Fatalf("matching owner=%q err=%v", owner, err)
	}
	for _, tc := range []struct{ host, target, status string }{
		{"other.example", "https://origin.example", "active"},
		{"client.example", "https://other.example", "active"},
		{"client.example", "https://origin.example", "pending"},
	} {
		if owner, err := matchingDefaultOwner(tc.host, tc.target, tc.status, "user-1", "client.example", "https://origin.example"); err == nil || owner != "" {
			t.Fatalf("mismatched row %+v yielded owner=%q err=%v", tc, owner, err)
		}
	}
}

func TestLookupDefaultOwnerUsesConfiguredHost(t *testing.T) {
	const (
		host   = "shield.example.com"
		target = "https://origin.example.com"
	)
	var lookedUpHost string
	loader := func(_ context.Context, gotHost string) (id, rowTarget, mode, evidenceToken, status, ownerUserID string, err error) {
		lookedUpHost = gotHost
		return "7a5af8bf-08aa-4cba-b237-abf0a1d0aabc", target, "shadow", "", tenant.StatusActive, "user-1", nil
	}

	owner, err := lookupDefaultOwner(context.Background(), host, target, loader)
	if err != nil {
		t.Fatalf("lookupDefaultOwner: %v", err)
	}
	if lookedUpHost != host {
		t.Fatalf("loader host = %q, want configured host %q", lookedUpHost, host)
	}
	if owner != "user-1" {
		t.Fatalf("owner = %q, want user-1", owner)
	}
}

func TestLookupDefaultOwnerAllowsMissingPilotRow(t *testing.T) {
	loader := func(context.Context, string) (id, target, mode, evidenceToken, status, ownerUserID string, err error) {
		return "", "", "", "", "", "", pgx.ErrNoRows
	}

	owner, err := lookupDefaultOwner(context.Background(), "shield.example.com", "https://origin.example.com", loader)
	if err != nil || owner != "" {
		t.Fatalf("missing row yielded owner=%q err=%v, want empty owner and no error", owner, err)
	}
}

func TestRedactCredentials_NeverLeaksPasswordSubstring(t *testing.T) {
	// The specific regression this guards: a raw password string must
	// never appear in the redacted output, regardless of URL shape.
	password := "i4A1jmYKSFcZwEOe-not-a-real-secret"
	in := "postgresql://postgres.xyz:" + password + "@aws-0-ap-south-1.pooler.supabase.com:5432/postgres"
	got := redactCredentials(in)
	if strings.Contains(got, password) {
		t.Fatalf("redactCredentials leaked the password into: %q", got)
	}
}

func TestRedactCredentials_UnparseableInputDoesNotPanic(t *testing.T) {
	got := redactCredentials("://not a valid url at all")
	if got == "" {
		t.Error("expected a non-empty fallback message for unparseable input")
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "DATABASE_URL=postgresql://example/db\n# a comment\n\nSUPABASE_URL=\"https://example.supabase.co\"\nQUOTED_SINGLE='value'\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test .env: %v", err)
	}

	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("SUPABASE_URL")
	os.Unsetenv("QUOTED_SINGLE")
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SUPABASE_URL")
		os.Unsetenv("QUOTED_SINGLE")
	})

	loadDotEnv(envPath)

	if got := os.Getenv("DATABASE_URL"); got != "postgresql://example/db" {
		t.Errorf("DATABASE_URL = %q, want %q", got, "postgresql://example/db")
	}
	if got := os.Getenv("SUPABASE_URL"); got != "https://example.supabase.co" {
		t.Errorf("SUPABASE_URL = %q, want %q (quotes should be stripped)", got, "https://example.supabase.co")
	}
	if got := os.Getenv("QUOTED_SINGLE"); got != "value" {
		t.Errorf("QUOTED_SINGLE = %q, want %q", got, "value")
	}
}

func TestLoadDotEnv_DoesNotOverrideExistingEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("SOME_VAR=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("writing test .env: %v", err)
	}

	os.Setenv("SOME_VAR", "from-real-shell-env")
	t.Cleanup(func() { os.Unsetenv("SOME_VAR") })

	loadDotEnv(envPath)

	if got := os.Getenv("SOME_VAR"); got != "from-real-shell-env" {
		t.Errorf("SOME_VAR = %q, want the pre-existing shell value to win over .env", got)
	}
}

func TestLoadDotEnv_MissingFileIsNotAnError(t *testing.T) {
	// Must not panic or os.Exit; a missing .env is the normal case in
	// production where config comes from real env vars.
	loadDotEnv(filepath.Join(t.TempDir(), "does-not-exist.env"))
}

func TestInternalRoutesReachGuardWhenChallengeRoutesAreMountedExactly(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatalf("origin proxy: %v", err)
	}
	store := tenant.NewStore()
	if err := store.Add("default", tenant.TenantConfig{Target: origin.URL, Mode: config.ModeEnforce, Policy: config.PolicyBalanced}, []string{"example.com"}, proxy); err != nil {
		t.Fatalf("add tenant: %v", err)
	}
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	guard := core.NewGuard(store, c)

	mux := http.NewServeMux()
	mountChallengeRoutes(mux, c)
	mux.Handle("/", guard)

	// Puzzle issuance happens inside Guard after a tenant and risk decision.
	// The standalone GET endpoint must not mint signed challenges for arbitrary
	// Host headers or amplify unauthenticated requests into large HTML pages.
	issueReq := httptest.NewRequest(http.MethodGet, "http://unknown.example/__hakaishield/challenge", nil)
	issueRec := httptest.NewRecorder()
	mux.ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusNotFound {
		t.Fatalf("public challenge minting endpoint returned %d, want 404", issueRec.Code)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "http://example.com/__hakaishield/healthz", nil)
	healthRec := httptest.NewRecorder()
	mux.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("healthz should reach Guard through production mux shape, got %d", healthRec.Code)
	}

	trapReq := httptest.NewRequest(http.MethodGet, "http://example.com"+signals.HoneypotPath, nil)
	trapReq.RemoteAddr = "203.0.113.9:443"
	trapReq = trapReq.WithContext(core.WithJA4(trapReq.Context(), "t12d190800_4464c1bd5eb7_b3394627b738"))
	trapRec := httptest.NewRecorder()
	mux.ServeHTTP(trapRec, trapReq)
	if trapRec.Code != http.StatusNotFound {
		t.Fatalf("honeypot trap should reach Guard and return 404, got %d", trapRec.Code)
	}
	if !signals.HoneypotTripped("default", "203.0.113.9", "t12d190800_4464c1bd5eb7_b3394627b738") {
		t.Fatal("honeypot trip was not recorded through production mux shape")
	}
}
