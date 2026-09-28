# HakaiShield: current status and remaining work

**Snapshot:** 2026-09-28, branch `release/client-pilot-hardening`. Read this file first for the
handoff; `CLIENT_PILOT_RELEASE.md` is the operational release checklist,
`SIGNAL_COVERAGE.md` is the signal inventory, and `CLIENT_READY_IMPLEMENTATION_PLAN.md`
is the client-ready and detection implementation plan. Code and tests remain the final source
of truth.

## Honest product status

The code supports a **managed, one-domain, one-node shadow pilot**. An owned AWS
staging deployment now has public DNS and valid TLS, but it is not a completed
client handoff: the real client domain/origin, authenticated tenant binding,
full browser journeys, restart/load checks, and labelled production traffic
remain unverified. **70–80% bot detection is a target, not a measured result.**
A staging health check and bot ladder cannot establish population-level recall
or false-positive rate. Do not promise production availability or enable
enforcement from this document alone.

The `hakaishield-dashboard` Cloudflare Pages deployment serves
`https://interviewyaar.lol`; the current public build is the marketing surface,
not the authenticated dashboard. The AWS proxy is reachable at
`https://shield.interviewyaar.lol`, where public TLS and the health endpoint
pass. Its root path needs the tested split-host option deployed before it can
proxy the Pages site successfully. Supabase Auth redirects, tenant ownership,
the authenticated dashboard build, and its final API URL remain launch work.

PR #19 merged an earlier release snapshot to `main`. Later audit commits are
being finalized on `release/client-pilot-hardening`; the workflow
runs on pull requests and `main`, so these later commits have no remote CI run
yet. The worktree also has concurrent, uncommitted HTTP/2 code and unrelated
changes in `graphify-out/`, `patchright_test.py`, `bot_shield_test.py`, and
`CODEX_TODO_bot_detection.txt`.
Inspect `git status` before staging and do not sweep those into a docs commit.

## What is implemented

| Area | Current behavior | Limit that matters |
|---|---|---|
| Inline proxy | Go terminates TLS, captures ClientHello/JA4, checks Host/SNI, forwards to an origin, and supports `shadow` or `enforce`. A deployment can explicitly use the target URL's virtual host when a staging hostname fronts a separately hosted origin. | The AWS pilot is one node; public TLS and health pass, while restart recovery and representative latency remain unmeasured. |
| Scored detection | Nine scored checks cover TLS/UA/header mismatch, known bad JA4, openly named scripting tools, per-IP velocity in **five endpoint-class buckets** (login 10/s, API 100, checkout 20, nav 20, assets 300 per 1s window), per-JA4 velocity, distinct-path crawling, and a tenant honeypot. Unverified crawler UA claims now enter the existing distinct-path check; supported crawlers remain exempt only after reverse/forward DNS verification. Authenticated customers can save exact login/checkout route labels in a versioned shadow policy; labels affect the existing velocity check only after policy activation. Redis keys are tenant-scoped. | A real browser with plausible headers and slow requests can evade these checks. Unverified crawlers crossing 60 distinct pages/minute may be challenged in enforce mode; thresholds need client traffic calibration. |
| Challenge and continuous trust | Signed, host-bound challenge/cookies, bounded proof-of-work and canvas checks, nonce replay protection, adaptive difficulty, and rescoring after a passed cookie. The verify path now has a bounded concurrent admission ceiling (64; overflow gets a counted 503), and per-tenant solve/fail outcomes feed the dashboard. Fresh high-confidence evidence can still block; velocity/crawl can rate-limit. | A scripted client can forge browser telemetry and PNG proof. In proxy shadow mode visitors never see this challenge. |
| Additional observations | Chromium client-hint contradictions are recorded as `shadowSignals` during normal proxy shadow traffic. Four more candidates—WebGPU f16 absence, duplicate canvas output, pointer inactivity, and legacy automation globals—are recorded only after a valid **enforced** challenge solve. | None changes score or action. The four challenge-only candidates collect **no real-visitor samples in initial proxy shadow mode**. |
| Verified bots and false-positive controls | Search crawler claims use bounded reverse/forward DNS verification. Policies have explicit owner checks, versioning, shadow preview, rollback, and an activation gate. | The policy gate uses local aggregates and needs real traffic review. The dashboard's legacy rule/settings pages do not control live policy. |
| Evidence and dashboard | Tenant-scoped evidence, stats and offenders; selected-domain queries are ownership checked. Dashboard distinguishes scored signals from yellow observed candidates and cancels stale domain-switch requests. P1 measurement is wired end to end: per-tenant egress bytes (saturating counter, measured at the response writer) and challenge solve/fail counts appear in `/api/v1/dashboard/stats` and the Overview page. Rate-limited requests are visible in Overview and selectable in the evidence filter. | Evidence and several aggregates are in memory and disappear on restart; UI test coverage is still small. |
| Learned scoring | Candidate label pipeline and pure-Go trainer/model exist. Candidate samples in Postgres are pruned at startup and hourly in bounded batches, default 30 days (operator configurable). Loaded model predictions are recorded alongside rule decisions. Model artifacts are version 2 with provenance. The evaluation helper now keeps its holdout strictly later and identity-clean, and refuses a holdout without both classes. The trainer refuses an approval stamp for candidate labels, zero holdout, or a holdout with fewer than 50 rows and 5 of either class. | Model is **shadow-only**. The trainer's JSONL format lacks event time and identity, so its own holdout is not yet the locked, leakage-safe evaluation needed for promotion. No representative, independently reviewed labels or measured model quality exist. Client-approved sample retention still needs confirmation before traffic collection. |
| Managed onboarding and packaging | Self-service domain creation is disabled; operator binds the verified client domain. Compose requires stable secrets, Postgres, Supabase and dashboard origin. Certbot key is copied to a restricted directory for the non-root container; the renewal hook refreshes it before restart. | Domain ownership, TLS and origin checks are manual for this pilot; automatic multi-domain onboarding and billing are not built. |

The five `inspired/` repositories were reviewed for ideas, with no code copied.
`nexus` informed policy/challenge handling; `horizon` replay and input forensics;
`vertex` corroborated browser/client-hint evidence; `quantum` automation
regression cases; `zenith` conditions, outcomes and resource checks. Risky
debugger/crash hooks and unmeasured timing thresholds were not adopted. See
`CLIENT_PILOT_RELEASE.md` for the per-repository decision table.

## Work completed on 2026-09-24

- Hardened passed-cookie handling so each later request is scored again;
  closed a challenge bypass and improved tenant-scoped Redis counters.
- Added client-hint GREASE and major/platform/mobile contradictions as
  observation-only signals; added WebGL platform mismatch checks on the
  challenge page and four bounded challenge telemetry candidates. Those four
  do **not** increase pilot detection coverage in proxy shadow mode.
- Deferred DNSBL/IP reputation for the low-cost pilot. It could flag known bad
  source IPs but introduces list quality, false-positive, latency and provider
  cost questions; revisit only if labelled traffic shows a gap it can close.
- Reworked the dashboard into an honest managed-pilot flow, removed fake
  payment/trial claims and placeholder data, and required a production API URL.
- Fixed domain switching for evidence/offenders and made unowned domain IDs
  fail closed. Exposed observed evidence separately from scored signals.
- Fixed Docker TLS-key access for the non-root image by mounting a restricted
  certificate copy rather than Certbot's root-only tree.

Relevant commits: `da9b44b3`, `0a886dcc`, `2094da41`, `3f64e5c3`,
`0f5478b7`, `0e4b3674`, `12115dae`, `5f773e98`. The short commit index is
`PROGRESS.md`; commit messages hold the detailed tests and trade-offs.

## Work completed on 2026-09-25

- Per-tenant exact-path login/checkout labels use the existing velocity signal,
  with strict validation, owner-scoped versioned shadow drafts, an activation
  gate, and dashboard editing. Built-in sensitive routes cannot be weakened.
- Candidate training samples now have a configurable, bounded hourly cleanup;
  the default pilot owner is loaded from a matching active database row on
  startup, so policy ownership does not rely on a visitor-supplied value.
- P2 review found no additional safe new signal without client traffic or an
  approved browser collection surface. Route sequences, asset fidelity and
  client-hint promotion remain evidence/calibration work.

## What remains, in execution order

### 1. Before the first client request — launch gate

1. Owner supplies the actual domain, protected origin, a server near that
   origin, legal entity/jurisdiction, working support/legal email, approved
   pilot terms, retention period and support/rollback contact.
2. Point DNS to the server before Certbot HTTP-01. Keep any CDN in DNS-only
   mode if it would terminate TLS before HakaiShield; otherwise JA4 loses its
   visitor ClientHello. Restrict direct origin access where practical.
3. Fill `deploy/.env` on the server with stable generated secrets and the
   required domain/origin/Postgres/Supabase/dashboard origin values. Run
   `deploy/setup.sh`, Compose config/build/up and certificate renewal checks
   exactly as in `deploy/README.md`. Never commit secrets.
4. Build the static dashboard with its Supabase and API URLs, configure Auth
   redirects, manually bind the verified pilot tenant using the SQL in
   `CLIENT_PILOT_RELEASE.md`, restart the proxy to load the default tenant's
   owner, and test the authenticated dashboard and route-tag drafts.
5. Smoke-test the **real** HTTPS request path: SNI/Host, JA4 variation across
   clients, origin routing, normal login/checkout/API journeys, good crawlers,
   challenge and rollback paths, health, Redis outage/recovery, reboot and
   renewal. Measure p95/p99 latency and origin errors at expected load.
6. Start in `shadow`, review real visitor evidence with the client and label a
   representative sample of humans and bots. Record sample size, bot recall,
   human false-positive rate and challenge solve/failure rates by browser and
   device. Tune policy before any scoped enforcement; get client sign-off and
   keep a tested immediate rollback path.

These are release gates, not evidence that the current code is broken. They
require the real client environment; none is verified by local unit tests.

### 2. Detection coverage after the pilot baseline

| Gap | Why it matters | Next safe step |
|---|---|---|
| **Client-side behavioural analysis** | Slow automation in a real browser can look normal in JA4, headers and request rate. | Choose an opt-in first-party collection path for normal page views; collect bounded interaction timing, scroll/navigation cadence and session continuity, without raw coordinates or keystrokes. Start as evidence only, measure across mobile, accessibility tools and privacy browsers, then review any score weight. |
| Challenge-only probes | Current four candidates have no samples during proxy shadow mode. | Run a separately reviewed challenge cohort after the client agrees; measure false positives before promotion. A solve is not proof of a human. |
| Asset/referrer fidelity and API patterns | A scraper can load pages slowly while skipping assets or enumerating API pages. | Establish legitimate per-endpoint baselines, then add bounded session-level evidence and endpoint-aware rules in shadow. |
| Maintained browser JA4 knowledge and HTTP/2 | Current JA4 blocklist is narrow and the listener lacks HTTP/2 fingerprinting. | Build a curated, versioned, expiry-aware fingerprint feed; unknown fingerprints stay neutral. Validate protocol changes against real browsers before scoring. |
| Session consistency and optional IP context | Proxy/fingerprint pipelines may claim inconsistent language, timezone or network context. | Corroborate multiple signals; keep geography/IP lists out of single-signal blocking. DNSBL stays deferred until measured need. |
| Learned model | Candidate labels are biased and can be forged; model has no validated recall/FPR. | Curate independent labels, hold out evaluation data, compare shadow predictions by tenant/browser and keep enforcement off until explicit review. |

Behavioural analysis is **not built for ordinary page traffic**. Existing
`velocity_spike`/`crawl_pattern` describe server-side request behaviour, and
the challenge's pointer-active flag is only a coarse, visitor-reported bit
after an enforced challenge. Neither equals mouse/click/scroll behavioural
scoring. Building this well needs an agreed collection surface, privacy limits,
bounded storage and real-user false-positive testing; it is not a safe
same-day hard-block switch.

### 3. Product/platform work after one-domain validation

Automated domain ownership verification and ACME, durable evidence and shadow
aggregates, full dashboard editing of tenant policy beyond route tags,
usage metering/billing, support
operations, multi-node replay/failover, load/soak testing, and reviewed legal
documents remain. These are requirements for a self-service hosted product,
not capabilities of the managed single-client pilot. The detailed backlog is
in `CLIENT_READY_IMPLEMENTATION_PLAN.md` and `ROADMAP.md`.

## Verification actually completed

On the current release branch, local checks passed: backend
`go test ./... -count=1`, `go vet ./...`, `go build ./...`, and
`golangci-lint run ./...` (0 issues); dashboard `npm run typecheck`,
`npm test` (12 tests), and `npm run build`; Compose config with dummy values.
The 2026-09-25 audit fixed a model split chronology leak, a one-class promotion
gate, a calibration bucket panic, and unsafe trainer approval stamps. Focused
regressions failed before each fix and passed afterwards. The production Docker
build initially failed because the uncommitted HTTP/2 dependency update raised
`go.mod` to Go 1.26 while the builder stayed at 1.25; the builder was aligned
and a local image build passed. The uncommitted HTTP/2 implementation also has
parser and preface-timeout fixes; the current worktree passed focused HTTP/2
tests and Linux `go test -race ./...` in a Go 1.26 container. It is not part of
the committed pilot release yet.
The route-label and retention changes passed focused red/green and mutation
checks, including temporary-Postgres tests for batch deletion and policy
persistence. Earlier branch work passed a production Docker image build,
binary `-h`, shell syntax, and a TLS permission/renewal fixture confirming
UID/GID 65532 can read the key while an unrelated user cannot. On 2026-09-26,
the AWS staging host served a valid Let's Encrypt certificate and returned 200
from the public health endpoint; Certbot renewal dry-run passed, Redis was
healthy, and startup logs confirmed shadow mode. The 2026-09-28 recheck again
passed certificate and public health validation. The root path exposed a real
split-host integration bug: Cloudflare Pages rejected the protected staging
Host with 403. An explicit target-host option was added with red/green and
mutation coverage; it still needs deployment after the operator's changed IP
is allowed through the SSH security rule. Restart recovery, representative
load, full browser journeys, tenant owner binding, and measured bot recall/FPR
have not passed.

## Documentation cleanup and next-agent entry point

This file replaces the dated `SESSION_HANDOFF_2026-09-24.md`. The generic
`go_expert.txt` tool list and obsolete Phase 1 review/Phase 2 status snapshots
were removed; their lasting decisions and open work are recorded here, in the
implementation plan, the topic docs, and Git history. Consult the source code
and Git history for the older Phase 1 API contract.

For the next session: read `CLAUDE.md` and `AGENT.md`, then this file and
`CLIENT_PILOT_RELEASE.md`; run `git status` and check the release branch before
touching concurrent edits. Finish the real launch gate before promising a
client a catch rate. Keep behavioural/browser candidates in observation mode
until labelled traffic supports a safe enforcement decision.
