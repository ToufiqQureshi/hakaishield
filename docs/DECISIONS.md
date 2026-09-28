# DECISIONS.md — Why We Chose What We Chose

A running log of real decisions made on this project and the
reasoning behind them, so a new session/agent doesn't have to guess
or re-derive context that already exists. Newest entries at the top.

When you make a real decision (tech choice, scope cut, priority
change, rejected alternative), add an entry here **before** ending
your session. See `CLAUDE.md` Section 0 / the mandatory update rule.

---

## Cloudflare Pages for the dashboard — 2026-09-25

**Decision.** Host the static React dashboard on Cloudflare Pages in its own
`hakaishield-dashboard` Direct Upload project. Pin Node 24 and validate public
build-time Supabase/API values before a full-dashboard upload. Since the backend
API is not live, publish only public marketing routes in the first Pages build;
the authenticated dashboard is a separate later build. Direct Upload needs an
explicit upload for each release; GitHub pushes alone do not deploy it.

**Why.** Pages is suitable for static dashboard files. It is kept separate
from the inline customer proxy, which must terminate visitor TLS itself to
retain the visitor's ClientHello/JA4. Missing Vite values otherwise produce a
successful static build that fails at runtime; the validation gate refuses that
state and obvious server-side Supabase keys.

---

## Managed first-client onboarding — 2026-09-24

**Decision.** The first client uses one operator-provisioned domain, origin,
certificate and shadow rollout. Authenticated dashboard users may list their
domains, but public domain creation is unavailable until DNS ownership proof,
automatic certificate issuance and a safe activation lifecycle exist. A
pending domain is never displayed as protected. Dashboard billing, contact
submissions and policy controls must not imply an effect they cannot deliver.

**Why.** The old add-domain endpoint accepted any host into a unique database
row before ownership proof; an account could reserve somebody else's domain.
The contact form logged personal data and displayed success without sending
anything. The first pilot can be delivered with explicit operator checks while
the full self-service control plane is built and tested separately.

---

## Passed-cookie detection continuity — 2026-09-24

**Decision.** A passed cookie suppresses repeating a low-risk challenge, but
Guard evaluates every later request once. A hard-block score still blocks;
server-observed velocity or crawl findings rate-limit even below the block
threshold. Existing tenant policy skip semantics for solved sessions remain
explicit in evidence. Chromium major, platform and mobile client-hint
contradictions remain shadow-only because those fields are client controlled.

**Why.** Previously a bot could solve one challenge with a browser, switch to
an openly scripted client or known scraper TLS fingerprint, and reach the
origin for the cookie lifetime. Running only the velocity check after a solve
discarded stronger new evidence. This change adds the remaining bounded Redis
checks to passed requests; production latency must be measured on the pilot.

---

## Pilot replay storage and first Phase 3 candidate — 2026-09-24

**Decision.** Tenant-scope each dynamic Redis counter; never evict a live
spent challenge nonce to admit a new solve. The single-node Compose stack uses
`noeviction` Redis with an AOF volume and a stable, minimum 32-byte signing
secret. The public mux exposes verification but not direct puzzle issuance;
the default tenant binds to an explicit host in deployment. A Chromium
user-agent/client-hint major mismatch is recorded separately as shadow evidence
and has zero influence on the existing score.

**Why.** Shared counter keys let one customer's traffic penalize another.
Evicting an unexpired replay marker makes a signed token reusable. A random
startup key invalidates all issued state after restart, and a wildcard default
host can forward unclaimed hosts to the wrong origin. Client hints are cheap to
compare but client-controlled and can be affected by browser variants, so a
production weight needs real traffic review first.

**Limits.** Redis outage fallback is per-node; this release targets one node.
AOF durability is not an absolute replay guarantee across a crash. Full
multi-node replay guarantees and calibrated candidate enforcement are deferred
until the corresponding outage and false-positive evidence exists.

---

## Phase 2 verification hardening: bounded PNG and server-enforced attempt expiry - 2026-09-23

**Decision:** Decode the browser's submitted PNG only on the challenge
verification path, cap its encoded size, require the canvas's fixed 300x150
dimensions and visible nonuniform pixels, and reject malformed/blank images.
The signed failed-attempt cookie now contains its issue time, checked by the
server against the 15-minute window; browser cookie MaxAge alone is not a
security boundary. Failed or JavaScript-disabled visitors get recovery text.

**Limit:** A script can generate a valid PNG and solve the PoW without a
browser. These checks do not promote challenge solves from candidate labels
to verified human labels. Rejecting all unusual but valid renderings would
increase false positives, so image validation stays deliberately coarse.

## Adaptive challenge difficulty: server-chosen, cookie-tracked, mobile-capped - 2026-09-23

**Decision:** Phase 2's challenge difficulty is chosen by the server from
the score `core.Guard` already computed, never from anything the client can
influence, and it travels only inside the HMAC-signed challenge token.
The range is 1–3 leading hex zeros (16/256/4096 expected `crypto.subtle`
hashes). Escalation after failed solves rides a signed, host-bound attempt
cookie with a hard cap (6 attempts, 15 min), deliberately not per-IP state —
per-IP would punish everyone behind one corporate NAT for one noisy script.
A passed cookie's trust window decays with the difficulty the visitor
actually solved (30/15/5 min), re-derived from the difficulty inside the
signature, not from the cookie's MaxAge (which the client controls).
Client-reported telemetry (automation/headless/elapsed) is strictly bounded
and shape-checked, 400 on malformed; a fast solve is measured, never
enforced (a cached page or clock skew would otherwise eat a real visitor).

**Why this shape:** the plan demanded a "strictly capped mobile-safe
range" — difficulty 3 (4096 awaited hashes) is the ceiling until someone
measures a real mid-range phone, because each extra zero multiplies work by
16 on a browser API far slower than a native loop. The client cannot lower
its difficulty (it would make the ladder decorative); it can only drop the
attempt cookie, which loses it nothing it had not earned back.

**Alternatives considered / rejected:** difficulty per IP or per session in
server-side storage (rejected — new request-path state, NAT collateral,
bounded-eviction burden for an input the signed cookie already carries);
trust window enforced only by cookie MaxAge (rejected and mutation-tested —
the client controls its own cookie store, so the window must re-derive from
the signed payload, which is why `Passed()` parses 3 fields); escalating on
one failed solve (rejected — one typo must not make a real visitor's next
page load slower; the step is 2 failures). Honest gaps recorded: the
telemetry envelope is only partially server-trusted — the client computes
its own booleans, so they are bounded and measured, not trusted; real
canvas validation remains ROADMAP item 28, which is what makes these
booleans meaningfully harder to fake.

## Phase 1 policy engine: real DB-backed PolicyProvider, still shadow-only - 2026-09-23

**Decision:** `backend/pkg/policyprovider` is a new package that closes the
gap the previous entry below left open. `Provider.ForTenant(tenantID)`
resolves the tenant to its owning account via a new `tenant.Store.
OwnerUserID` method (never a caller-supplied owner), reads that account's
rules through injected `ruleLister`/`settingsGetter` interfaces (satisfied
by `*rules.Store`/`*settings.Store`, or fakes in tests), and returns a
`*policy.Policy` built by a new `rules.ToPolicy` adapter. Results are
cached per owner (not per tenant — an owner's rules apply across every
domain they own) for 30 seconds, with a 10-second negative cache for a
DB error or an owner with nothing configured, and a 4096-entry bound with
oldest-first eviction. `main.go` attaches it (`guard.WithPolicyProvider
(provider.ForTenant)`) whenever `-db-url` is set — no Supabase auth
required, since this reads already-stored rules server-side. `tenant.
TenantConfig` gained `OwnerUserID`, populated from a new `owner_user_id`
select in `db.GetTenant`/`GetTenantByID` (the column already existed;
nothing selected it before this).

**Why:** This closed the missing binding of rules to validated tenant identity
through a bounded, cached provider, tested through the real DB-to-guard
path for two owners and two domains. `rules.ToPolicy` re-validates every
row through `policy.ValidateRule` at read time (not just at Create) and
drops anything that fails, because a guardrail added after a rule was
saved must not silently start being enforced/ignored differently from
what `ValidateRule` says today — the alternative (trusting stored rows
unconditionally) would let a Create-time bug ship a rule that quietly
never fires, or one written under different rules than current code
ships with, evaluate anyway.

**Alternatives considered / rejected:** Caching by tenant ID instead of
owner ID (rejected and mutation-tested — `mitigation_rules` is
owner-scoped, so two domains under one account must share one cache
entry and one Postgres query, not one each; a test asserts `rules.List`
is called once, not twice, for two tenants under the same owner).
Precompiling `MATCHES` regular expressions when building the policy
(deferred, not rejected — `Condition` would need a `sync.Once`-guarded
field and a pointer-receiver `matches`, since the same `*policy.Policy`
value is shared across concurrent requests once cached; that's a real
concurrency hazard to introduce under time pressure for a feature that
is still fully inert, so it's tracked as open work in the plan doc
instead of rushed here). A live Postgres integration test in this
package (rejected — `pkg/rules`' own Postgres integration test already
covers the SQL; this package's test doubles plus one real-`tenant.Store`
+ real-`core.Guard` end-to-end test (`TestDBToGuardPath_TwoOwnersTwoDomains`)
prove the wiring without needing a database in every CI run).

**Revisit when:** Building the enforcement flip — needs rule ordering/
version/rollback storage (`Policy.Version` has no persistence today),
a decision for the guard branches that return before policy evaluation
runs (verified good bots, already-solved challenges, honeypot trips all
short-circuit before `shadowPolicyOpinion` is reached), and a shadow
period's worth of `policy_shadow_match_total` / agreement data. Revisit
regex precompilation once real traffic makes it a measured cost, not a
guess.

## Phase 1 policy engine: pure package, shadow-only guard wiring, provider left unattached - 2026-09-23

**Decision:** `backend/pkg/policy` evaluates an account's dashboard rules
against a request (`Evaluate`) with no database or HTTP dependency, and
validates a candidate rule (`ValidateRule`) against two structural
guardrails: a rule cannot grant PASS from a single User-Agent condition
alone, and a DECEIVE rule needs a Threat Score floor strictly above the
account's configured block threshold. `core.Guard` gained an optional
`PolicyProvider func(tenantID string) *policy.Policy` hook (mirrors the
existing `WithShadowModel` pattern): when attached, the match is recorded
on `evidence.Evidence.Policy`, but `signals.Decision` — computed earlier,
unconditionally — is the only thing `ServeHTTP`'s switch acts on. No
provider is constructed or attached in `main.go` in this change, so the
feature is inert in the deployed service until that follow-up lands.

**Why:** This is the split agreed with Codex CLI (working the same repo,
see `codex-claude-chat.json`) for Phase 1 of
`docs/BACKEND_IMPLEMENTATION_PLAN.md`: Claude takes `pkg/policy` and the
shadow-only `guard.go` wiring; Codex takes `pkg/rules` Create/Update
validation (unknown field/operator/action, zero-condition block/deceive
rules) using the same `ValidateRule` guardrails. Landing the provider
wiring separately — rather than in the same change — keeps this slice to
files neither side needs to touch to build the other's half, and keeps
the "shadow before enforce" guardrail literal: there is no code path yet
that can act on a policy match at all, not just a flag that's off.

**Alternatives considered / rejected:** Loading rules straight from
`pkg/rules`'s Postgres store inside `guard.go` (rejected — `TenantConfig`
has no `OwnerUserID` today, `pkg/tenant`'s DB loader doesn't select it,
and wiring that in touches `pkg/tenant`/`pkg/db`/`main.go`, none of which
either side had claimed in the relay coordination; doing it under time
pressure risked a collision with Codex's `pkg/rules` work happening in
parallel). Making `pkg/policy` import `pkg/rules` directly for a
`CustomRule`→`Rule` adapter (rejected — keeps the two packages
independently testable and avoids a one-directional dependency neither
side asked for yet).

**Revisit when:** Building the real `PolicyProvider` — needs
`TenantConfig.OwnerUserID` loaded from the `tenants` row (the column
already exists, `pkg/tenant`'s `TenantLoader` just doesn't select it
yet), a `pkg/rules`→`pkg/policy` adapter, and a bounded/TTL-cached lookup
so the request path doesn't take a Postgres round trip per request (same
cost concern as the existing `negativeHost` cache in `pkg/tenant`). Only
after a measured shadow period with recorded agree/disagree data (see
`observability.Inc("policy_shadow_match_total")`) does policy output earn
a second, separately reviewed change to actually drive enforcement.

## Audit P0 routing, JA4, host-cache, and public-origin posture - 2026-09-22

**Decision:** Internal challenge routes are mounted exactly (/__hakaishield/challenge and /__hakaishield/verify) instead of claiming the whole /__hakaishield/ subtree; aggregate JA4 velocity fails open until a common-browser prefix database is loaded; unknown Host database misses are negatively cached for a short bounded TTL; and customer/dashboard-created origins use a public-origin proxy that rejects internal IP targets and rechecks the connected address at dial time.

**Why:** The hosted service cannot let an internal route registration silently disable health checks and honeypot evidence, cannot mass-challenge real browser JA4s before the browser database exists, cannot let random Host headers become attacker-controlled Postgres work, and cannot let an authenticated tenant turn hakaishield into an internal-network fetcher.

**Alternatives considered / rejected:** Reject unknown hosts before any database lookup (rejected for now because existing lazy loading needs one exact lookup before falling back to the single-tenant wildcard); applying strict public-origin validation to all NewOriginProxy calls (rejected because the default CLI -target path and tests intentionally support local/self-hosted loopback origins); seeding fake common-browser prefixes in code (rejected because a stale placeholder list would create a different false-positive risk).

**Revisit when:** Domain verification and ACME onboarding are built, because the origin validation and pending/active lifecycle should become one coherent onboarding policy; revisit the JA4 fail-open once a maintained browser fingerprint feed exists.

## Phase 0 closeout: SNI/Host lifecycle, shared challenge nonces, and aggregate observability - 2026-09-22

**Decisions:**

- Tenant host mapping is canonicalized before storage and lookup. A canonical
  host cannot be mapped to two tenants in one process, malformed hosts are
  rejected before tenant lookup, and TLS requests with a non-empty SNI that
  differs from the HTTP Host return 421.
- Dashboard-created domains stay in `pending_verification` and are readable by
  owner-scoped admin APIs, but they do not route visitor traffic until their
  status is `active`.
- Challenge replay protection now has a Redis nonce store. Healthy Redis gives
  node-wide single-use semantics; Redis failure degrades to the bounded local
  store rather than locking out real visitors.
- Operational counters are aggregate-only and bearer-token protected at
  `/__hakaishield/observability` when `-observability-token` or
  `HAKAISHIELD_OBSERVABILITY_TOKEN` is configured. Counter names are code-owned
  and contain no raw IPs, hosts, tokens, credentials, or visitor payloads.
- Real Postgres tenant-isolation coverage is opt-in through
  `HAKAISHIELD_TEST_DATABASE_URL`. The test creates a random schema and refuses
  URLs that do not look local/test-oriented, so it does not mutate production
  Supabase by accident.

Why: Phase 0 is the safety foundation. Cross-tenant SNI/Host mismatches,
pending-domain routing, replay across nodes, and invisible JWKS/DNS/Redis/origin
failure rates are all production hazards in a hosted request-path product.

## Request-path hardening and admin isolation — 2026-09-22

**Decisions:**

- Unknown JWT `kid` values use a short-lived bounded negative cache and
  serialized JWKS refreshes. This prevents attacker-controlled tokens from
  turning every failed dashboard request into an outbound Supabase JWKS call,
  while preserving quick key-rotation recovery.
- Goodbot reverse/forward DNS verification uses a non-blocking concurrency
  budget. A claimed crawler that cannot be verified is treated as unverified;
  request handlers are never queued behind unbounded DNS work.
- The resolved client IP is carried from `Guard` into the origin proxy. The
  proxy must not re-parse visitor forwarding headers or substitute the direct
  load-balancer peer after the trusted-proxy boundary has been established.
- Dashboard stats require Supabase authentication and, with Postgres enabled,
  ownership of the requested tenant. The legacy query parameter is not an
  authorization boundary.
- Exact database tenant lookup runs before the single-tenant wildcard fallback;
  otherwise the wildcard makes multi-tenant lazy loading unreachable.

Why: these choices satisfy the implementation plan's Phase 0 requirements for
bounded request-path work, tenant isolation, and separation of data-plane and
admin APIs without adding a new external dependency. Full verification and
known follow-up gaps are recorded in `docs/PROGRESS.md`.

## Built-in `.env` loader instead of a dependency; found a plaintext-password-in-logs bug while wiring it — 2026-09-21

**Decision:** `backend/main.go` gained a ~20-line `loadDotEnv(".env")`
that sets process env vars from a local `.env` file without
overwriting ones the shell already set, called at the top of `main()`
before flags are defined. `-db-url`/`-supabase-url` now default to
`$DATABASE_URL`/`$SUPABASE_URL`.

**Why hand-rolled instead of `github.com/joho/godotenv` or similar:**
the actual behavior needed — split each line on the first `=`, skip
blank lines and `#` comments, strip surrounding quotes, never clobber
a real env var — is small enough that a dependency buys nothing but
another line in `go.sum` (`CLAUDE.md` Section 20, standard-library-first).

**Bug found while verifying this, unrelated to the loader itself:**
`main.go` logged the full `-db-url` value — including the database
password — on every successful Postgres connection. Real Supabase
password would have gone into plaintext logs (and, on most hosted
platforms, into a third-party log aggregator) the first time this ran
in anything but a bare terminal. Fixed with `redactCredentials()`,
which parses the URL and replaces only the password component,
leaving everything else (including the username, which is useful for
debugging which role connected) intact. Verified the fix actually
prevents the leak — not just that a function named `redactCredentials`
exists — via a test that asserts the literal password string is
absent from the output, mutation-checked by disabling the redaction
branch and confirming the test fails with the real password visible
in its own failure message.

**Verified how:** real end-to-end run against the live Supabase
project (see `docs/PROGRESS.md`) — `.env` (gitignored, contains the
password the owner pulled from the Supabase dashboard) loaded
automatically, backend connected to the real Postgres, the startup
log showed the connection string with `REDACTED` in place of the
password, and `/domains`/`/rules`/`/settings/protection` correctly
rejected requests with no token, a garbage token, and a
well-formed-but-wrong-signature token — the last two proving the
backend actually validated against the real Supabase JWKS endpoint
rather than merely checking a token's presence.

**Known gap:** a real signed-in session hitting these endpoints is
still unverified — needs one real verified email, which this session
correctly declined to fake past its own safety classifier. See
`docs/PROGRESS.md`'s matching entry.

**Revisit when:** the owner verifies a real signup email and the full
signed-in loop can be confirmed end-to-end.

---

## Migrated dashboard auth from custom bcrypt+JWT to Supabase Auth — 2026-09-21

**Decision:** deleted `backend/pkg/account` (bcrypt signup/signin, a
`users` table) and the custom HMAC `pkg/auth` issuer entirely.
Signup, signin, sign-out, email verification, and password reset now
go straight from the frontend (`dashboard/src/lib/supabaseClient.ts`)
to Supabase Auth. `pkg/auth` still exists but now does one thing:
verify the session JWT Supabase already issued, by fetching that
project's public JWKS (`<url>/auth/v1/.well-known/jwks.json`) and
checking the ES256 signature — no shared secret, no password ever
touches this backend. `owner_user_id` on `tenants` /
`mitigation_rules` / `protection_settings` changed from this backend's
own `usr_`-prefixed string IDs to Supabase's `auth.users.id` (UUID),
with a real foreign key.

**Why:** the owner asked directly ("Supabase chahiye for db and auth")
after the earlier session's remaining-work list flagged "email
verification and password reset are blocked — no email service
configured" as the biggest concrete gap. Supabase Auth's built-in
email flow (verification on signup, reset-password emails) solves
that gap for free, without standing up a separate SendGrid/SES
integration this project has no credentials for. It also deletes an
entire class of code this backend has no business owning — password
hashing, session issuance, email delivery — in favor of a managed
service built for exactly that.

**Why ES256/JWKS instead of the project's shared HS256 JWT secret:**
Supabase exposes both (legacy projects use a shared HS256 secret;
this project, created 2026-09-21, defaults to per-project ES256 asymmetric
keys with a public JWKS endpoint). JWKS verification needs nothing
secret at all on this backend's side — confirmed via
`curl https://<project>.supabase.co/auth/v1/.well-known/jwks.json`
before writing any code, rather than assumed. A leaked backend config
under the shared-secret model can forge sessions; under JWKS
verification it can't, because the backend only ever holds a public
key. The `-supabase-url` flag replaces the old `-jwt-secret` flag
entirely — there's no secret to configure.

**What moved to Supabase, concretely:**
- `POST /auth/signup`, `POST /auth/signin`, `GET /auth/me`,
  `POST /onboarding/complete` (this backend's own endpoints) — all
  deleted. The frontend calls `supabase.auth.signUp()`,
  `signInWithPassword()`, `getUser()`, and stores
  `onboarding_complete` in Supabase's own `user_metadata` via
  `updateUser()` instead of a dedicated backend call.
- `ForgotPassword.tsx` — previously a fake `console.log` (same shape
  of problem as the Payment.tsx issue found earlier this session);
  now calls `supabase.auth.resetPasswordForEmail()` for real.
- The `users` Postgres table — deleted; `auth.users` (Supabase-managed)
  is the only user table now.

**What did NOT move, and stayed on this backend:** domains, mitigation
rules, and protection settings CRUD — none of that is an auth concern,
and Supabase's Postgres is used as *a database*, not as a reason to
move unrelated business logic into Supabase's own tooling (Edge
Functions, PostgREST). The Go backend keeps its own direct `pgx`
connection to that same Postgres instance for those tables.

**Alternatives considered:**
- *Route domains/rules/settings through Supabase's PostgREST/RLS
  instead of a direct Go/pgx connection.* Rejected for this pass: it
  would mean re-deriving this backend's existing, tested query logic
  as RLS policies, and mixing two different access patterns (direct
  SQL for the proxy's own tenant lookups, PostgREST for the dashboard)
  for the same tables. RLS policies were still added (see the
  `hakaishield_core_schema` migration) as defense-in-depth even though
  this backend's own connection bypasses them (connects as the
  privileged role) — Supabase's advisor linter expects them on any
  public-schema table.
- *Keep the custom JWT issuer and only add Supabase for the database.*
  Rejected: that would mean maintaining a second, parallel auth system
  (this backend's own signup/signin) *and* Supabase's, for no benefit
  — the whole point was solving the email-verification gap, which only
  Supabase's own Auth flow (not just its Postgres) provides.

**Verified how:**
- Backend: full `go build`/`go vet`/`gofmt`/`go test ./...` pass.
  `pkg/auth`'s new JWKS-based `Verifier` has its own test suite
  (round-trip against a real ES256 key pair via a fake JWKS server,
  expired-token rejection, wrong-key rejection, unknown-`kid`
  rejection, alg=none rejection, key-rotation recovery — the rotation
  test itself caught a bug in the *test's own* mock JWKS handler
  closing over the wrong variable, fixed before trusting the result).
  Confirmed the deployed project really serves an ES256/JWKS endpoint
  via a direct `curl` against
  `https://oxvwvzthqnttehqwfgux.supabase.co/auth/v1/.well-known/jwks.json`
  before writing the verifier, rather than assuming Supabase's default.
- Frontend: `npm run typecheck` and `npm run build` clean.
- **Real signup against the live Supabase project** via Playwright,
  visible Chrome: `supabase.auth.signUp()` returned 200 with a real
  user UUID and `confirmation_sent_at` timestamp, and the frontend
  correctly showed a "check your email" state (no session yet, since
  the project has email confirmation on by default). Note: the first
  attempt used an `@example.com` address and Supabase rejected it
  (`email_address_invalid`) — Supabase blocks known placeholder
  domains; retried with a plausible real domain and it went through
  normally, which is expected production behavior, not a bug.
- **Not verified end-to-end with a live signed-in session**: manually
  confirming the test account's email via SQL
  (`UPDATE auth.users SET email_confirmed_at = ...`) was blocked by
  this session's own auto-mode safety classifier as an
  auth-bypass-shaped action, and that block was respected rather than
  worked around. So the full loop — sign in with a real Supabase
  session → call the Go backend's `/domains`, `/rules`,
  `/settings/protection` with that real token → confirm the backend's
  `Verifier` accepts it — is proven correct in two *separate* pieces
  (Supabase issues real ES256 tokens; this backend's `Verifier`
  correctly validates ES256 tokens against a JWKS server in tests) but
  not as one continuous live path. See Known gaps.
- **Not verified at all**: the actual Go backend running against the
  new Supabase Postgres — this session does not have that database's
  connection password (Supabase doesn't expose it via the MCP tools
  used to create the project; only the dashboard shows it, and only
  right after a manual password reset). The `hakaishield_core_schema`
  migration was applied and confirmed clean by
  `get_advisors(type: "security")`, but `backend/pkg/db`,
  `pkg/rules`, `pkg/settings` were never run against it this session.

**Known gaps / follow-up:**
- **The owner needs to get the Supabase Postgres connection string**
  (Supabase dashboard → Project Settings → Database → Connection
  string, or reset the DB password there) and supply it as `-db-url`
  before the domains/rules/settings API can run for real.
- **The owner needs to verify one real email** (their own, or confirm
  a test account) to prove the full signed-in loop end-to-end, since
  this session couldn't bypass email confirmation.
- Supabase's project-level email sending (the built-in "Inbucket"-style
  sender used before custom SMTP is configured) has low rate limits
  on the free tier — fine for the testing done here, not meant for
  production volume. Configuring a custom SMTP provider in the
  Supabase dashboard is a follow-up before real signups scale.
- The `dashboard/BACKEND_WIRING_DOCS.md` file (the original
  aspirational Node.js backend spec this dashboard was scaffolded
  against) is now further out of date — it still describes this
  backend's own auth endpoints, which no longer exist. Not rewritten
  this session; it was already noted as aspirational/superseded in
  earlier entries and updating it further wasn't asked for.

**Revisit when:** the owner has supplied the Supabase DB connection
string and verified a real signed-in session end-to-end; at that point
this entry's "not verified" gaps close.

---

## Removed the fake billing UI instead of leaving it as a known gap — 2026-09-21

**Decision:** `Subscription.tsx` and `Payment.tsx` no longer show
fabricated account state (a fake "Growth plan, $500/mo, 4.2M/10M
requests" summary, three fake "Paid" invoices, and a card-collecting
form that falsely claimed "Secure payment processed by Stripe").
Both pages now show a plain "not wired up yet, contact us" notice.

**Why this got fixed instead of just logged as a gap:** everything
else deferred this session (SIEM, WAF, traffic chart) was an *absence*
— a section that does nothing and says so. This was different: a form
that looks exactly like a real Stripe checkout, explicitly claims
Stripe processes and encrypts the card data, and then does a
`console.log` + fake success redirect. A real user has no way to tell
this apart from a working payment form except by inspecting network
traffic. That crosses from "unfinished feature" into "actively
misleading, and specifically about payment security" — CLAUDE.md
Section 27 exists for exactly this shape of problem, and unlike the
other placeholders, leaving it in place risked a real person typing a
real card number into it.

**Why not just add a warning banner instead of removing the form:** a
banner next to a form that still visually looks and behaves like a
working checkout doesn't fix the actual risk — the field pattern
(`1234 5678 9012 3456` placeholder, CVC, expiry, "Pay $X and
subscribe") signals "this is real" more loudly than a small notice
signals "this isn't." Removing the card-collecting form entirely was
the only version of this that couldn't be misread.

**Alternatives considered:** stub in a real Stripe test-mode
integration — rejected for this pass; that's real backend/API-key work
(`docs/ROADMAP.md` item 12.1), not a UI honesty fix, and conflating the
two would have meant either shipping a half-wired Stripe integration
or delaying the honesty fix behind it.

**Revisit when:** real Stripe credentials exist and payment gets built
for real (`docs/ROADMAP.md` item 12.1) — at that point this page
becomes the real checkout instead of a placeholder.

---

## Dropped Go 1.22+ method-prefixed route patterns for the dashboard API — 2026-09-21

**Decision:** every `mux.HandleFunc` registration for the
account/domains/rules/settings API in `backend/main.go` (11 routes)
uses a bare path (`"/api/v1/auth/signup"`) instead of a
method-prefixed pattern (`"POST /api/v1/auth/signup"`). Each handler
checks `r.Method` itself instead.

**Why:** `net/http`'s `ServeMux` (Go 1.22+) rejects any request whose
method doesn't match a method-prefixed pattern *at the routing layer*,
before the handler runs. A browser's CORS preflight is always an
`OPTIONS` request, so every method-prefixed route in this API was
unreachable by preflight — the request never got far enough to hit
each handler's own `if r.Method == http.MethodOptions { return }`
short-circuit and its `Access-Control-Allow-Origin` header. Every
`curl`-based test in the two previous sessions' entries passed anyway,
because `curl` doesn't send a preflight; only a real browser does.
Found by an actual Playwright run in a real Chrome browser — see
`docs/PROGRESS.md`'s matching 2026-09-21 entry for the reproduction
and the full fix verification.

**Why drop the prefix instead of registering a second OPTIONS-only
route per path:** every handler already validates its own method
(either directly or via `RequireAuth`'s common CORS/method handling),
so a second registration per route would be pure duplication for no
benefit — the bare-path pattern plus the existing per-handler checks
already produce the same behavior with less surface area.

**Known gap this doesn't cover:** production CORS posture.
`Access-Control-Allow-Origin: *` is set on every one of these routes,
which is fine for local dev (dashboard and backend both on
`localhost`) but permissive by default for a real deployment where the
dashboard and API are on different real domains. Not changed here —
scoping allowed origins is a separate decision from "does the
preflight even reach the handler," and conflating them would have
delayed the actual bug fix.

**Revisit when:** a real multi-tenant production deployment is being
configured — origin allowlisting should be decided then, not
defaulted to `*` by inertia.

---

## Dashboard wiring: real account/domains/rules/settings API, same binary, no separate control-plane service — 2026-09-21

**Decision:** wired `dashboard/` (previously a UI-only scaffold with mock data — see the 2026-09-21 "Brought the untracked dashboard frontend into the repo" entry) to a real backend API served by the same `hakaishield` binary: `pkg/account` (users + bcrypt), `pkg/auth` (JWT issue/verify), `pkg/rules` (custom mitigation rules), `pkg/settings` (protection thresholds), plus new handlers under `pkg/api` (`auth.go`, `domains.go`, `rules.go`, `settings.go`, `dashboard_extra.go`) gated behind two new flags, `-db-url` and `-jwt-secret`. Domains reuse the existing `tenants` table (extended with `owner_user_id`, `name`, `status`, `created_at`) rather than a new table — a "domain" in the dashboard's terms is exactly what `tenant.Store` already models.

**Why not a separate service:** `BACKEND_WIRING_DOCS.md` (written before this session, describing an aspirational Node.js/Express control plane) recommends a whole second stack — Node, its own Postgres/Redis, JWT, Stripe, SendGrid, S3. Standing that up is a different, much larger project than "wire the existing frontend to something real," and this repo already has one Go binary, one Postgres connection (`pkg/db`), and an existing account-free dashboard API (`/api/v1/dashboard/stats`, `/evidence`). Adding account/domain/rule/settings HTTP handlers to that same binary reuses all of it; a second service would mean two deployments, two databases (or a shared one two codebases both migrate), and duplicated tenant/auth logic. If this product later needs a real multi-region control plane, that's a deliberate future split, not a default.

**What's real vs. placeholder, and why each placeholder stopped where it did:**
- **Auth (signup/signin/me/onboarding-complete):** real, bcrypt + JWT, tested against a live Postgres. No email verification or password reset — those need an email service (SendGrid/SES), which needs a real API key nobody has provided; a new account is usable immediately instead of gated behind a verification step the product can't complete. Deliberately deferred, not skipped-and-hidden — see `README.md`'s "Dashboard (frontend)" section, which says so.
- **Domains:** real CRUD against the `tenants` table. A newly added domain isn't taking live traffic immediately — enforcement starts once `tenant.Store` picks the row up, either lazily (first real request for that host, via `GetByHost`/`fetchFromDB`) or, after this session's fix, lazily via a *dashboard* read too (`GetByID` now also falls back to the database, see the bug entry below). There is still no in-process "start enforcing on this host right now" push; a fresh domain enforces once something looks it up.
- **Mitigation rules:** the four always-on `pkg/signals` layers are listed as read-only "managed rules" (cannot be toggled off from the dashboard — turning off JA4 fingerprinting is a code change, not a switch a visitor-facing account should have, per `CLAUDE.md` Section 10). Custom rules are real CRUD (stored as JSON conditions) but **not enforced** — nothing in `pkg/core`/`pkg/signals` reads `mitigation_rules` yet. Wiring a custom rule into the live scoring decision is a separate, larger integration (need to decide how an arbitrary JA4/score/path condition composes with the existing signal-based scoring) and wasn't attempted blind.
- **Protection settings:** real GET/PUT against a `protection_settings` table, with the same validation rule the UI implies (block threshold > challenge threshold). **Not wired into `pkg/signals/score.go`'s live thresholds**, which are still fixed in code — same reasoning as custom rules: this is preference storage today, not a live control, and saying so beats a slider that silently does nothing.
- **Top offenders / evidence logs:** real, aggregated from the existing in-memory `evidence.Trail` (JA4 + decision counts) — no new store added, since the trail already had exactly this data. IP/geo/ASN/method/path shown in the old mock UI don't exist in `evidence.Evidence` (deliberately — see its own doc comment on visitor privacy) and were removed from the dashboard tables rather than faked.
- **Traffic-over-time chart:** removed and replaced with an explicit "not available yet" note. `stats.Stats` is running atomic counters, not a time series; building one is real scope (a bucketed store, a retention policy) that wasn't invented to fill a chart.
- **Payments (Stripe), SIEM integrations, WAF toggles:** not touched. All three need either real third-party credentials nobody has provided (Stripe, SIEM vendor APIs) or are themselves substantial unbuilt features (WAF). The dashboard says so in place of the toggle rather than pretending they work.

**Bug found and fixed during this work:** `tenant.Store.GetByID` only ever checked its in-memory map — unlike `GetByHost`, it had no database fallback. A domain added via the dashboard (which only writes to Postgres) would 404 as "tenant not found" from `/api/v1/dashboard/stats` until the *proxy* happened to receive a real request for that host first (which populates the in-memory store via `GetByHost`/`fetchFromDB`). Fixed by giving `GetByID` the same lazy-DB-fetch fallback, factored into a shared `addFromDBRow` helper. Found by manually driving the real HTTP API end-to-end against a live Postgres (see Tested how below) — a domain added via `POST /domains` and immediately queried via `GET /dashboard/stats?tenant=<id>` returned `tenant not found` before this fix, real stats after.

**Second bug found in the same pass:** the origin format `dashboard/BACKEND_WIRING_DOCS.md` itself documents as an example (`"10.0.1.50:8080"`, no scheme) is not a URL `core.NewOriginProxy` can parse (`url.Parse` requires a scheme). Submitting exactly the documented example would have silently stored an unusable origin and only failed later, opaquely, as the same "tenant not found". Fixed with `normalizeOrigin` in `pkg/api/domains.go`, which accepts either form and prepends `http://` when no scheme is present — with its own test covering the edge cases (`"http://"`, `"://bad"`) that a naive "does it contain `://`" check gets wrong.

**Third bug found in the same pass:** `evidence_token` is a nullable column, and a domain created via the dashboard never sets it — `pgx` cannot scan SQL `NULL` into a non-pointer `*string`, so both `GetTenant` and the new `GetTenantByID` panicked/errored on any domain without a token. Fixed with `COALESCE(evidence_token, '')` in both queries.

**Alternatives considered:** building the Node.js/Express backend `BACKEND_WIRING_DOCS.md` describes — rejected as described above (unjustified second stack for what a few Go handlers on the existing binary already cover). Faking the missing pieces (a canned traffic chart, a WAF toggle that does nothing, mock SIEM "Connected" states) — rejected outright: `CLAUDE.md` Section 27 exists specifically because a dashboard that lies about state is worse than one that admits a gap.

**Tested how:** `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` (all packages, including new unit tests for `pkg/auth` — JWT round-trip, tampered signature, expired token, alg-none/algorithm-confusion rejection, missing-claim rejection; `pkg/account` and `pkg/settings` — validation-before-database-check ordering; `pkg/rules` — condition JSON round-trip, managed-rule invariants; `pkg/api` — `RequireAuth` middleware, `normalizeOrigin`). Mutation-checked: JWT signature verification (swapped in a fixed wrong key → round-trip test failed as expected, restored → passed), the settings threshold rule (`if false` in place of the real check → validation test failed as expected, restored → passed), `normalizeOrigin`'s `://` branch (forced into the wrong branch → four cases failed as expected, restored → passed). Beyond unit tests: stood up Docker Postgres + Redis and drove the **real HTTP API end-to-end** — signup, duplicate-email rejection, weak-password rejection, signin, wrong-password rejection, `/auth/me` with and without a token, onboarding-complete, add/list/duplicate-reject a domain, list/create/toggle a mitigation rule, get/reject-invalid/update protection settings, and (after the two bugs above were found and fixed) live `/dashboard/stats` and `/dashboard/top-offenders` for a domain that had never received proxy traffic. Frontend: `npm run typecheck` and `npm run build` clean after every page's wiring.

**Known gaps / follow-up:** all listed inline above per feature; the full list also lives in `docs/PROGRESS.md`'s matching entry and `docs/ROADMAP.md` item 12.

**Revisit when:** email service credentials exist (verification/reset), Stripe credentials exist (payments), someone decides how a custom rule composes with signal-based scoring (rule enforcement), or the running-totals-only limitation of `stats.Stats` becomes a real product ask (traffic-over-time chart).

---

## Honeypot trap: scoped to (tenant, IP, JA4), scored not blocked — 2026-09-20

**Decision:** a honeypot trip is recorded against the triple (tenant ID, source IP, JA4) with a 6h TTL and a hard 50k entry cap, and feeds scoring as a 50-point `honeypot_trap` signal. It is explicitly **not** written to the shared `ja4:scrapers` blocklist.

**Why not the blocklist:** the original implementation called `AddKnownScraperJA4(ja4, ...)` on every trip, which lands in the map `score.go` reads for its 100-point `ja4_blocklist` check. JA4 identifies a *browser build*, not a machine — every real Chrome user on a given version shares one fingerprint. So one headless-Chrome scraper tripping the trap would have hard-blocked every genuine Chrome visitor of that version, on **every tenant**, permanently and with no expiry. That is the worst false positive this product can produce (`CLAUDE.md` Sections 14 and 16), triggered by a single request.

**Why (IP, JA4) and not either alone:** JA4 alone is too coarse (above). IP alone is too coarse the other way — CGNAT and corporate egress put unrelated people on one address. Requiring both means a trip counts against the caller that actually walked into the trap and close to nobody else.

**Why 50 points and not a block:** following a `display:none` link is strong evidence of DOM-walking automation, but not proof of malice. Screen readers walk the DOM, and browsers speculatively prefetch. At 50 a lone trip lands in challenge range, which a real person recovers from in one round trip, while an actual scraper also trips the handshake, UA-mismatch or crawl-pattern signals and crosses the block bar on combined evidence — which is what `CLAUDE.md` Section 6 requires anyway.

**Rejected:** Redis fan-out of trips (`HSet` + `Publish` per hit, each in its own goroutine). It spawned an unbounded goroutine and two Redis round trips per request to an endpoint an attacker can call in a loop, and the `Publish` had no subscriber anywhere in the codebase. Cross-node propagation is a real want, but it needs a bounded writer and a TTL'd, tenant-scoped key; it is a follow-up, not a prerequisite.

**Revisit when:** trips need to propagate between nodes, or when the trap link is injected somewhere other than deceived responses.

---

## Consolidating PR #10 and PR #11; dropping the forensics analyzer — 2026-09-20

**Decision:** the deception and honeypot work ships; the `pkg/forensics` client-behaviour analyzer from PR #10 does not, and PR #10's dependency changes are discarded entirely.

**Why the dependencies were discarded:** PR #10 downgraded every direct dependency — Go 1.25→1.23, `fingerproxy` v1.2.3→v0.6.1, `pgx` v5.11→v5.6, `go-redis` v9.22→v9.7, `sentry-go` v0.49→v0.27. Nothing in the PR needed older libraries; for a product whose job is sitting in the TLS request path, silently moving back across a year of upstream fixes is a security regression, not a neutral change.

**Why forensics was dropped rather than merged:** it is 298 lines with no caller anywhere in the tree, and wiring it as written would have added a signal that *lowers* risk based on numbers the client supplies — a bot posts `{"mouse_entropy": 0.45}` and scores clean (`CLAUDE.md` Section 17). Two of its five layers (mouse entropy, timing variance) also have no safe reading on touch devices or for keyboard-only visitors, so a naive collection script would have penalised every mobile and accessibility user (Section 14). Separately, the challenge page already collects canvas, WebGL, `navigator.webdriver` and permission-probe telemetry over an HMAC-bound, size-limited POST — so the transport this analyzer needs already exists and the detection partly overlaps.

**What it would take to ship it:** bind the payload to the existing challenge token, define signals that degrade safely on touch and keyboard-only input, and validate against real mobile traffic before it can influence a decision. That is a feature, not a merge conflict, and it is on the roadmap rather than half-wired into the request path.

**Revisit when:** someone picks up client-telemetry scoring as its own piece of work. The code is preserved in PR #10's branch history.

---

## Enterprise Hardening: Adaptive Policy Modes, Verified Good Bots, and Production Transport — 2026-09-19

**Decision:** Bot Shield now supports adaptive policy modes (`PolicyBalanced` vs `PolicyStrict`). Under `PolicyBalanced` (the production default for e-commerce/SaaS), clean visitors (score 0) are passively forwarded to the origin with zero latency, reserving challenges for elevated risk scores (25-99) and blocks for >=100. `PolicyStrict` preserves the mandatory ~50ms invisible interstitial for strict zero-scrape environments. In addition, genuine search engine crawlers (Googlebot, Bingbot, Applebot, etc.) are verified via cached reverse+forward DNS and granted clean passthrough to protect SEO indexing. Upstream proxy connections are backed by a production-tuned pooled transport with explicit timeouts and custom 502/504 error handlers.

**Why:** Forcing 100% of organic visitors through an interstitial challenge page increases bounce rates and harms SEO on e-commerce platforms. Adaptive policy modes provide maximum operational flexibility without compromising security against automated scraping.

**Revisit when:** Dynamic machine-learning based risk scoring thresholds are deployed or multi-region anycast edge nodes are added.

---

## Server-observed request patterns; asset-aware rate limiting — 2026-09-19

**Decision:** request classification by URL path (`isStaticAsset`) is now a first-class server-side fact. The per-IP velocity limit counts navigations and subresources separately (20/s vs 300/s), and a new `crawl_pattern` signal flags a browser-claiming client that walks many distinct page paths in a window (Redis HyperLogLog, >60/min). Scoring now reads a `RequestFacts` struct (IP/JA4/UA/Header/Path).

**Why the velocity change was not optional:** the old counter capped *every* request at 5/s per IP and Guard hard-429'd passed sessions. A real browser loading one page fires dozens of asset requests in a second, so this returned 429 to paying customers' real visitors — a production false positive of the worst kind (`CLAUDE.md` Section 14). Classifying assets server-side fixes the bug and, for free, makes navigation rate a real signal: a person cannot open 20 pages a second, a crawler can.

**Why request patterns are worth a layer:** every other signal we have is either client-reported (spoofable) or TLS-level (defeated by real-browser impersonation). The *shape and rate of requests* is observed by the server and cannot be forged from the client — the attacker's only defence is to slow down, which is the economic goal. This is the layer that the headful-scraper gap needs; it is also why it is scoped to browser-claiming clients (a script is not pretending, so there is no lie to catch — Section 8) and why assets are excluded (a real page load is mostly assets — Section 14).

**Why HyperLogLog, not a set:** distinct-path cardinality per IP is all we need, and HLL bounds that state at ~12KB regardless of how many paths one IP requests (`CLAUDE.md` Section 15 — bounded state against adversarial volume). It also naturally ignores volume: hitting one page 500 times is a reload loop, not a crawl, and must not fire.

**Alternatives considered:**
- *Count distinct paths in a Go map per process.* Rejected: unbounded memory per hostile IP, and wrong across instances on a hosted multi-node deployment.
- *Use `Sec-Fetch-Dest` to classify navigations.* Rejected: a client header, spoofable; the URL path is not.
- *Raise the old single limit instead of splitting it.* Rejected: any single number either 429s real page loads or lets crawlers through — the two request classes genuinely need different limits.

**Revisit when:** real traffic gives real numbers for the caps, or a NAT/mobile-carrier test shows the navigation cap is too tight.

---

## Wire header_anomaly into scoring; drop the orphaned probe.js — 2026-09-19

**Decision:** the header-consistency check (`HeaderAnomaly`, `signals/headers.go`)
is now a real scoring input: `Score`/`Analyze` take the request `http.Header`,
and the single `checks` table gains a `header_anomaly` entry weighted 25 — below
the block bar. Separately, the `/__hakaishield/probe.js` endpoint and its
`probeScript` are deleted.

**Why header_anomaly:** it was fully built and tested but never referenced by
scoring, so it contributed nothing while looking finished — the "built but
unwired" gap `CLAUDE.md` Section 21 warns about. Its weight is deliberately the
lowest: a browser-claiming client that omits every `Sec-Fetch-*` and `Sec-CH-UA`
header is suspicious, but privacy tools and unusual-but-real clients can drop
them, so it must never decide a block alone. 25 on its own stays below the 100
block threshold, and every path to 100 already fires a stronger signal — pinned
by `TestHeaderAnomalyNeverBlocksAlone`.

**Why delete probe.js:** it served a standalone script that set a `_bs_probe`
cookie nothing ever read, and nothing injected it anywhere. That contradicts the
earlier recorded decision (2026-09-16, "Automation probe lives inside the JS
challenge, not injected site-wide"): the probe belongs inside the challenge page,
which is where item 6 actually runs. A second, unused probe endpoint only widened
the attack surface and implied a site-wide injection feature that was explicitly
not built.

**Also recorded:** `Decide` no longer carries a `challengeThreshold` constant it
does not use. With the mandatory interstitial (see the entry below), the honest
contract is "block at 100, challenge otherwise", so a dead threshold that
implied a band which no longer existed was removed rather than left in place.

**Revisit when:** real traffic exists to tune the header_anomaly weight, or
HTML-injection into origin responses becomes a roadmap item (which would let
item 6's probe run everywhere, making a site-wide probe script relevant again).

---

## Fast, Invisible Challenges and Headful Bot Evasion — 2026-09-19

**Decision:** The JS challenge engine now runs at an ultra-fast 8-bit Proof-of-Work (PoW) difficulty (~50ms) and supports theme customization (`ghost` and `branded` modes). It also strictly checks `Error.stack` and `navigator.permissions` to block headful Playwright/Puppeteer (e.g., Scrapling, Patchright) that evade TLS fingerprinting.

**Why:** Clients hate visible interstitial pages ("Checking your browser..."). However, removing the interstitial entirely allows advanced Headful bots to scrape the first page before background telemetry catches them. To guarantee **zero scraping**, the interstitial is mandatory.
To make it "sellable", we implemented:
1. **Ultra-Fast PoW:** Reduced from 12-bit ("000") to 8-bit ("00") solving the 4-second delay caused by JS `await` event loop overhead. It now executes in <50ms.
2. **Ghost Mode (`-theme=ghost`):** A completely blank page that executes the 50ms check and redirects, appearing as normal network latency rather than a security check.
3. **Branded Mode (`-theme=branded`):** An enterprise-style loading spinner for clients who want a visible security check.

**Why Error.stack?** Headful bots (Patchright, Scrapling) perfectly spoof `navigator.webdriver` and JA4 signatures. They are indistinguishable from real Chrome at the network level. However, to control the browser, they inject internal evaluation scripts (`puppeteer_evaluation_script`, `evaluate@`). Throwing a deliberate error in JS and reading the stack trace is the only reliable way to catch them on the very first request without behavioral biometrics.

---

## Back to one agent; Claude Code owns the dashboard too — 2026-09-16

**Decision:** the two-agent workflow is over. The project owner ended
the Antigravity arrangement. `claude_and_agy.md` is deleted,
`CLAUDE.md` Section 25 is replaced with a plain statement of sole
ownership, and `proxy/`, `cmd/hakaishield/` **and** `dashboard/` are all
Claude Code's.

**What does not change:** the `dashboard/` code stays. It works, it is
wired to the real stats endpoint, and `CLAUDE.md` Sections 12 and 13
apply to it exactly as to any other code — read it before changing it,
and don't delete parts of it as "dead" without checking. Ending a
working arrangement is not a reason to throw away working code.

**What does change, and it is the part worth remembering:** the
dashboard now has to meet this file's bar rather than a softer "it's
only UI" one. It needs real tests, it must not fail silently, and it
must behave when the backend is slow, down, or returning an error. A
dashboard that shows a blank card when the API is unreachable is not
a cosmetic bug — it tells the customer their traffic is clean when we
have no idea what their traffic is.

It is also the only part of the product a customer ever sees. They
will never read `proxy/score.go`; they will judge hakaishield entirely
on whether the numbers on screen make sense and are believable.

**Also retired with it:** `agentchat/` (the two-agent coordination
log) has no remaining purpose. It was already gitignored and is not
shipped product, so nothing in the repo depends on it. The earlier
entries in this file about the agentchat MCP server stay as history —
they record a real lesson about why MCP tool calls cannot wake an idle
agent, which is worth keeping even though the setup is gone.

**Revisit when:** a second builder joins, human or agent. At that
point re-read the earlier entries rather than re-deriving the file
collision and edit-war problems from scratch.

---

## Pivot: hosted SaaS is the product; self-hosting becomes Enterprise — 2026-09-16

**Decision:** hakaishield is sold as a **hosted service we run**.
Customers point DNS at us, their traffic flows through our
infrastructure, they pay monthly. Self-hosting is not removed — it
becomes a **priced-up Enterprise option**, not the default.

**Why the owner asked for this:** the self-hosted-only framing had no
path to recurring revenue for someone starting with no capital. A
licence key for software that runs on someone else's server is a
harder sell, a harder collection, and a harder product to keep people
paying for. The owner wants a SaaS. That is a business call, and it
is theirs to make.

**What it costs us, stated plainly so nobody is surprised later:**
- **Bandwidth becomes our bill.** Every byte of a customer's traffic
  now crosses our infrastructure. A modest e-commerce site can mean
  hundreds of GB a month. This is why DataDome and Cloudflare meter
  per request — their costs scale with traffic, and now so do ours.
  **Every plan must therefore carry a bandwidth/request cap**, or one
  large customer erases the margin on ten small ones.
- **We are now in the critical path.** Our downtime is the customer's
  site being down. For a solo maintainer that is an operational
  burden, not a coding one.
- **We now compete with Cloudflare on their own ground**, and they
  have hundreds of points of presence and near-zero marginal cost. We
  cannot win that on price. The differentiators stay what they were:
  evidence, agent governance, and a product aimed at people Cloudflare
  does not serve well.
- **The "your traffic never leaves your infra" pitch is gone** for the
  default product. That was one of the two buyer segments in the
  positioning entry below. It survives only as Enterprise.

**What it gains us, and one gain is bigger than expected:**
- Recurring revenue, self-serve signup, no licence enforcement to
  build, and we control updates.
- **The moat gets easier.** Item 19's fingerprint database was the
  expensive research cost. Hosting every customer's traffic means we
  observe real browser fingerprints continuously — the database can
  largely build itself from our own traffic. A bad fingerprint seen
  on one customer can protect the rest. That cross-customer network
  effect was explicitly rejected as infeasible under self-hosting
  (see the 2026-09-15 competitor scan in `RESEARCH.md`); hosting makes
  it available. It still needs a clear data-handling policy in terms
  of service before it is switched on — `CLAUDE.md` Section 18 has not
  moved.
- JA4 is unaffected: we terminate TLS, so we still see the
  ClientHello. The technical edge survives the pivot intact.

**What "Enterprise" means here** (recorded because it is easy to
misread as a feature tier): it is a *deployment and contract* tier,
not a feature list. Regulated or large customers who cannot send
traffic to our cloud run hakaishield themselves, on a custom price
**above** the hosted plans, invoiced rather than card-billed, with a
contract and direct support. It is sales-led, not self-serve.
**It is not to be built now** — there are zero customers. It exists
in the docs so the option stays open and so a future session does not
delete the self-hosting code path as dead.

**Alternatives considered:**
- *Stay self-hosted only.* Rejected by the owner: no viable recurring
  revenue from a standing start.
- *Ship a decision API instead of proxying traffic* (ROADMAP item 13)
  to avoid bandwidth costs. Rejected as the primary model: without
  terminating TLS we never see the ClientHello, so JA4 — the entire
  technical edge — disappears. It stays a roadmap item for customers
  who want it *in addition*, never as the main product.
- *Open-source the engine and sell hosting, like Supabase or
  Firecrawl.* Rejected: `CLAUDE.md` Section 1 forbids it. Noted here
  only because the model came up in discussion and the pattern —
  monetise the operations, not the bits — is the right instinct even
  though the licensing is not.

**Honest risk:** this reverses the positioning entry below, which was
written the same day. That entry's reasoning about *what we sell*
(evidence, agent governance, not price) still stands; what changed is
*where it runs* and therefore who the second buyer segment is. Two
reversals in one day on zero customer evidence is itself a warning:
the next real signal should come from a customer, not another
strategy session.

**Revisit when:** the first month's real bandwidth bill lands, or a
customer asks for Enterprise self-hosting and we find out what they
will actually pay for it.

---

## The moat is the fingerprint database, not the code — 2026-09-16

**The question that forced this:** *"any competent dev could build
this with Redis and a few checks — why would a company pay us?"*

**The honest first half:** they're right about today's code. Two
signals, fixed thresholds, `fingerproxy` is open source. A good
engineer rebuilds the current hakaishield in a couple of weeks. There
is no answer to that objection in what is built so far, and pretending
otherwise would just mean discovering it in a sales call instead.

**The second half, which is the actual answer:** building it once and
*keeping it working* are different products. Bot detection is not a
feature, it's a standing fight with someone who updates:

- Chrome ships roughly every 4 weeks and its TLS fingerprint moves
  with it. `curl_cffi`, Patchright and friends update specifically to
  break detection.
- An in-house system therefore **decays silently**. It doesn't crash
  or alert — it just drifts toward allowing everything, and nobody
  notices for months. There is no error log for "your detection
  stopped detecting."
- In-house systems also don't get tuned. The first time one blocks a
  real customer, a support ticket lands and the team switches it off.
  Writing detection is the easy part; spending months on false
  positives is the part nobody volunteers for.
- And it sits in front of checkout. The engineer who wrote it leaves
  in eight months and nobody wants to touch it after that.

**Decision:** a maintained **known-browser fingerprint database** is
hakaishield's commercial moat, and gets treated as a first-class
roadmap item (new item 19) rather than as a research cost to avoid.

**Why it, specifically.** `ROADMAP.md` item 3 already identified this
and deliberately deferred it: *"that needs a maintained database of
known-browser fingerprints, which is a real ongoing research cost."*
That framing was right as engineering and backwards as business — the
ongoing cost **is** the defensible asset. It isn't code, so it can't
be copied in a weekend; it's continuously refreshed data, so its decay
becomes our problem instead of the client's. That is what a
subscription actually buys, and it is why CrowdSec can give its engine
away free and still sell tiers: the engine is free, the intelligence
is not.

**Consequence for prioritisation:** adding detection signals #3 and #4
does not make anyone pay. Being able to say *"we track N known browser
builds and refresh them weekly"* does. Items 7–10 are no longer
automatically ahead of item 19.

**Consequence for sales:** a prospect who says "we'll just build it"
is not a customer — don't argue with them. In this category the buyer
has almost always been hurt first (scraped pricing, scalped
inventory, credential stuffing). Pain precedes purchase, which is also
why cold outreach doesn't work here.

**Alternatives considered:**
- *Compete on breadth of signals.* Rejected: signals are the copyable
  part, and `CLAUDE.md` Section 14 already forbids adding them for
  their own sake.
- *Lean on convenience/ease of deployment as the moat.* Rejected as a
  moat (it's a real advantage, but a competitor matches it in one
  release); it stays a selling point, not a defence.
- *Accept "they'll build it themselves" and target only enterprises
  who won't.* Rejected: that's the buyer a solo maintainer can't serve
  — see the positioning entry below.

**Honest risk:** building and maintaining this database is a genuine,
recurring cost and nobody has scoped it yet. It could turn out to be
more work than one maintainer can carry, which would be a real
strategic problem rather than a missing feature. Scope it before
committing to it, and record what that scoping finds here.

**Revisit when:** item 19's scoping says how much work refreshing the
database actually is, or a real client tells us they'd have built it
themselves (which would mean the moat argument isn't landing).

---

## Evidence trail: token-gated, in-memory, one shared signal table — 2026-09-16

**Decision:** ROADMAP item 12a ships as `proxy/evidence.go`: a fixed
1000-entry ring buffer with a 24h retention window, served from
`GET /api/v1/dashboard/evidence` behind a bearer token that must be
set with `-evidence-token` or the endpoint isn't mounted at all.

**Why token-gated when `/stats` is wide open.** `/stats` returns four
counters — aggregate, nothing about any individual. This endpoint
returns per-visitor JA4 fingerprints *and* the decision each one got,
which makes it two different problems at once: it discloses visitor
data, and it is an **evasion oracle** — a bot can query it to learn
whether its own fingerprint is being flagged and iterate until it
isn't. That second one is the reason this is not merely "add auth
later"; an open version of this endpoint actively helps the attacker
hakaishield exists to stop. So: token required, `crypto/subtle`
comparison, an empty configured token denies everyone (a deployment
that forgot to set one fails closed), and deliberately **no wildcard
CORS**, unlike `/stats`.

**Why off by default rather than on with a generated token.** A
generated token has to be printed or stored somewhere, and the
operator who never wanted the endpoint now has a live one they aren't
watching. Unset flag → unmounted route → no attack surface at all.

**Why one `checks` table in `score.go`.** `Score` and the signal names
shown in the evidence were originally going to be two separate lists.
That is a bug with a fuse on it: add a signal to the scorer, forget
the name list, and the trail now confidently explains a block with the
wrong reason. `CLAUDE.md` Section 23b's "a wrong fingerprint is worse
than a missing one" applies exactly here — a wrong *explanation* is
worse than none, because an ops engineer will act on it. One table,
read by both, removes the drift class instead of documenting it.

**Why `challenge_solved` is recorded as its own reason.** A request
let through on a solved-challenge cookie skips scoring entirely.
Recording it as a score-0 allow would have the trail assert the
visitor looked clean, when in fact they may have carried both bad
signals and were let through on proof they'd already given. The trail
is only worth having if it never says something untrue.

**Why in-memory, not the planned store.** Same reasoning already
logged for `Stats` and the challenge secret: a durable store is its
own roadmap item, and shipping a half-wired Postgres dependency to
get history would be worse than an honest 1000-entry window. Both
caps exist from day one rather than "later" because this runs against
adversarial traffic (`CLAUDE.md` Section 9).

**Alternatives considered:**
- *Write evidence to the normal log stream instead of a queryable
  endpoint.* Rejected: it puts us back where CrowdSec already is —
  parsing logs — and the whole point of item 12a is that a client's
  ops engineer can answer "why was my customer blocked?" without a log
  pipeline.
- *Store the request path and IP too.* Rejected for now under Section
  18 (only what the decision used). It is the obvious next gap — see
  the honest-gaps note in `PROGRESS.md` — but widening visitor data
  collection is a decision to make deliberately, not a convenience.
- *Reuse `/stats`'s open CORS for dashboard convenience.* Rejected on
  the spot; see above.

**Revisit when:** the dashboard needs to read this endpoint
cross-origin (needs a real allowed-origin config, not a wildcard), or
per-client configuration (item 11) arrives and the size/retention caps
should become per-deployment settings.

---

## Positioning: self-hostable agent governance, not "cheap DataDome" — 2026-09-16

**Decision:** hakaishield stops describing itself as an *affordable
alternative to Akamai/DataDome* and starts describing itself as
**the inline, self-hostable layer that decides which automated
clients reach a site, and proves why.** `ROADMAP.md`'s intro and
`AGENT.md`'s mission line are updated to match.

**Why (this is the important part):** a market scan on 2026-09-16
(numbers in `RESEARCH.md`) found the "affordable" slot is not an
opening — it is the most crowded part of the market, and its floor is
$0:

- Below us: CrowdSec, SafeLine, Coraza — free and self-hosted.
  Cloudflare's free tier. Prosopo at ~$39/mo.
- Above us: DataDome/HUMAN/Kasada at ~$1K–50K/mo.

Pitching "cheaper bot detection" invites exactly one reply: *"CrowdSec
is free."* We cannot win a price argument against zero. Worse, the
old framing made **price** our differentiator, which meant every
roadmap item was implicitly judged as "does a big vendor have this?"
— the precise habit `CLAUDE.md` Section 14 exists to stop.

**What we actually have that the free tier does not:** CrowdSec and
friends parse **logs** — they react to an IP *after* it has misbehaved
somewhere. hakaishield reads the **live TLS ClientHello** and scores
the first request, with no prior sighting of that client. No
self-hostable product does inline JA4 fingerprint scoring today. That
is a narrow but real moat, and it is worth money to two buyers the
free tier cannot serve:

1. Teams under GDPR/DPDP-style constraints who **cannot** route
   traffic through a foreign SaaS. Their alternative to us is not
   DataDome — it is nothing.
2. Sites where bot traffic is a **direct revenue leak**, not an
   annoyance: pricing-sensitive e-commerce, ticketing/booking
   inventory, job boards and classifieds, usage-billed APIs. For
   them this is a margin tool, not a security line item — which is
   the difference between a $200/mo yes and a "we'll think about it."

**The second half of the decision — build for agents, not just bots.**
Forrester renamed the category in Q2 2026 to *Bot and Agent Trust
Management*. Cloudflare shipped pay-per-crawl; RSL and Web Bot Auth
appeared. The buyer's question moved from "is this a bot?" to "which
agent is this, is it allowed, and can I prove what I decided?"
Mid-market sites get exactly two answers today: allow or block.
Nobody sells them policy. Our JA4 + UA-consistency signals already
answer "which client is this, really" — that is the raw material for
governance, and it is a feature we can ship, not a market we have to
create.

**Roadmap consequences** (all reuse existing machinery, per Section
14 — no new detection layer):
- Item 11b: verified agent policy — per-agent allow / rate-limit /
  deceive rules, so "GPTBot is fine, a scraper wearing its User-Agent
  is not" is expressible. Extends item 11, not a new package.
- Item 12a: decision evidence trail — per-request record of score and
  which signals fired. Blocking is commodity; *proving why* is not,
  and it is what settles a false-positive dispute (Section 8).
- Item 18: shadow mode — score and report without enforcing. This is
  primarily a **sales and trust** instrument, and it is listed as a
  roadmap item so it does not get treated as optional polish.
- Item 17 (pricing) now has a stated anchor: ~$200/mo, justified by
  self-hostability and governance, never by a discount.

**Alternatives considered and rejected:**
- *Compete on price (~$49/mo, undercut Prosopo).* Rejected: the floor
  is $0 and we would be arguing against free software with a worse
  brand. Also caps revenue below what one support conversation costs
  a solo maintainer.
- *Go enterprise, chase DataDome's buyer.* Rejected: that buyer
  requires SOC2, a sales team, 24/7 support and a mobile SDK. A solo
  maintainer cannot serve it, and pretending otherwise is how the
  product dies mid-deal.
- *Build pay-per-crawl / HTTP 402 billing now.* Rejected: no major AI
  lab has adopted pay-per-crawl or Web Bot Auth, so we would ship a
  toll booth nobody pays at. Identify-and-govern works today; charging
  does not.
- *Ship an open-source community edition for reach.* Rejected on the
  spot — `CLAUDE.md` Section 1: this is commercial software. Noted
  here only so a future session does not re-propose it as a growth
  idea.

**What this does NOT change:** the detection architecture. Multi-layer
scoring (Section 6), the fail-open/fail-closed choice, latency budget,
and the false-positive bar all stand exactly as they are. This is a
decision about *who we sell to and what we say*, and about which
roadmap items earn priority — not a rewrite of the product.

**Honest risk:** all of this is desk research. Zero paying clients
have confirmed any of it, and the $200/mo anchor is reasoned, not
observed. The two buyer segments above are hypotheses. Treat the
first real client conversation as the test — and if it contradicts
this entry, write the correction here rather than quietly drifting.

**Revisit when:** a real client's traffic and budget contradict the
segments above, or a major AI lab adopts Web Bot Auth / pay-per-crawl
(which would move billing from "no" to "worth scoping").

---

## Automation probe lives inside the JS challenge, not injected site-wide — 2026-09-16
**Decision:** ROADMAP item 6's "client-side automation-tool probe"
runs inside the existing JS challenge page (`proxy/challenge.go`),
checked only for visitors who reach that page — not injected into
every proxied origin response.
**Why:** injecting a JS snippet into arbitrary origin HTML is a real,
separate feature (parse/rewrite HTML, handle charset and compressed
responses, interact correctly with the origin's own CSP) that this
codebase has no infrastructure for and no roadmap decision to build.
The challenge page is the one place hakaishield already controls its
own JS execution in a visitor's browser — reusing it is both smaller
and immediately testable with the existing harness, matching
`CLAUDE.md` Section 3/13's bias against building new machinery when
existing machinery already does the job.
**Alternatives considered:** a full response-body JS-injection
middleware — rejected as a large, undecided feature of its own; would
also need to handle every origin's existing CSP header correctly to
avoid breaking real sites, which is a meaningful security surface on
its own.
**Coverage trade-off, accepted:** the probe only ever runs for traffic
that already reached the challenge (score ≥ 50) — a request scored
`DecisionAllow` never executes it. This narrows item 6's literal
"flags obvious automation frameworks in-browser" (which reads as
site-wide) to "flags them for traffic already under suspicion,"
similar in spirit to item 3's narrowing of "verify real client family"
to what's provable without a maintained database.
**Revisit when:** HTML-injection into origin responses becomes an
actual roadmap item for other reasons (e.g. behavioral scoring, item
7, likely needs this too) — at that point item 6 should extend to
run everywhere, not just on the challenge page.

---

## Added 2 competitor-gap items to roadmap, rejected the rest — 2026-09-15
**Decision:** after a competitor scan (DataDome, Akamai, Cloudflare,
Kasada, Arkose, HUMAN/PerimeterX — see `RESEARCH.md`), added item 9a
(API-aware endpoint rules) and item 11a (deception/decoy response) to
`ROADMAP.md`. Both reuse existing machinery (item 9/11's config, the
scoring engine's decision output) rather than adding a new detection
layer or package.
**Why these two:** low implementation cost, no new architecture, and
genuine differentiation — deception in particular is under-offered
even by big vendors. Both fit `CLAUDE.md` Section 14 (real gap, not
"vendor X has it so we should too").
**Rejected:** persistent cross-session device fingerprinting (needs a
standing ML similarity model + cross-session storage — an infra
project of its own), native mobile SDK (separate codebase/maintenance
surface, off hakaishield's web-proxy shape), shared cross-customer
threat intel (needs a consent/data-sharing framework first, or it's a
Section 18 data-overreach problem), WASM deep browser-engine
fingerprinting (research-heavy, no observed client need yet). Full
reasoning for each in `RESEARCH.md`.
**Risk accepted knowingly:** item 11a (deception) is scoped narrow on
purpose — origin app owns the fake data, hakaishield only signals the
decision, and it's gated to only the highest-confidence score band
(above the block threshold) because a wrongly-deceived real customer
sees *wrong data as real*, which is a worse failure mode than a
false-positive block. Not to be turned on for any client until the
scoring engine (item 5) and dashboard (item 12) both exist to measure
it separately from block/challenge false positives.
**Revisit when:** item 5 (scoring engine) ships — that's the actual
blocker for both 9a and 11a, since neither can be built before there's
a decision layer to plug into.

---

## Scoring thresholds: additive weights, no single signal blocks alone — 2026-09-15
**Decision:** `Score()` gives the JA4-fragmentation signal and the
UA-mismatch signal 50 points each, additive. `Decide()` blocks at 100,
challenges at 50, allows below that. A single signal firing alone can
only ever reach 50 (challenge), never 100 (block) — block requires
both.
**Why:** `CLAUDE.md` Section 6 is explicit that no single signal may
be the only thing between allow and block. Fragmentation and UA
mismatch are correlated (a fragmented handshake is one of UAMismatch's
own inputs) but not the same finding: fragmentation is a
fingerprint-layer anomaly true regardless of what the client claims to
be; UA mismatch is a consistency-layer *lie*, which needs a browser
claim to exist at all. Scoring them as two separate, additive signals
— rather than collapsing them into one — means a bot that fragments
its handshake but doesn't claim to be a browser (most naive scripts:
curl, requests, a bare `net/http` client) gets challenged, not
blocked, matching the product's own "don't hard-block on a single
imperfect signal" principle even when that signal is strong.
**Alternatives considered:** a single combined "TLS anomaly" signal
worth 100 whenever fragmentation OR UA mismatch fires — rejected,
that's exactly the single-point-of-failure shape Section 6 forbids,
and it would immediately hard-block anything that merely fragments
without ever claiming to be a browser (an honest scripted client that
happens to fragment for unrelated reasons — a censorship-circumvention
tool, per `docs/RESEARCH.md` — shouldn't get the harshest outcome for
one imperfect signal).
**False-positive risk, accepted:** a claimed-browser client on an old
TLS stack or a fragmenting network path hits the challenge threshold
alone (50) — costs a real user one extra JS-challenge page load, not a
block. A corporate proxy that both intercepts TLS *and* somehow
triggers fragmentation would hit the block threshold — considered
unlikely enough to accept for v1, revisit if real traffic shows
otherwise.
**Revisit when:** item 6 (client-side automation probe) adds a third
signal — weights need rebalancing so three signals firing doesn't make
the threshold model meaningless (e.g. any two of three always
blocking regardless of severity). Also revisit once real client
traffic exists to tune against instead of reasoned guesses.

---

## agentchat: removed the MCP server, kept plain file + manual relay — 2026-09-15
**Decision:** built, then deleted the same day, `agentchat/mcp_server.py`
— an MCP server exposing `send_message`/`get_messages`/
`wait_for_message` tools over `chat.jsonl` so Claude Code and
Antigravity could message each other without hand-editing a file.
Reverted to the original design: a plain `chat.jsonl` log plus
`agentchat/web.py` (stdlib-only HTTP bridge) for the project owner to
type into directly. Coordination between the two agents is manual —
the owner tells each one to check the log.
**Why:** two real problems, found by actually building and testing it,
not guessed in advance:
1. **Shared code file caused a live edit war.** Both agents were told
   they could edit `mcp_server.py`; when both did, each fix silently
   overwrote the other's — twice, in the same session, confirmed by
   the file breaking (`ImportError`/`AttributeError` from a
   low-level-API rewrite that removed the decorator the working
   version needed) right after being fixed.
2. **It never actually solved the problem it was built for.** MCP
   tool calls only execute when an agent's host chooses to call them —
   there is no push. Verified two ways: (a) Antigravity's own
   background script "just prints to stdout, which doesn't wake me
   up," confirmed live in `chat.jsonl`; (b) web research (see below)
   confirms this isn't specific to our setup — MCP's 2026-07-28 spec
   added `notifications/resources/updated` subscriptions, but even
   working implementations "do not guarantee durable event delivery,
   wake a model, or start an agent turn," and adoption is near zero
   because of it.
**Alternatives considered:** giving each agent its own separate MCP
server file (no shared code, avoids problem 1) — rejected once problem
2 was confirmed, since it wouldn't have fixed the actual goal
(automatic notification), only the file-collision symptom.
**What would actually work, and why it's not built here:** a trigger
inside each agent's own host application (a file-watcher, a scheduled
job, a webhook-driven automation — the pattern real products like
Cursor's "Automations" use). This has to be configured per-agent, in
that agent's own tool/IDE settings — Claude Code cannot build or
enable this for Antigravity, and vice versa. If the project owner
wants real automatic wake-up, it needs enabling on each side
separately, not more code in `agentchat/`.
**Revisit when:** either agent's host natively supports a
"wake on event" mechanism the owner can point at `chat.jsonl`, or the
MCP ecosystem's server-initiated-events work (webhooks/channels, on
its own 2026 roadmap per the research below) actually ships and gets
adopted — not before.
**Sources checked:** [Using MCP Push Notifications in AI Agents](https://gelembjuk.com/blog/post/using-mcp-push-notifications-in-ai-agents/), [MCP Has Notifications. So Why Can't Your Agent Watch Your Inbox?](https://ankitmundada.medium.com/mcp-has-notifications-so-why-cant-your-agent-watch-your-inbox-bb688fde7ac5), [anthropics/claude-code#36665](https://github.com/anthropics/claude-code/issues/36665), [MCP Roadmap 2026](https://www.explainx.ai/blog/the-new-mcp-roadmap-2026).

---

## JS challenge: sha256+canvas proof, not a math-only puzzle — 2026-09-15
**Decision:** `proxy.Challenge` requires two things from the client's
JS, not just one: the SHA-256 of a server-issued nonce, and a
`canvas.toDataURL()` render. It does not stop at the math step alone.
**Why:** a sha256-of-a-nonce puzzle by itself is not actually
JS-specific — any language can compute a SHA-256 in one line, so a
scripted client that bothers to parse the challenge HTML and hash the
nonce defeats it without ever running JS, which would fail the
ROADMAP's own claim ("a plain HTTP client without a JS engine fails
immediately"). Requiring an actual `canvas.toDataURL()` output raises
the real bar toward needing a genuine browser rendering engine, which
a bare HTTP client cannot produce without one.
**Alternatives considered:** math-only challenge (simpler, matches a
literal reading of "math + timing") — rejected once traced through:
it only filters clients too lazy to read the page, not clients without
a JS engine, which is the actual stated threat.
**Known limitation, accepted:** the canvas proof is a client-reported
string (`validCanvasProof` checks its shape — prefix + minimum
length — not its actual pixel content). A bot author who studies
hakaishield specifically can fake a plausible-looking string without
ever rendering anything. Verifying real pixel content server-side
needs either a headless-render comparison service or a much larger
research effort — out of MVP scope, same class of accepted limitation
as item 3's JA4-database gap. This is why the challenge raises cost
for a naive-to-intermediate bot; it does not claim to stop a bot built
specifically against hakaishield.
**Revisit when:** real traffic data shows the canvas check is either
pulling its weight or not worth the false-positive risk on browsers
with canvas disabled (privacy tools, some accessibility setups).

---

## JS challenge secret: random, in-process, not shared — 2026-09-15
**Decision:** `NewChallenge()` generates a random 32-byte HMAC secret
at process start, kept in memory only. No config flag, no persisted
key.
**Why:** a hardcoded secret in source would let anyone who reads the
code forge challenge tokens and passed-cookies — worse than no signing
at all. A random per-process secret closes that immediately, at the
cost of invalidating outstanding challenges on restart or across
multiple instances. Since there's only one process in the current
architecture (`docs/ARCHITECTURE.md` — "No Kubernetes, no
microservices for v1"), that cost is real but currently free.
**Alternatives considered:** a secret passed via flag/env var — not
implemented yet; would let a restart survive without invalidating
outstanding cookies, but there's no config-loading mechanism in the
codebase yet to hang it off, and the current architecture is single-
instance so the gap has no observable effect yet.
**Revisit when:** hakaishield runs more than one process (needs a
shared secret — the planned Redis store, `docs/ARCHITECTURE.md`, is
the natural place) or when the passed-cookie's 30-minute lifetime
surviving a restart becomes something a real client asks for.

---

## UA-consistency check: structural heuristics, not a browser-fingerprint database — 2026-09-14
**Decision:** `UAMismatch` catches a UA claiming a browser while the
TLS handshake shows TLS 1.0/1.1 or the `JA4Unreadable` fragmentation
signal — not a full "does this JA4 really belong to Chrome 120"
classification.
**Why:** the strict version of ROADMAP item 3 ("real client family")
needs a maintained table mapping JA4 hashes to real browser versions.
Real anti-bot vendors run that as a standing research/maintenance
cost, updated as browsers ship. Building and maintaining that
database is a project of its own, not a one-line addition, and
`CLAUDE.md` Section 15 says research before building — not fake a
version of something that needs real ongoing data. The two conditions
implemented instead are provably true of every current real browser,
need no external data to verify, and reuse the `JA4Unreadable` signal
already established for fragmentation.
**Alternatives considered:** a hardcoded list of a few known-good JA4
hashes for major browsers — rejected: browsers update their TLS stack
often enough that the list would go stale within months and start
producing false positives on real users (`CLAUDE.md` Section 8), with
no mechanism in this repo to keep it current.
**False-positive risk, accepted:** a corporate TLS-inspecting proxy or
an unusually old/locked-down real browser could legitimately negotiate
TLS 1.0/1.1 and get flagged. This is why the result is a signal for
future scoring (item 5), never a block on its own (`CLAUDE.md`
Section 6).
**Revisit when:** item 5's scoring engine exists and real traffic data
shows whether a maintained JA4-to-browser database is worth the
ongoing cost, or when HTTP/2 fingerprinting is built (adds another
structural signal of the same no-database kind).

---

## hakaishield is closed-source commercial software, not open source — 2026-09-14
**Decision:** hakaishield is proprietary. `README.md` previously said
`License: MIT`, which was wrong and is corrected to "Proprietary — All
Rights Reserved." There is no LICENSE file granting copy/modify/
redistribute rights, and none should be added.
**Why:** the project owner is building this to sell as a paid product
(a SaaS / self-hosted commercial license), not to give away. An MIT
license would have let anyone legally clone, rebrand, and resell it —
directly undermining the reason it's being built. This was a docs
mistake carried over from an earlier session's generic project
scaffolding, not a considered choice, and it was live in the repo
until caught here.
**Alternatives considered:** open-core (core engine open, paid
features closed) — not rejected outright, just not decided; revisit
if the owner ever wants community contributions or wider adoption as
a growth strategy. Until then, default to fully closed.
**What doesn't change:** hakaishield still *uses* open-source
libraries internally (`fingerproxy`, `BotD` — see the "assemble
proven open-source pieces" entry below). Depending on open-source
components is normal for commercial software and is unrelated to
whether hakaishield's own code is licensed for redistribution.
**Revisit when:** the owner explicitly decides on a monetization/
distribution model (self-hosted license sales, managed SaaS, open-
core) — that decision picks the real license text, ideally with a
lawyer's input before any code ships to a paying customer.

---

## Report an unreadable handshake as a signal, not as "no fingerprint" — 2026-09-14
**Decision:** when a connection is TLS but we cannot read its
ClientHello, `JA4FromContext` returns `JA4Unreadable` ("unreadable"),
not `""`. `""` now means only "this wasn't a TLS connection".
**Why:** a client can split its ClientHello across two TLS records.
The handshake succeeds, but our capture (and `fingerproxy`'s, which
reads one record) sees a fragment, so JA4 parsing fails. Reproduced
locally: the request sailed through with an empty fingerprint, which
looked exactly like an ordinary unfingerprinted request. That made a
one-line bot change into a *silent* bypass of the product's core
detection. Making the two states distinguishable doesn't stop the
evasion, but it stops it being invisible — and a real browser never
fragments this way, so "TLS but unreadable" is itself a bot signal.
**Alternatives considered:** blocking on an unreadable handshake —
rejected: some legitimate stacks and censorship-circumvention clients
fragment too, and `CLAUDE.md` Section 6 says no single signal decides,
Section 8 says a false positive is worse than a miss. This is an input
for scoring, not a verdict. Also considered: writing record-layer
reassembly ourselves — rejected for now, `DECISIONS.md` above says we
don't hand-write TLS parsing; see `RESEARCH.md` for the open item.
**Revisit when:** the scoring engine (`ROADMAP.md` item 5) exists and
can weight this, or when reassembly lands upstream in fingerproxy.

---

## Strip every client-IP header, not just the X-Forwarded family — 2026-09-14
**Decision:** the proxy deletes `X-Real-IP`, `True-Client-IP`,
`CF-Connecting-IP`, `X-Client-IP`, `Fastly-Client-IP` and
`X-Cluster-Client-IP` from the outbound request, then sets
`X-Real-IP` itself from the real connection address.
**Why:** net/http's `Rewrite` strips only `Forwarded` and
`X-Forwarded-*`. nginx, Rails, Laravel, Cloudflare and Fastly stacks
routinely read the others, so a visitor could still choose the IP the
origin logs, allowlists or rate-limits — the same spoof we closed for
`X-Forwarded-For`, through a different door.
**Alternatives considered:** stripping only `X-Real-IP` (the most
common) — rejected, each remaining header is a full bypass on some
origin stack, and they cost one line each.
**Revisit when:** hakaishield runs behind a trusted CDN that legitimately
sets one of these; that needs an explicit trusted-upstream setting,
never blanket trust.

---

## Let net/http own the TLS handshake; delete our hand-rolled version — 2026-09-14
**Decision:** `proxy.NewCaptureListener` returns a real `*tls.Conn`
from `Accept` and does **not** handshake it. net/http then runs the
handshake itself. This deleted our handshake timeout, accept-retry
backoff, per-connection panic recovery, goroutine-per-connection and
its semaphore, and the `hack.ChannelListener` handoff — roughly 60
lines.
**Why:** reading `$GOROOT/src/net/http/server.go` showed all four
already exist there: `Server.tlsHandshakeTimeout()` (derived from
`ReadHeaderTimeout`), the `tempDelay` accept-retry loop,
`conn.serve()`'s `defer recover()`, and its connection handling. The
stdlib versions are better than ours were — they log handshake
failures and reply properly to a plain-HTTP client hitting the TLS
port, both of which our version silently skipped. The one thing we
genuinely need (the raw ClientHello) is kept by wrapping the
connection *underneath* `tls.Server`, which costs one line.
**Alternatives considered:** keeping our own accept loop for "control"
— rejected, it was control over code we had reimplemented worse.
**Cost checked, not assumed:** the fingerprint is now derived per
request rather than once per connection. Benchmarked at **14.3µs**
against a ~2ms budget (`ARCHITECTURE.md`), so no cache — adding one
would be optimising something that costs 0.7% of its budget.
**Trade-off accepted:** `ReadHeaderTimeout` in `cmd/hakaishield` is now
load-bearing for the TLS handshake too, not just headers. Noted in a
comment there so nobody removes it as "just a header thing".
**Revisit when:** HTTP/2 support is added — returning a real
`*tls.Conn` is also what makes stdlib h2 negotiation possible, so
that work got cheaper, not harder.

---

## Use ReverseProxy.Rewrite, not Director — 2026-09-14
**Decision:** build the reverse proxy with `httputil.ReverseProxy{
Rewrite: ...}` instead of `NewSingleHostReverseProxy` + `Director`.
**Why:** net/http strips a visitor's `Forwarded` and `X-Forwarded-*`
headers before calling `Rewrite`, and does not before calling
`Director` (verified in `$GOROOT/src/net/http/httputil/reverseproxy.go`).
With `Director`, a visitor sending `X-Forwarded-For: 1.2.3.4` had it
forwarded to the origin as `"1.2.3.4, <real ip>"` — and most code
reads the first entry, i.e. the attacker's chosen value. Per-IP rate
limiting and geo checks (`ROADMAP.md` items 8 and 9) would have been
bypassable from day one. `Rewrite` + `SetXForwarded()` sends the real
client IP only. `Rewrite` also drops unparsable query parameters,
which closes a proxy/origin request-smuggling gap.
**Behaviour kept deliberately:** `SetURL` would rewrite the `Host`
header to the origin's host; we restore the visitor's `Host` (`r.Out
.Host = r.In.Host`), since the origin serves the client's own domain.
**Behaviour changed deliberately:** an inbound `X-Forwarded-For` is
now replaced rather than appended to. That is the secure default and
there are no deployments yet to break.
**Revisit when:** hakaishield is ever deployed *behind* another trusted
proxy (a CDN), where the inbound `X-Forwarded-For` is legitimate — at
that point it needs an explicit "trusted upstream" setting, never a
blanket trust of the header.

---

## Import only fingerproxy's `ja4` package, not the whole library — 2026-09-14
**Decision:** depend on `github.com/wi1dcard/fingerproxy`, but import
only its `pkg/ja4` package (JA4 hash computation, stdlib + `utls`
only) for now — not `pkg/fingerprint` (pulls in Prometheus metrics)
or `pkg/proxyserver`/`pkg/ja3` (pull in `gopacket`/`dreadl0ck/tlsx`).
**Why:** we don't need JA3, HTTP2-frame fingerprinting, or metrics yet
— only JA4. Go compiles per-package, so importing the narrower
package keeps Prometheus and gopacket out of the actual binary
entirely (verified with `go list -deps`), matching `CLAUDE.md` Section
3 (no unnecessary dependencies) and Section 14 (no code "in case it's
needed later").
**Alternatives considered:** importing `fingerproxy.Run()`'s full
opinionated server (`pkg/proxyserver`) directly instead of writing our
own capture wiring — rejected: it owns the entire accept loop and
HTTP/1.1-vs-HTTP/2 branching, which would mean replacing our own
`proxy.New` design instead of extending it. Revisit if TLS-capture
wiring turns out to need functionality we'd otherwise reimplement.
**Revisit when:** JA3 or HTTP/2 fingerprinting (also on `ROADMAP.md`)
is actually built — re-check whether pulling in `pkg/fingerprint` at
that point is cheaper than keeping our own thin wrapper.

---

## TLS capture listener: HTTP/1.1 only for now, no HTTP/2 — 2026-09-14
**Decision:** `proxy.NewCaptureListener` forces `NextProtos =
["http/1.1"]`, so browsers fall back to HTTP/1.1 against hakaishield
instead of using HTTP/2.
**Why:** capturing JA4 means terminating TLS and handshaking
ourselves instead of letting Go's `http.Server` do it, which breaks
the stdlib's automatic HTTP/2 upgrade (it only kicks in when the
accepted connection is literally a `*tls.Conn`, not our wrapped
type). Supporting HTTP/2 correctly means serving it ourselves
alongside HTTP/1.1 (the way `fingerproxy/pkg/proxyserver` does) —
real, separate work, not a one-line fix.
**Alternatives considered:** hand-rolling the HTTP/2 branch in this
same pass — rejected: `ROADMAP.md` item 2 already lists "JA4" and
"HTTP/2 fingerprint" as two separate things, and JA4 alone already
catches most naive scripted clients (the ROADMAP's own claim). Ship
one working half instead of both halves half-working.
**Revisit when:** ROADMAP's HTTP/2 fingerprint item is picked up.
**RESOLVED 2026-09-25:** h2 is now offered and fingerprinted. See
"HTTP/2 greeting fingerprinting: evidence-only, weight 0" below.

## Cap concurrent TLS handshakes at 1000 — 2026-09-14
**Decision:** `proxy.NewCaptureListener` runs each connection's TLS
handshake in its own goroutine, but only allows 1000 to run at once
(a buffered channel used as a semaphore); beyond that, new
connections wait for a slot before their handshake starts.
**Why:** `CLAUDE.md` Section 9 requires every worker pool to have a
bounded size — without a cap, a flood of connections (accidental or
a deliberate flood attack) could spawn unlimited goroutines and take
the process down, which would fail the client's site closed instead
of open.
**Alternatives considered:** no cap (simplest, but violates Section
9); a smaller/larger number — 1000 is a reasonable starting guess for
a single small-to-mid deployment, not measured against real traffic.
**Revisit when:** ROADMAP item 16 (soak testing) gives real numbers
to tune this against.

---

## Format for new entries

```text
## <short title> — <date>
Decision:
Why:
Alternatives considered / rejected:
Revisit when:
```

---

## Assemble proven open-source pieces, don't reinvent TLS/fingerprint parsing — 2026-09-14
**Decision:** use `fingerproxy` for JA3/JA4/HTTP2 fingerprinting and a
`BotD`-style approach for the client-side automation probe, instead of
writing TLS ClientHello parsing or automation-detection heuristics
from scratch.
**Why:** these are hard, well-solved problems with mature open-source
implementations already (see `docs/RESEARCH.md`). Original engineering
effort should go into the part that's actually the product: the
scoring/decision layer and the deployment experience — not
re-deriving TLS fingerprinting from the spec.
**Alternatives considered:** building fingerprinting in-house for
full control — rejected, high effort for something already solved,
delays the actual differentiator.
**Revisit when:** a specific limitation in `fingerproxy`/`BotD` blocks
a real requirement and can't be worked around.

---

## Large-scale residential proxy networks: fingerprint/pattern-based, not IP-based — 2026-09-14
**Decision:** don't attempt IP-reputation-based defense against
large commercial residential-proxy traffic (e.g. Bright Data-class
networks). Rely on cross-IP fingerprint correlation and
aggregate-rate detection instead, and treat this as a cost-raising
goal, not a 100%-block goal.
**Why:** the IPs in these networks are real residential connections —
IP reputation has nothing to flag. The client software driving the
proxy still has a consistent fingerprint/behavior regardless of which
IP it tunnels through; that's the real signal. See `docs/RESEARCH.md`
for the full reasoning.
**Alternatives considered:** none viable — even Akamai/DataDome don't
fully stop this tier; promising full protection here would be a false
claim to clients.
**Revisit when:** P0/P1 fingerprint+rate infrastructure exists and
real client traffic shows this tier is a priority (ties to
`ROADMAP.md` item 9).

---

## Product is one deployable service, not a library — 2026-09-14
**Decision:** hakaishield ships as one product (reverse proxy +
dashboard) a client deploys directly. It is not published/marketed as
a reusable Go library for other developers to import.
**Why:** the actual customer is a non-technical site owner, not a Go
developer. A "library + product" framing added an audience and
maintenance burden nobody asked for.
**Alternatives considered:** core packages exposed as an importable
library in addition to the proxy — rejected as unnecessary scope for
v1 (see `CLAUDE.md` Section 3, no unnecessary abstraction).
**Revisit when:** a real client explicitly asks to embed detection
logic directly in their own app instead of running the proxy.

---

## Go stack, same as goScraper — 2026-09-14
**Decision:** hakaishield is written in Go.
**Why:** team already knows Go from goScraper; this service sits in
every request's path, so it needs high concurrency and low memory —
Go fits (same reason Cloudflare/Caddy/Traefik are Go/Rust, not
Python/Node, for this kind of workload).
**Alternatives considered:** Node.js (faster to prototype, weaker at
sustained high-concurrency proxy workloads); Rust (better raw
performance, slower team ramp-up, not worth it for v1).
**Revisit when:** never, unless a specific measured bottleneck proves
Go is the wrong tool for a specific component.

---

## Multi-signal scoring, no single-check verdicts — 2026-09-14
**Decision:** every detection layer (TLS/JA4, behavioral, session
consistency, rate pattern) contributes to a combined risk score.
No single signal alone can allow or block a request.
**Why:** advanced automation tools (patched browser-automation
frameworks, TLS-impersonating HTTP clients) are specifically built to
defeat one or two common checks (`navigator.webdriver`, basic CDP
detection). A single boolean check is a single point of failure that
these tools are already designed around.
**Alternatives considered:** simple allowlist/blocklist checks only —
rejected, defeated trivially by the exact tools we studied
(patched browser automation, TLS-impersonation clients).
**Revisit when:** never as a principle; specific thresholds/weights
will be tuned as real traffic data comes in.

---

## MVP scope = naive-to-intermediate bots, not nation-state-grade evasion — 2026-09-14
**Decision:** P0 (items 1-6 in `ROADMAP.md`) targets plain scripted
clients, unconfigured HTTP libraries, and basic headless browsers.
Large-scale residential-proxy traffic (e.g. big commercial proxy
networks) is explicitly P1/P2, not MVP.
**Why:** that class of traffic is the majority of what mid-size
clients actually get hit by today, and it's realistically achievable
with fingerprint + JS-challenge alone. Defeating large residential-
proxy-network traffic requires cross-IP fingerprint correlation and
aggregate rate analysis across huge IP pools — real capability, but a
P1/P2 problem, not something an MVP can or should promise.
**Alternatives considered:** trying to handle every adversary tier in
v1 — rejected, would delay shipping anything usable (see `CLAUDE.md`
Section 15, Solo/Small-Team Maintainer Mandate: ship something solid,
not something big and half-done).
**Revisit when:** P0 is live against real client traffic and proven;
then large-proxy-network correlation moves up the roadmap.

---

## Target market: mid-size companies priced out of enterprise bot management — 2026-09-14
**Decision:** target customer is a mid-size company (e-commerce,
ticketing, job portals, SaaS) that currently has weak/no bot
protection because Akamai/DataDome/PerimeterX pricing ($1,500-
$50,000+/month) is out of reach for them.
**Why:** this is a real, provable gap — the project owner's own
clients are already in this exact situation. Competing on feature
count against Akamai is not viable for a small team; competing on
price + fast deployment for an underserved segment is.
**Alternatives considered:** building a scraping/automation-evasion
tool instead (selling to bot operators, not against them) — rejected
outright: worse legal/ethical position, saturated market, and directly
conflicts with what the project owner's actual clients need protection
from. See `docs/AGENT.md` and `CLAUDE.md` Section 18 (Ethical/Legal
Boundary).
**Revisit when:** never as a market direction; pricing/tiering itself
is still TBD (`ROADMAP.md` item 17).

---

## Redis rate evidence fails open with a shared circuit — 2026-09-21
**Decision:** Redis-backed velocity and crawl signals share a one-second
circuit. The first Redis error opens it; while open, these optional signals
return no evidence without contacting Redis. After the cooldown, exactly one
request probes Redis. A successful probe closes the circuit, while a failed
probe starts a new cooldown. The Redis client also has command retries disabled
and permits one dial attempt for this request-path workload.
**Why:** this proxy is inline. A Redis network partition previously allowed up
to three separate timeout-bound calls during one score evaluation (IP velocity,
JA4 velocity, and crawl pattern), and automatic client retries could compound
the cost. Redis evidence is useful but must never turn a dependency outage into
a customer-site latency or availability outage. Fail-open can miss rate
evidence briefly; it cannot falsely block a legitimate visitor.
**Alternatives considered:** fail closed (rejected: it would take healthy
customer traffic down for a detector dependency); independent per-signal
breakers (rejected: a single request would still probe the same failed Redis
service multiple times); unbounded/background retry (rejected: amplification
and recovery thundering-herd risk).
**Revisit when:** production telemetry shows the one-second recovery cadence is
too aggressive or too slow, or Redis becomes a policy-enforcement dependency
whose failure posture needs an explicit customer-facing product decision.

---

## Client IP comes from X-Forwarded-For only behind configured proxy CIDRs — 2026-09-22
**Decision:** the default request identity remains the direct TCP peer. An
operator may set `-trusted-proxy-cidrs` to enable `X-Forwarded-For` only when
that peer belongs to one of those CIDRs. The resolver scans a valid forwarding
chain from the closest hop backwards, skipping trusted proxy hops and selecting
the first untrusted address. Invalid or absent forwarding data falls back to
the direct peer.
**Why:** forwarded headers are visitor-controlled on a direct connection, so
trusting them by default lets an attacker forge IP-based rate limits,
allowlists, and evidence. Hosted deployments behind a CDN/LB still need the
real visitor identity, including IPv6. Explicit CIDRs preserve both cases.
**Operational requirement:** listed proxies must overwrite or safely append
`X-Forwarded-For`; a proxy that forwards an attacker-supplied header unchanged
cannot provide a trustworthy client identity.
**Alternatives considered:** always trust the header (rejected: trivial IP
spoofing); never trust the header (rejected: loses visitor identity behind a
legitimate edge); trust by proxy hostname (rejected: DNS is not an
operator-controlled network trust boundary in the request path).
**Revisit when:** deployments require RFC 7239 `Forwarded` support or another
provider-specific, authenticated client-identity mechanism.

## Learned decision weights are a linear model over existing signals, not a new model — 2026-09-22

**Context.** "System One" decision models (TypeSafe Jev, 2026-09-15) return a
typed value with a probability and a confidence instead of text, and the
question came up of building our own for hakaishield. See `docs/RESEARCH.md`,
"System One models (TypeSafe Jev) and what they mean for scoring".

**Decision.** Add `pkg/decide`: logistic regression over the binary checks
`pkg/signals` already runs, trained offline from our own labelled traffic,
inferring in-process in Go. No transformer, no hosted inference call, no new
dependency.

**Why this and not the alternatives.**

- *A hosted System One API per request* — 70–500 ms added to a ~36 µs guard, a
  per-request external charge on traffic that is already our hosting cost, and
  a request-path dependency whose failure mode is fail-open (Sections 15, 19).
- *An MLX or Core ML port* — those target local Apple Silicon inference. The
  backend is Go on Linux.
- *A larger model of our own* — with around ten binary inputs, a linear model
  is the honest amount of capacity. More would fit noise, and would stop
  producing a per-feature contribution we can show a customer.

The existing scorer is already a linear model; its coefficients were just
chosen by hand. Learning them changes where the numbers come from, not the
architecture.

**Consequences and the limits accepted.**

- The model never decides anything today. It scores alongside the rules and its
  opinion is recorded next to the real decision (`-model`, shadow only). A
  model earns the right to enforce by being compared against the rules on real
  traffic first, not by passing tests.
- Its block bar is deliberately stricter than the rule scorer's: it will not
  return `DecisionBlock` unless at least two checks fired, however certain it
  is. The rule scorer permits a single-signal block for two verified-decisive
  checks (`ja4_blocklist`, `scripting_tool`); the model has no equivalent
  verification behind any one weight, so it gets the stricter rule
  (Sections 10, 14).
- **The blocker is verified labels, not inference code.** Challenge solves and
  honeypot hits are candidates, not ground truth; see the correction below.
  An independently reviewed customer report can supply a stronger label.
  Labelling with the
  current rule score would only teach the model to repeat the guesses it exists
  to improve on, and it would then score excellently against the very data that
  misled it. Until that labelling exists, `pkg/decide` is a working pipeline
  with nothing trustworthy to train on.
- Feature order is part of the model file. A model whose feature names or order
  do not match the running build is refused at load, because the fired-check
  vector is positional and a stale model would apply every weight to the wrong
  signal.

## Automatically collected labels are candidates, not ground truth — 2026-09-23

**Correction to the 2026-09-22 decision above.** A challenge solve does not
prove a human used a browser: proof-of-work is computable by any client, and
the canvas and automation fields are client supplied. A honeypot hit is strong
automation evidence but can also come from prefetch or accessibility software.
The per-identity cap limits volume; it does not authenticate either label.

**Decision.** Keep collecting source-tagged candidates for investigation, but
make `hakaishield-train -db-url` refuse them by default. An operator can use
`-allow-unverified-labels` for a shadow-only experiment, or supply independently
reviewed JSONL through `-in`. The proxy has no learned enforcement mode. Before
one is designed, obtain verified labels, correct selection bias, compare false
positives with the rules on held-out data, and resolve whether models are
tenant-specific or shared. The first honeypot hit now captures its own sample;
no follow-up request is required.

## Tenant policy revisions use append-only snapshots — 2026-09-23

**Decision.** Keep existing account-wide `mitigation_rules` in shadow and add
`tenant_policy_revisions` for a site's ordered policy. Each authenticated
write checks tenant ownership and an expected version while holding the
tenant row lock. Rollback appends a new shadow revision rather than mutating
history. A saved revision cannot enforce until the operator activates it
after measured shadow traffic. A cold policy cache falls back to the existing
signal scorer while a bounded background loader reads Postgres.

**Why.** The old owner-wide rules have no per-domain scope, version, priority,
or rollback. Activating them in place would let an experimental rule for one
site affect every site in the account. Immutable snapshots make the exact rule
order and action that produced a decision auditable. The proxy cannot afford a
Postgres lookup in the request path, and a missing policy must not turn a DB
outage into origin downtime.

**Limits.** The shadow activation gate currently reads node-local bounded
per-version aggregates that survive the evidence ring wrapping. This is
deliberately conservative after a restart or on a node
that has not served traffic, but does not provide a fleet-wide production
measurement. Durable aggregate telemetry and representative false-positive
review are required before multi-node activation. Existing challenge-solved
and honeypot branches keep their dedicated behavior; their policy skip reason
is explicit in evidence and those requests do not satisfy the gate.

---

## Challenge browser probes remain observational — 2026-09-24

**Decision:** WebGPU f16 absence, identical outputs from two different canvas
draws, absent pointer movement during a reported solve of at least one second,
and legacy automation globals are recorded only after a valid challenge solve.
They do not change the challenge outcome or request score. Browser telemetry is
client-reported and can be forged; these probes need labelled traffic and
false-positive review before enforcement.

**Why:** WebGPU f16 is an optional hardware/driver capability, a person may
leave a pointer motionless, privacy tools can alter canvas output, and some
embedded browsers expose legacy globals. The challenge page adds no forced
gesture or tracking on customer pages. WebGPU discovery has a 100 ms client
timeout and the canvas pair is sent only below 40 KiB. Per-tenant evidence
uses an already loaded exact host, avoiding a database call from verify.
On a cold second node or wildcard development tenant, shadow evidence may be
absent.
The proxy's initial shadow mode serves no challenge page, so it produces no
real visitor observations for these four challenge-only probes. Evaluation
requires a separately reviewed challenge cohort; server-side header and
traffic shadow signals continue to collect normally in shadow mode.

---

## Defer DNSBL for the low-cost pilot — 2026-09-24

**Decision:** launch the managed pilot without a DNSBL or external IP
reputation provider. The owner prioritised keeping initial costs low while
the domain and deployment are still being arranged.

**Why:** a live commercial feed adds recurring cost and licensing work;
a self-hosted list adds refresh and validation operations. Existing JA4,
velocity, challenge, and honeypot signals already provide the first pilot
baseline. Revisit after labelled traffic shows whether IP reputation would
add useful coverage without harming legitimate shared-IP visitors. Any later
provider choice must be reviewed for terms, cost, latency, and false positives.

---

## Model artifacts become v2 with provenance and a promotion gate — 2026-09-25

**Decision:** the saved model format moves to version 2 and every v2 artifact
carries a provenance record: the check-list hash the training masks used, a
hash of the labelled dataset, the training options, the held-out evaluation
summary, and an optional approver. `decide.Load` refuses any artifact whose
provenance contradicts its own feature list, and v1 files (no provenance) are
refused outright. The trainer stamps provenance automatically; `-approved-by`
records the human sign-off, and the proxy logs whether a loaded model is
approved at startup. Evaluation gained `SplitLeakageSafe` (a later-time,
identity-clean holdout), plan metrics (intervention recall, hard-block human
FPR, challenge burden, Brier), calibration buckets, and `RunPromotion`, which
holds the model unless it matches rule recall on the holdout at equal-or-lower
hard-block human harm. None of this changes enforcement: the model stays
shadow-only.

**Why:** the plan's `pkg/decide` section required label provenance, leakage-
safe evaluation and a promotion test before the model may influence anything.
A model file that cannot explain where it came from is operator input nobody
can audit; a promotion decision made on aggregate accuracy can hide human
harm. The gate encodes the plan's threshold language (recall at no more human
harm, minimum holdout size) in code so "it looked good" cannot promote a
model. Enforcing remains a separate, explicit, later step.

---

## Egress bytes and challenge burden are first-class tenant stats — 2026-09-25

**Decision:** per-tenant stats gained a saturating egress-bytes counter fed
from a measuring response writer wrapped once in `Guard.ServeHTTP`, plus
per-tenant challenge solve/fail counters fed from the challenge verify path
through a host-scoped recorder. Both surface through the existing
`/api/v1/dashboard/stats` response and the Overview page (egress formatted
MB/GB).

**Why:** the plan's measurement stage (P1) and its cloud-bill section name
egress bytes as the dominant hosting cost and challenge burden as the human-
impact number; neither existed per tenant. Counting at the response writer
means every response path (origin body, challenge page, block page) is
counted without each path remembering to. The counter saturates instead of
wrapping so hostile traffic cannot corrupt the cost number. Challenge
outcomes are recorded against an already-loaded cached tenant only, so the
unauthenticated verify route cannot create database lookups or touch another
tenant's numbers.

---

## Endpoint-class velocity buckets and a shared classifier — 2026-09-25

**Decision:** `velocity_spike` now counts per-IP requests in five endpoint-
class buckets — login 10, API 100, checkout 20, navigation 20, assets 300
per 1s window — with class computed from the normalized path plus HTTP
method. The classifier (`Classify`, `NormalizePath`) moved from `pkg/policy`
into `pkg/signals`; `pkg/policy` re-exports it. The verify path's admission
ceiling (64 concurrent, counted 503 shed) implements the plan's admission
budgets for the only unauthenticated endpoint with attacker-supplied work.

**Why:** credential stuffing is a login-shaped flood, scraping is a browse-
shaped flood, and no single global rate limits both without either missing
the stuffing or breaking real browsing. The classifier lives in `pkg/signals`
because `pkg/policy` already imports `pkg/signals` and the reverse edge would
be a cycle; delegation keeps one vocabulary so dashboard rules and rate
buckets cannot disagree about what a "login endpoint" is. Bucket limits are
deliberately conservative and need client-traffic calibration; the login
limit is strictest because no person posts ten logins a second while NAT
sharing makes strictness elsewhere risky.

---

## Tenant route drafts and bounded sample retention — 2026-09-25

**Decision:** Customers may label up to 64 canonical exact paths as login or
checkout in the existing owner-scoped, versioned tenant policy. Dashboard
Settings saves a shadow revision through the authenticated policy API.
The labels reuse `velocity_spike` only when that revision is activated; they
cannot weaken built-in sensitive routes or create a new model feature.
The default pilot tenant loads its owner from a matching active database row
at startup. Candidate training samples are pruned off the request path at
startup and hourly, with 1000-row SQL batches, at most 10 per run, and a
configurable 1-365 day retention period (default 30).

**Why:** A client may use `/account/signin` instead of `/login`; the global
classifier otherwise gives it the navigation limit. Versioned shadow drafts
preserve existing ownership, validation, history and activation gates. Exact
paths avoid broad prefixes that could capture unrelated traffic. Restricting
to sensitive classes prevents a customer typo from silently loosening their
login bucket. Retention cannot depend on an operator remembering to run SQL,
and a single unbounded delete could stall the pilot database.

**Limits:** The existing policy activation gate uses overall local traffic,
not per-route samples, so the operator must review each tagged route and
shared-IP impact before activation. The 30-day default must be confirmed
against the client's agreed retention period before label collection. The
current P2 plan offers no evidence that another uncalibrated browser signal
would improve bot recall; resource/session/route-sequence work waits for
real baselines or an approved collection surface.

---

## Crawler claims do not bypass path-behavior detection — 2026-09-25

**Decision:** Keep the existing `crawl_pattern` feature and threshold, but
count distinct page paths for User-Agents that declare themselves as a
crawler. Only supported crawler families whose source IP passes
reverse/forward DNS verification receive the crawler exemption. `Guard`
passes that server-verified fact into scoring; a visitor cannot supply it.

**Why:** Previously any `bot`, `spider` or `crawl` substring made
`claimsBrowser` false, which exempted the request from `crawl_pattern` even
when a spoofed Googlebot traversed more than 60 distinct pages per minute.
The same marker also hid the request from browser-consistency checks. The plan
requires unknown crawlers to receive ordinary evaluation and forbids UA-only
trust. This change improves the existing behavior signal without changing the
model feature list or adding another scored check.

**Limits and cost:** An unverified legitimate crawler may receive the existing
challenge after 60 distinct page paths in a 60-second window once enforcement
is enabled. Review the client's crawler traffic in shadow mode. Only declared
crawlers add the existing bounded Redis HyperLogLog work; Redis failures still
fail open, and DNS verification retains its concurrency budget.

## HTTP/2 greeting fingerprinting: evidence-only, weight 0 — 2026-09-28
**Decision:** The capture listener offers `h2` again. A negotiated h2
connection goes to `TLSNextProto["h2"]` (`core.WireHTTP2`), which reads
the client's connection preface (SETTINGS, connection WINDOW_UPDATE,
PRIORITY frames, first HEADERS pseudo-header order), renders the Akamai
format fingerprint, replays the consumed bytes, and serves the connection
with one shared `x/net/http2` server. The fingerprint is carried in the
connection context, recorded as `h2_tool_match` at **weight 0**, and
forwarded to the origin as `X-HakaiShield-HTTP2` (visitor copies stripped).
**Why:** the preface comes from the client's HTTP/2 stack, not
application code, so a script that fakes a browser UA and even its JA4
often still greets like Go, curl/nghttp2 or a Python stack. It also
restores h2 for real browsers, which were being held to HTTP/1.1.
**Why weight 0:** the seed list has two verified tool captures and no
browser corpus. It may not score until the Redis feed (`h2:tools`) is
populated and its false-positive rate is reviewed on pilot traffic.
**Why one shared server via `http2.ConfigureServer`:** a per-connection
`http2.Server` is not registered for graceful shutdown, so `srv.Shutdown`
waited out its full deadline on every open h2 connection
(`TestWireHTTP2GracefulShutdown`).
**Limits:** the preface read is bounded by `ReadHeaderTimeout` (10s),
16 frames and 16 KiB per frame. A client that has not sent the 24-byte
preface by then is dropped. One that sent the preface but no request yet
(a browser preconnect) keeps its connection, bounded by the h2 idle
timeout, and loses only its fingerprint
(`TestWireHTTP2ServesSlowPreconnect`). A parse failure likewise costs
only the fingerprint, never the connection.
**Alternatives considered:** `http.Server.Protocols`/`ServeTLS` —
rejected, they run their own TLS handshake and lose the ClientHello.
