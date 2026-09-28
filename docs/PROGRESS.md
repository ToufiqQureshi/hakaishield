# PROGRESS — what happened, in one line each

An index, not a journal. Git already stores what changed, and the commit
messages in this repository are written long and deliberately: what was
tried, what was rejected, what was verified and how. Repeating that here
made the file 3,700 lines, which every session then read instead of the
code.

So this file holds the **pointer**, and one thing git cannot give you.

## Format

```
### YYYY-MM-DD — <what it was>
`<commit>` — one or two lines on what changed and why.
Gotcha: <only when there is one — something that would bite the next
person and is not visible in the diff>
```

The gotcha line is the part worth keeping. `git log` will tell you that a
bit was masked out; it will not tell you that leaving it in makes the model
learn the label back.

## Where the detail actually lives

| Question | Look here |
|---|---|
| What exactly changed, and how was it verified? | `git show <commit>` |
| Why was it done, what was rejected? | `docs/DECISIONS.md` |
| What works, what's next? | `docs/STATUS.md` |
| How does it work? | `docs/ARCHITECTURE.md`, `docs/DEPLOYMENT.md` |
| Older long-form notes (deleted 2026-09-25) | `git show a7fe7ce:docs/PROGRESS_ARCHIVE.md` |

Doc names in older entries below may point at files merged into the five
above on 2026-09-25; `git show <commit>:<path>` still has them.

---

### 2026-09-25 — tenant route labels and sample retention

`b76a8583` — Owner-scoped exact login/checkout labels are saved as versioned
shadow drafts and feed the existing velocity buckets only after activation;
candidate samples now have bounded startup/hourly Postgres pruning.
Gotcha: the existing policy activation gate measures overall local traffic,
not traffic on each tagged route; review route and shared-IP impact first.
Mutation checks: route activation, SQL batch limit, retention cutoff and
dashboard authorization each made the relevant tests fail before restoration.

### 2026-09-25 — client-ready plan code slices (P1 measurement, P2 detection, decide gates)

`f730c14e` — Endpoint-class velocity buckets (login/API/checkout/nav/asset) via a
shared classifier moved into `pkg/signals`; per-tenant saturating egress-byte
counter + challenge solve/fail stats wired proxy→API→dashboard; bounded
concurrent admission ceiling (64) on the verify path with a counted 503;
model artifacts v2 with provenance (dataset hash, options, eval summary,
approver) and a leakage-safe evaluation + rules-vs-model promotion gate.
Model stays shadow-only.
Gotcha: splitting one backing array into train/holdout slices means growing
either side in place clobbers the other — `SplitLeakageSafe` rebuilds both
sides as fresh copies; the first version of this test passed vacuously
because no identity straddled the time boundary.

### 2026-09-22 — deployment setup, bot ladder, PROGRESS split
`14de934` — `deploy/` (Dockerfile, production compose, systemd unit, setup.sh
for a fresh box), `bot-testing/ladder/` (seven rungs, each adding one
capability), and this file became an index with the old content in
`PROGRESS_ARCHIVE.md`.
Gotcha: the container listens on **8443**, not 443 — the host publishes
443→8443 so the process needs neither root nor a capability. Debugging a
"port 443 refused" by looking at the process first will waste an hour.

### 2026-09-22 — where it deploys and what bandwidth costs
`c002cea` — `docs/DEPLOYMENT.md`: the TLS-termination constraint that rules out
every managed edge, AWS vs Hetzner egress maths, and ROADMAP item 27.
Gotcha: putting Cloudflare, Railway, an ALB or CloudFront in front does not
error. JA4 silently becomes a constant and detection quietly degrades.

### 2026-09-22 — what the product actually is
`ff55455` — `docs/WHAT_IS_BUILT.md`: plain-language inventory of what works,
what does not, and the measured numbers, for explaining it to someone.

### 2026-09-22 — scoring explained from zero
`35c1ef8` — `docs/SCORING_EXPLAINED.md`: the onboarding read. Worked examples
use real weights with outputs verified by running `Predict`.

### 2026-09-22 — label collection (ROADMAP item 26)
`14cc08c` — `pkg/labels` collects human labels from solved challenges and
automated ones from honeypot trips, behind `-collect-labels`. Seven mutation
checks.
Gotcha: `honeypot_trap` must be cleared from the vector of any sample it
labelled, or the model just learns the label back instead of learning from the
other eight checks.

### 2026-09-22 — how the model gets fed
`58f7ec1` — `docs/LEARNED_SCORING.md`: label sources, the traps, selection
bias, and the gate before a model may enforce.
Gotcha: a verified good-bot lookup looks like a free correct label and is not
usable — `guard.go` forwards crawlers before `Evaluate` runs, so no vector
exists, and training on them teaches the model to stop crawler-shaped traffic.

### 2026-09-22 — learned scoring weights (ROADMAP item 25)
`80be315` `27eb71d` `e716796` `2bff8f9` — `pkg/decide`: logistic regression over
the existing checks. 14ns and zero allocations per prediction, shadow only.
Lint went to 0 issues repo-wide, 13 of which pre-dated the branch.
Gotcha: the fired mask is **positional**. Reorder or rename a check and every
saved model and stored sample silently means something else. `FeatureVersion()`
is what guards it — do not work around it.

### 2026-09-23 — hosting decided: Hetzner
`e36d0f4` — DEPLOYMENT.md §3 rewritten from "AWS because free tier" to the
actual decision, with the comparison that drove it (Hetzner 20 TB included and
~$0.001/GB overage vs AWS 100 GB then $0.09/GB).
Gotcha: Hetzner has **no India datacenter**. Singapore is closest, so Indian
visitors pay 60–150 ms that AWS Mumbai would not charge — accepted because
bandwidth decides this product's economics and latency does not. Revisit if
that stops being true.
**Superseded by `46168a7`:** the bandwidth figures in this entry are Hetzner
EU pricing and do not apply to Singapore. Read that entry, not this one.

### 2026-09-23 — external review of PR #13: two fixes, two roadmap items
`3d06c1a` — `Collector.Record` could panic ("send on closed channel") when it
raced `Close`; guarded with an RWMutex and a closed flag. Corrected the comments
that claimed a solved challenge proves a browser ran JavaScript. Opened ROADMAP
28 (decode the canvas PNG) and 29 (label the honeypot trip on the request that
trips it).
Gotcha: a `select` with a `default` case does **not** make a send on a closed
channel safe — it still panics. The non-blocking send read as if it did, which
is exactly why nobody caught it.

### 2026-09-23 — DEPLOYMENT.md §3 corrected: the Hetzner numbers were EU-only
`46168a7` — the "20 TB included, €1/TB" figures that decided the hosting choice
are Hetzner **EU** pricing. Singapore includes 0.5 TB and charges €7.40/TB.
CX22 does not exist in Singapore at all (CX is EU-only; it has CPX/CCX).
Decision still holds — 11× cheaper per GB than AWS Mumbai, not 90×.
Gotcha: hakaishield is a **reverse proxy**, so the origin must live in the same
datacenter as the proxy. Proxy in Singapore with an origin in India makes an
Indian visitor cross that link three times — ~180–240 ms, not ~60 ms.

### 2026-09-23 — hosting settled: DigitalOcean Bangalore
`4bd5221` — DEPLOYMENT.md §3 rewritten again. DO Bangalore ($24/mo, 4 TB
included, $0.01/GiB) costs the same as Hetzner Singapore at 10 TB and sits in
India: 5–40 ms instead of 55–70 ms. `deploy/` retargeted.
Gotcha: the only reason Hetzner ever won was a wrong number — its famous
"20 TB / €1 per TB" is **EU-only**, and Singapore is 0.5 TB / €7.40 per TB.
Once that was corrected the cost gap vanished and latency decided it. Always
check a provider's rate for the region you are actually deploying to.

### 2026-09-23 — Phase 1 policy engine, shadow-only
`d75a26e` — `pkg/policy`: pure Condition/Rule/Policy types, `Evaluate`
(never errors/panics — unknown field/operator/zero-condition all resolve
to no-match), `ValidateRule` (UA-only-allow and deceive-floor-above-block
guardrails). `core.Guard` gets an optional `PolicyProvider` hook, shadow
only: recorded on `evidence.Evidence.Policy`, never drives enforcement.
Claude's half of a Phase 1 split with Codex CLI (`codex-claude-chat.json`).
Gotcha: no provider is attached in `main.go` — this is inert in the
running service until `TenantConfig.OwnerUserID` is plumbed through
(column exists, loader doesn't select it) and a cached rules→policy
adapter is built. See `DECISIONS.md` for the full reasoning.

### 2026-09-23 — Phase 1 policy production review and rule validation
`31053427` — Codex validated dashboard rule creation against the shared
policy matcher, bounded rule input, hardened malformed-policy matching,
repaired the two-tenant shadow test, and passed Go tests/vet/build plus an
isolated real-Postgres rule/ownership check. Full findings:
the historical review in commit `31053427`.
Gotcha: `main.go` still attaches no policy provider; stored rules do not yet
produce live shadow opinions or enforce traffic.

### 2026-09-23 — real DB-backed PolicyProvider (Phase 1 gate #1)
`5f1ebd9` — `pkg/policyprovider`: resolves a request's tenant to its
owning account (new `tenant.Store.OwnerUserID`, new `owner_user_id`
select in `db.GetTenant`/`GetTenantByID`), reads that account's rules
through a bounded/TTL-cached lookup, converts via new `rules.ToPolicy`.
`main.go` wires it whenever `-db-url` is set. Still shadow-only.
Gotcha: cache is keyed by **owner**, not tenant — two domains under one
account must share one cache entry/query. Mutation-tested: keying by
tenant instead made `TestForTenant_TwoDomainsSameOwnerShareRules` fail
as expected.

### 2026-09-23 — PR #13 label and trainer safety
`56433255` — Honeypot trips now emit a candidate label on the trip request;
the trainer refuses auto-collected database labels unless explicitly opted in.
It also rejects incomplete JSONL samples. Tests, vet, and build passed.
Gotcha: challenge and honeypot labels are candidate observations, not verified
ground truth. The model remains shadow-only; do not enable learned enforcement.

### 2026-09-23 — versioned tenant policy and guarded enforcement
`11dece77` `6a0c3a00` — Phase 1 backend: ordered tenant revisions, ownership/audit/
rollback, preview and shadow summary APIs, compiled matcher, asynchronous
provider, and action enforcement after a measured activation gate. The staged
snapshot passed full Go test/vet/build, real Postgres isolation, and a
hard-block PASS mutation check. See the Phase 1 API contract in
`BACKEND_IMPLEMENTATION_PLAN.md`.
Gotcha: production activation still needs representative traffic review and
durable cross-node telemetry. Concurrent uncommitted Phase 2 challenge work
in the shared tree currently fails a challenge flow test; it is not in this
commit.

### 2026-09-23 — Phase 2 adaptive challenge, completed
`959ab46d` (branch `phase2-challenge-trust`) — difficulty banded by the
guard's risk score (1–3 hex zeros, clamped), escalation via a signed attempt
cookie, trust decay 30/15/5 min on the passed cookie, bounded telemetry with
browser-class counters, guard wiring, 16 new tests, 5 mutation checks, docs
(PLAN/ROADMAP/DECISIONS/RESEARCH; the dated status file was consolidated).
Gotcha: html/template's JS escaper pads interpolated numbers
(`var difficulty =  1 ;`), so strict regexes silently match nothing — and
test helpers must read the difficulty from the served page, never hardcode
"00", which is a valid difficulty-1 answer and can make reject tests pass
by accident. The branch also carries Codex's `5ae489b9` (scripting UA
blocklist) because we share one working tree; merge both together or
cherry-pick accordingly.

### 2026-09-23 — Phase 2 verification hardening
`47da7c51` — bounded PNG decoding and nonblank canvas validation, server-
enforced attempt-cookie expiry, retry/no-JavaScript guidance, and FIFO local
nonce eviction. Blank PNG, expired cookie and nonce-cap mutation checks went red.
Gotcha: a scripted client can still forge a valid PNG; challenge solves remain
candidate observations, never verified human labels.

### 2026-09-24 — first client pilot hardening and reference audit

`da9b44b3` (branch `release/client-pilot-hardening`) — tenant-scoped dynamic
Redis counters, replay-safe nonce capacity, stable secrets/host-bound Compose,
shadow-only client-hint evidence, corrected bot ladder, and all five reference
repos mapped to a concrete pilot release gate. Tenant isolation, nonce, route,
and secret regressions were red before fixes; full Go test/vet/build, lint,
Linux cross-build and Compose config passed.
Gotcha: Docker image build was blocked by Docker Hub DNS and Windows race build
by missing gcc. Live domain/origin traffic and detection rate are unmeasured.

### 2026-09-24 — passed-session detection continuity

`0a886dcc` (branch `release/client-pilot-hardening`) — Guard now runs the
scorer once on each passed-cookie request, blocks newly decisive scraper
evidence, rate-limits server-observed burst/crawl behavior, and records fresh
signals. Chromium platform/mobile hint contradictions join major mismatch as
shadow-only evidence. Full Go test/vet/build and lint passed; the bypass and
new shadow-signal tests were red before implementation.
Gotcha: this adds bounded Redis checks after a solve; live latency and false
positive measurement remain pilot gates. Browser-hint observations do not
enforce.

### 2026-09-24 — production-test packaging and velocity test gate

`f4e3b6ab` — Production Compose image built locally and its packaged binary
started. The passed-session velocity test now tolerates a one-second fixed-window
boundary; full Go suite, repeated focused test, and vet passed.
Gotcha: the image check does not replace live TLS, origin, browser, and load
tests on the pilot host.

### 2026-09-24 — managed pilot onboarding and dashboard hardening

`2094da41` — Dashboard was reduced to real pilot capabilities: no fake signup,
contact submission, billing, legal, domain activation or live-policy controls.
Pending domains are never shown as protected; operator-managed setup is explicit.
The API rejects customer-created unverified domains, Compose requires database,
Supabase and exact dashboard-origin settings, and dashboard/Go regression tests
plus CI gates were added.
Gotcha: self-service DNS ownership, ACME certificates, billing and legal terms
remain launch blockers for a hosted SaaS; this release is a managed pilot.

### 2026-09-24 — GREASE-brand client-hint shadow signal

`3f64e5c3` — `client_hint_missing_grease_brand` added to the client-hint
shadow signals: a Sec-CH-UA header naming only real brands, missing
Chromium's mandatory randomized GREASE brand, is evidence of a forged
header. Shadow-only, same as the other client-hint candidates.
Gotcha: none — pure header parse, no new state, mutation-checked.

### 2026-09-24 — WebGL renderer-vs-OS mismatch check

`0f5478b7` — Challenge page's WebGL renderer check now also flags a
renderer naming an OS-exclusive backend (Direct3D/Metal/Adreno) that
disagrees with the claimed platform, alongside the existing SwiftShader/
llvmpipe headless-VM check. Same `headless` flag, same rejection path.
Gotcha: JS-only logic, so it's tested by asserting the regex markers
survive in the served page (mutation-verified), not by executing it.

### 2026-09-24 - session handoff report

The dated handoff from this commit was later consolidated into
`docs/CURRENT_STATUS.md` with detection, dashboard and deployment findings.

### 2026-09-24 — challenge browser shadow candidates

`0e4b3674` — Valid challenge solves now record bounded tenant shadow evidence
for WebGPU f16 absence, duplicate canvas output, pointer inactivity, and
legacy automation keys; decisions remain unchanged. Tests, mutation checks,
vet, build, lint, and a JavaScript runtime smoke check passed.
Gotcha: exact-host evidence is skipped on a cold node until its tenant is loaded.

### 2026-09-24 — DNSBL pilot scope

`12115dae` — Defer DNSBL/IP reputation for the low-cost managed pilot;
revisit after labelled client traffic shows a specific coverage gap.

### 2026-09-24 — dashboard evidence and pilot TLS audit

`5f773e98` — Scope dashboard evidence to the selected owned domain, show
observed signals separately, and provide the non-root container a restricted
certificate copy that the renewal hook refreshes.

### 2026-09-24 — current status and documentation cleanup

`c44b83d6` — Consolidate completed work, behavioural detection gaps and
client launch gates in `CURRENT_STATUS.md`; remove obsolete snapshots and
correct stale product and architecture claims.

### 2026-09-24 — client-ready detection plan

`ea136178` — Sequence the managed-client launch, fifteen reviewed detection
tracks, `pkg/decide` evidence gates, and cloud-cost controls in
`CLIENT_READY_IMPLEMENTATION_PLAN.md`.
Gotcha: 80% blocked bots requires measured hard-block recall on independent
labels; challenges cannot be counted as blocks.

### 2026-09-25 — bounded host lookups + CI bug-finding

`b2d5ad4` — Unknown Host headers now cost at most 8 concurrent DB lookups per
node (same host shared); CI adds errorlint/bodyclose/nilerr/sqlclosecheck,
govulncheck and four 20 s fuzz runs.
Gotcha: during a random-host flood a brand-new domain's first request can get
"not found" — by design, it is not negative-cached and resolves next request.

### 2026-09-25 — docs collapsed to five files

`a7ea917` — docs/ went from 16 files / 11.3k lines to five short files
(STATUS, ARCHITECTURE, DEPLOYMENT, DECISIONS, PROGRESS); references updated.
Gotcha: CLAUDE.md now says docs/ holds exactly five files — extend one,
never add a sixth.

### 2026-09-28 — pilot deploy branch: main + release + HTTP/2

`c7c3cb7a` — Merged the 22 release commits (UUID owners, split-host
`-origin-host-from-target`, AWS staging, Pages frontend, crawler-claim fix)
onto main; release-only docs folded into the five files, runbook moved to
`deploy/RUNBOOK.md`, frontend handoff to `dashboard/FRONTEND.md`.
`2dfdb2d5` — HTTP/2 greeting fingerprint, weight 0, forwarded as
`X-HakaiShield-HTTP2`; fixed Shutdown hanging on open h2 conns, preconnects
being dropped, silent partial `.env` loads; circl v1.6.3.
Gotcha: adding `h2_tool_match` changed `FeatureVersion`, so any model or
samples from an older build are refused by design — retrain, don't patch.
Gotcha: local Go 1.26.5 has 7 stdlib vulns fixed in 1.26.6; build the
image with `--pull`.
