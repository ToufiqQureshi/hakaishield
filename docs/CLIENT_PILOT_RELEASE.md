# First client pilot: release gate (2026-09-24)

## Scope and honest claim

This branch prepares a **single-node, single-domain pilot**. Start in shadow
mode and show the client real evidence before enabling blocking. No production
traffic or independently labelled bot sample has been measured yet, so the
requested 70–80% catch rate cannot be claimed. A scripted ladder is a regression
tool, not a population-level detection estimate. The current backend is not a
multi-region, SLA-backed hosted service.

## Source review of all five `inspired/` repositories

The reference code was inspected for threat and operational ideas. No reference
code was copied into the product.

| Repo | Useful finding | Pilot decision | Later work |
|---|---|---|---|
| `horizon` (FCaptcha) | `HARDENING.md`, `server-go/detection.go` and `inputforensics.go` cover bounded replay state, browser consistency and measured input cadence. | Stable secret/replay storage are built. Treat timing thresholds as lab-specific until our clients are sampled. | Admission quotas, privacy-safe behavioral telemetry and multi-node replay state need load/outage tests. |
| `nexus` (Anubis) | Policy recipes under `data/common/` exempt robots, sitemap, favicon and `/.well-known/` routes and distinguish APIs from pages. | Keep verified crawlers and known-good paths out of aggressive global defaults; first client policy must be reviewed for their origin. | Endpoint-specific policy presets with tenant preview and accessibility tests. |
| `vertex` (bot-signal) | `src/server/analysis.ts` uses corroborating browser, client-hint, platform and TLS evidence rather than a single claim. | Record bounded Chromium major, platform and mobile contradictions as **shadow-only** evidence. | Evaluate variants on real traffic before any score weight. |
| `quantum` (Brotector) | `brotector.js` catalogs WebDriver/CDP/debugger/stack artifacts and aggressive prototype hooks. | Use the catalog as lab test cases only. Do not ship debugger traps, crash behavior, invasive hooks or a lone CDP block. | Browser automation regression suite after the first client's normal browser sample. |
| `zenith` (go-away) | Go conditions/actions, challenge pass/fail paths and `resource-load` challenge show the value of explicit outcomes and browser resource checks. | Existing versioned policy and signed challenge stay the integration point. A passed cookie no longer skips subsequent scoring. | Test resource-load fidelity and session continuity after live shadow review. |

## Code gate complete in this branch

- Tenant-scoped Redis counters for endpoint-class velocity (login/API/checkout/nav/asset buckets), JA4 velocity,
  and distinct-path crawl. Cross-tenant tests cover each key family.
- Public puzzle-minting route closed in the production mux; Guard can issue
  only after resolving a tenant. Replay and short-secret regressions pass.
- Stable secret, evidence token, public host and origin are required by Compose.
  Evidence token/secret travel in environment variables, not process arguments.
- Certbot's root-only private key is copied to a restricted root:65532 directory
  for the non-root container. The renewal hook refreshes the copy before restart.
- Redis keeps nonce keys until TTL (`noeviction`) and persists them in an AOF
  volume. At capacity, the local nonce store rejects new solves.
- Chromium client-hint mismatch appears as `shadowSignals` in evidence and
  aggregate counters. Major, platform and mobile contradictions have no score
  or enforcement weight.
- A solved challenge now grants temporary relief from repeat interstitials,
  while each later request is scored again. New scripting/JA4 hard-block
  findings still block; server-observed velocity/crawl can rate-limit.
- Bot ladder's crawl rung now sends enough browser-claiming distinct paths to
  exercise `crawl_pattern`.

## Deployment gate for this evening

Local verification on 2026-09-24: `go test ./... -count=1`, `go vet ./...`,
`go build ./...`, Linux/amd64 CGO-off cross-build of both binaries,
`golangci-lint run ./...` (0 issues), Python syntax parse, and Compose config
with dummy required values passed. The initial Docker Hub DNS issue cleared:
the production Compose image built locally and its binary started with `-h`.
The Go suite and `go vet ./...` passed again after the fixed-window velocity
test was made resilient to a one-second boundary. Windows `go test -race`
could not start because the C compiler is absent; Linux CI ran the race suite.
The AWS staging host has valid live TLS and a passing health endpoint. The root
origin path exposed a split-host 403 and must be retested after deploying the
explicit target-host option; real browser and client-origin smoke tests remain.
The tenant-isolation, nonce-cap, short-secret and public puzzle-route tests
were observed failing against the previous behavior and passing after the
corresponding fixes; the shadow-signal test was compile-red before its code
was added.
The passed-cookie bypass test was red against the previous Guard: a solved
cookie plus `python-requests` reached the origin and appeared as `allow`. It
now blocks and records the fresh signal in enforce mode, while shadow mode
records the same decision and forwards. Platform/mobile hint contradictions
were also tested red before implementation and pass now. The full Go suite,
vet, build and lint passed again after these changes.

1. Obtain the actual domain, origin URL, and a server close to the origin.
   Point the domain to the server **before** running `deploy/setup.sh`, because
   Certbot's HTTP-01 check must reach that server. Issue a valid TLS certificate. Keep the
   origin restricted to the proxy where possible; otherwise direct-origin
   access bypasses all decisions.
2. Fill `deploy/.env` from the example, generate each token with
   `openssl rand -hex 32`, and keep the file out of Git. Run
   `docker compose --env-file deploy/.env -f deploy/docker-compose.yml config --quiet`
   before `up`. The Compose pilot requires PostgreSQL and Supabase URL so the
   client dashboard cannot silently start with its API disabled.
3. Start in `HAKAISHIELD_MODE=shadow`. Confirm TLS/JA4 varies by client,
   correct Host routes, real browser/origin functionality, health endpoint,
   Redis health, evidence authentication, restart recovery, and cert renewal.
4. Run `bot-testing/ladder/ladder.py` only against our own pilot site. Review
   real visitor samples for false positives across mobile, privacy browsers,
   search crawlers, accessibility tools, checkout and login. Keep label
   collection off during the attack simulation.
5. Switch to enforce only after the client approves the impact and a reviewed
   shadow sample supports the policy. Keep the previous Compose mode and
   image/commit available for quick rollback. Monitor 403/challenge rates,
   origin errors, Redis errors, latency and challenge solve failures.

## Managed pilot domain and dashboard onboarding

Self-service domain creation is intentionally disabled in the pilot. A user
could otherwise reserve any unverified host in the unique `tenants.host` column,
and neither ownership verification nor automatic per-domain TLS exists yet.
The dashboard states this clearly. The operator provisions one domain using
the host-bound `deploy/.env` configuration and the certificate issued by
`deploy/setup.sh`.

After the operator verifies ownership, DNS, TLS, origin routing and shadow
traffic, create the client's Supabase Auth account and obtain its Auth user ID.
On a **fresh single-client pilot database**, bind that account to the already
running default tenant in the Supabase SQL editor:

```sql
INSERT INTO public.tenants
    (id, host, target, mode, owner_user_id, name, status)
VALUES
    ('default', 'customer.example', 'https://origin.example', 'shadow',
     '<Supabase Auth user ID>', 'customer.example', 'active');
```

Replace every example value. An existing `default` row or host conflict means
stop and inspect the current owner; do not overwrite it. The `active` row is
for dashboard ownership/status. Live routing still comes from the running
proxy's `HAKAISHIELD_DOMAIN`, `HAKAISHIELD_ORIGIN` and `HAKAISHIELD_MODE`.
Restart the proxy after the SQL bind. It checks that the row's host and origin
match the live settings and loads the owner for policy drafts at startup.
Check authenticated `GET /api/v1/domains`, dashboard stats/evidence, and the
real browser before handing credentials to the client. Configure the dashboard
build variables and Auth redirects as described in `../dashboard/README.md`.

Before routing client traffic, the operator must provide the actual pilot
agreement, privacy notice, retention terms and working contact address for
client review. The previous public draft pages contained unimplemented Stripe,
refund, retention and SLA promises; the dashboard no longer presents them as
binding documents. Public signup is invitation-only until reviewed terms exist.

## Known limits before client handoff

- In-memory evidence and per-node shadow statistics do not survive restart.
  This limits audit/history and policy activation evidence.
- A Redis outage falls back to per-node challenge nonce state; replay across
  nodes during that outage is not prevented. Keep the pilot to one proxy node.
- AOF with `everysec` can lose recent writes on a host crash; fully durable
  replay guarantees need a different fail-closed design.
- The shadow client-hint candidates have no measured precision or recall. New
  Phase 3 behavior, asset fidelity and HTTP/2 intelligence remain research
  work, not pilot protection claims.
- The four challenge-page browser candidates do not run while the proxy is in
  shadow mode, because visitors are forwarded without a challenge. They add
  no measured detection coverage to the initial pilot; a reviewed challenge
  cohort is needed before evaluating them against real visitors.
- Public staging DNS, certificate and health now pass. Root origin routing,
  restart recovery, representative load and real-browser/client-origin checks
  still need completion before client handoff.
