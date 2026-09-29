# Status — what works, what doesn't, what's next

Updated 2026-09-28. Code and tests are the final truth.
Read this first, then `CLAUDE.md`.

## Product in one paragraph

hakaishield is a hosted inline proxy. It terminates TLS in front of a
customer site, scores each request from request-level evidence (JA4,
headers, rate, traps), then allows, challenges, rate-limits, deceives or
blocks, and records why. It is not "cheap DataDome": the price floor is
free (Cloudflare free, CrowdSec). Our edge is scoring the first request
from the live TLS handshake, with an explanation a customer can check.

Target buyer: mid-size sites where bots leak revenue (e-commerce,
ticketing, job boards, paid APIs). Self-hosted is a priced Enterprise
option; do not build tooling for it, do not delete the single-tenant path.

Success = customer points DNS at us, sees bots drop on day one, real
users never break, and it runs for months without babysitting.

## Honest state

- Code is ready for a **managed, one-domain, one-node shadow pilot**.
- It has **never served real traffic**. No domain, server or labelled
  traffic yet. "70–80% catch rate" is a target, not a result.
- Backend: ~9.5k lines Go, ~9.6k lines tests, all passing in CI.

## What is built

| Area | Works today | Limit |
|---|---|---|
| Proxy | TLS, JA4, h2 fingerprint, Host/SNI check, shadow/enforce | h2 signal weight 0; one node |
| Scoring | 10 checks (9 scored), additive weights, block at 100 | Weights are hand-set guesses |
| Challenge | Signed PoW + canvas, difficulty 1–3, replay-safe | Telemetry forgeable |
| Deception (11a) | Decoy HTML + honeypot link | Opt-in; no FP tracking |
| Good bots | Reverse+forward DNS for 6 search engines | No AI-agent policy yet |
| Tenants | Host routing, tenant-scoped Redis keys and APIs | Onboarding is manual |
| Policy | Versioned, preview, rollback, route labels | Old rules pages shadow-only |
| Evidence | Per-request reasons, 1000-entry ring, 24h | In memory; lost on restart |
| Metering | Per-tenant egress bytes, challenge solve/fail counts | No caps or billing |
| Learned model | Pure-Go logistic model, shadow only, v2 provenance | No verified labels |
| Dashboard | Supabase Auth, stats, evidence, offenders, route labels | Small test coverage |
| CI | vet, race tests, golangci (gosec, errorlint…), govulncheck, fuzz | — |

## What a good bot still gets past

A real browser (Patchright-class), normal headers, not on the JA4
blocklist, under rate limits, never touching the honeypot. Shadow signals
now record the cheap tells (datacenter IP, no assets, direct login POST,
opt-in beacon: webdriver / no input / script never ran). None scores yet;
a Patchright bot on a residential proxy that loads assets and fakes input
still passes. Scoring them needs pilot FP data; the moat is item 19.

## Launch gate — before the first client request

1. Owner supplies: domain, origin, server near origin, legal entity,
   support email, pilot terms, retention period, rollback contact.
2. DNS A record to server **before** Certbot. CDN in DNS-only mode.
3. Fill `deploy/.env` on the server, run `deploy/setup.sh` and Compose.
4. Build dashboard, bind the tenant (SQL in `DEPLOYMENT.md`), restart.
5. Smoke-test real HTTPS: JA4 varies by client, login/checkout work,
   crawlers pass, challenge + rollback work, measure p95/p99 latency.
6. Run in `shadow`. Label a real sample. Record recall and human
   false-positive rate. Enforce only after client sign-off.

## Open decisions that belong to the owner

- **Label selection bias.** Only suspicious traffic gets challenged, so
  "human" labels all come from suspicious-looking humans. Pick a fix
  before any model enforces: sample clean traffic, reweight, or let the
  model act only in the suspicious band (leaning to the last).
- **Honeypot label on the tripping request** (item 29) changes training
  data, so it goes with the bias decision.
- **Per-user rate limit on `/api/v1/*`** — values not chosen yet.

## Next, in order (roadmap item numbers kept for code references)

**Product (needed before anyone pays):**
- 21 Domain ownership proof + automatic ACME certificates
- 22 Usage metering + bandwidth caps per plan
- 23 Billing (Stripe) + self-serve signup
- Durable evidence and shadow stats; multi-node replay/failover

**Detection (add only when labelled traffic shows the gap):**
- 7 Behavioural signals: opt-in beacon **built, shadow only** (09-29).
  Next: measure FP on pilot, then weigh.
- 19 Known-browser JA4 database — the real moat. Versioned, expiring.
- 11 Per-tenant threshold tuning, after shadow data exists
- 8 Session consistency (timezone vs IP vs OS), corroboration only
- 9 Asset fidelity and direct login/checkout POST **built, shadow only**
  (09-29); longer route sequences still open
- Promote h2 fingerprint above weight 0 (needs FP review); ClientHello
  multi-record reassembly
- 11b Verified AI-agent policy (Web Bot Auth, RFC 9421)
- 25/26 Learned model: independent labels, holdout beats rules, a
  person approves. Never auto-retrain.
- 29 Honeypot trip labelling (owner decision above)

**Scale:**
- 27 Sideband decision API when bandwidth dominates the bill
- 16 Soak/load test; 15 Prometheus metrics
- DNSBL/IP reputation: deferred until a measured gap justifies the cost

## Where things live

| Question | File |
|---|---|
| How does a request flow? How is it scored? | `ARCHITECTURE.md` |
| Where is it hosted, what does it cost, how to launch? | `DEPLOYMENT.md` |
| Why was X chosen, what was rejected? | `DECISIONS.md` |
| When did X change, and what bit us? | `PROGRESS.md`, then `git show` |
