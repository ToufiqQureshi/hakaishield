# hakaishield Architecture

**Pilot reality (2026-09-25):** The current deploy path is one TLS-terminating
Go proxy with a host-bound default tenant and private Redis. Velocity, JA4
velocity and crawl counters are tenant-scoped. Redis persists nonce markers
without eviction; on outage, nonce fallback is local to the node. A bounded
Chromium client-hint major/platform/mobile mismatch is stored as
`shadowSignals` evidence with no score/action effect. A passed challenge
cookie skips repeat low-risk puzzles while subsequent requests are rescored.
Completed challenge solves can also record bounded, per-tenant shadow evidence
for optional WebGPU f16 support, repeated canvas output, pointer inactivity,
and legacy automation globals. Verify uses a cached exact-host tenant lookup;
unloaded tenants yield no challenge shadow record on that node.
The proxy's shadow mode forwards without serving a challenge, so these
challenge-only observations appear only when an enforced challenge is solved.
Hosted global availability, automated tenant certificate
issuance and durable evidence are planned architecture, not current pilot
capabilities. See `CLIENT_PILOT_RELEASE.md`.
Exact custom login/checkout paths are stored in a tenant's owner-scoped,
versioned policy. Shadow drafts are inert for velocity; after activation, the
existing `velocity_spike` check uses the tagged route's bucket. The default
pilot tenant's owner is bound from a matching active database row at startup.
Candidate training samples are pruned off the request path at startup and
hourly in bounded batches; evidence/history still use in-memory storage.
The client dashboard is a separate static app with Supabase Auth and an
authenticated Go API. The first-client domain is operator-provisioned;
unverified customer-created routing rows and automated certificate issuance
are not part of the pilot.

This document covers the current proxy architecture and planned hosted design. Throughput and latency for a real client have not yet been measured. See `docs/ROADMAP.md` for upcoming features, `docs/DECISIONS.md` for design reasoning, and `CLAUDE.md` for engineering rules.

---

## Target delivery model — hosted service

The target is a hosted multi-tenant service. The current managed pilot is one
node and one operator-provisioned domain; global availability and self-service
onboarding have not been built or measured.

```text
Customer points their DNS (CNAME) at hakaishield
        │
        ▼
  our edge  →  terminates TLS for their domain (cert we issue)
            →  scores the request
            →  forwards clean traffic to their origin
        │
        ▼
  their origin server (unchanged, no code to install)
```

What this delivery model forces into the design, none of it optional:

| Requirement | Why |
|---|---|
| **Multi-tenancy everywhere** | The proxy has tenant-scoped stats, trail and origins; durable cross-node analytics and broader isolation tests remain future work. No query may cross the tenant boundary. |
| **Automated certificates (ACME)** | We terminate TLS for domains we don't own; certs must issue and renew without us touching them |
| **Bandwidth / request caps per plan** | Their traffic is now our bill. Without a cap, one large customer erases the margin on ten small ones |
| **Usage metering** | Billing needs it, and so does knowing which customer is costing what |
| **Uptime is now our problem** | Our outage is the customer's site being down. This is the single biggest operational change |

**Enterprise (self-hosted) stays supported** as a priced-up option for
customers who cannot send traffic to our cloud — regulated sectors,
data-residency requirements. It is *not* a separate product: it is the
same binary, run by them. Nothing in the codebase should assume we are
always the operator. **Don't build Enterprise tooling yet** — there
are no customers for it — but don't delete the single-tenant path as
dead code either; it is what Enterprise ships.

---

## System overview

```text
Internet (every visitor, hostile until scored)
   │
   ▼
[hakaishield]  ← terminates TLS, sits in front of the client's origin
   │
   ├── capture      BUILT   keep the raw TLS handshake  (proxy/capture.go)
   ├── fingerprint  BUILT   handshake → JA4 hash        (proxy/fingerprint.go)
   ├── proxy        BUILT   forward, strip spoofable headers (proxy/proxy.go)
   │
   ├── score        BUILT    combine signals → risk score (signals/score.go,
   │                         core/guard.go) — challenge/block/deceive.
   │                         Signals: fragmented_handshake, ua_mismatch,
   │                         header_anomaly, ja4_blocklist, scripting_tool,
   │                         velocity_spike, ja4_velocity_spike, crawl_pattern,
   │                         honeypot_trap
   ├── challenge    BUILT    JS challenge (challenge/challenge.go), now
   │                         triggered by score via Guard
   ├── deception    BUILT    rewrites HTML for deceived traffic only
   │                         (deception/deception.go, applied in
   │                         core/proxy.go ModifyResponse): tells automated
   │                         readers the page is not worth ingesting, and
   │                         plants the invisible honeypot link.
   │                         HTML 200s under 512 KiB only — anything else
   │                         streams through untouched.
   ├── honeypot     BUILT    records a fetch of the hidden trap path
   │                         (signals/honeypot.go) against
   │                         (tenant, IP, JA4), TTL'd and capped, feeding
   │                         the honeypot_trap signal. Per-node, in memory.
   ├── ratelimit     BUILT   Asset-aware per-IP velocity + crawl-pattern
   │                         detection (signals/velocity.go, signals/pattern.go)
   │
   ├──► Redis        BUILT   Tenant-scoped rate counters via INCR (velocity.go)
   │                         with a shared one-second, single-probe fail-open circuit;
   │                         background synchronization for JA4 blocklists (ja4db.go)
   ├──► Postgres     BUILT   Client configurations and lazy-loaded Tenant Store via pgxpool (pkg/db)
   │
   ▼
[Client's origin server]
   receives the request plus the headers in the contract below

[Dashboard]  React/Vite static app with Supabase Auth, wired to
             authenticated domain, stats and evidence endpoints.

[Mode]       BUILT  -mode enforce|shadow (proxy/mode.go) — shadow
             scores and records every request but forwards all of it,
             so a client can watch real traffic at zero risk. Surfaced
             in the startup log, /stats, every evidence record, and the
             dashboard (badge + banner + relabelled counters).

[Evidence]   BUILT  /api/v1/dashboard/evidence (proxy/evidence.go) —
             per-request record of why each decision was made, in a
             fixed 1000-entry ring buffer with a 24h retention window.
             Token-gated and off unless -evidence-token is set.

[Challenge state] BUILT  challenge tokens and passed cookies are host-bound.
             Solved challenge nonces are consumed through Redis when available,
             which rejects replay across nodes, and fall back to a bounded
             in-process store if Redis is unavailable.
             The in-process store evicts expired/oldest nonces in FIFO order
             without a full scan on each verify.
             Failed-attempt cookies carry a signed issue time with a server-
             enforced 15-minute expiry. Verify decodes a bounded 300x150 PNG
             and checks for nonblank pixels. It is still client-generated
             evidence, not proof of a human visitor.

[Observability] BUILT  aggregate operational counters for JWKS refresh/failure,
             unknown-kid rejection, Goodbot DNS budget rejection, Redis circuit
             opens/probes/skips, origin proxy errors, malformed forwarded IP
             headers, malformed Host, unknown Host, and SNI/Host mismatch.
             `/__hakaishield/observability` is mounted only when an
             observability bearer token is configured.
```

`core.Guard` evaluates request evidence and chooses a decision. In proxy shadow
mode it records that decision and forwards the visitor to the origin.

---

## What the origin receives — BUILT

This is the product's contract with the client's server, and with our
own future scoring code. Treat it as an API: changing it breaks both.

| Header | Meaning |
|---|---|
| `X-HakaiShield-JA4` | The connection's JA4 fingerprint, e.g. `t13d1516h2_8daaf6152771_e5627efa2ab1`. |
| `X-HakaiShield-JA4: unreadable` | TLS was present but the fingerprint could not be parsed; fragmentation is one possible cause. It is a signal to review, not proof of a bot by itself. |
| *(header absent)* | Not a TLS connection at all — hakaishield is running without `-tls-cert`, so there is nothing to fingerprint. |
| `X-Real-IP` | The real client address, set by us. |
| `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto` | Set by us from the real connection. |

The origin receives the visitor's protected `Host` by default. Operator-managed
split-host deployments may opt into `-origin-host-from-target=true`, which sends
the hostname from the configured target URL instead. This is needed when a
staging hostname fronts an origin with strict virtual-host routing; it is never
derived from visitor input.

**Every one of these is stripped from the inbound request before we
set our own value.** A visitor cannot forge any of them. That is not
a detail — a spoofable signal is worse than no signal, because the
scoring layer would trust it (`CLAUDE.md` Section 6).

---

## Tech stack

| Layer | Choice | Status | Why |
|---|---|---|---|
| **Core service** | Go | BUILT | Single static binary, low memory, high concurrency — it sits in every request's path |
| **Reverse proxy** | stdlib `httputil.ReverseProxy`, using `Rewrite` (not `Director`) | BUILT | `Rewrite` makes net/http strip the visitor's `X-Forwarded-*` headers; `Director` does not (`DECISIONS.md`) |
| **TLS termination + handshake capture** | stdlib `crypto/tls` + `fingerproxy`'s `pkg/hack` conn wrapper | BUILT | Go discards the raw handshake bytes after the handshake; JA4 needs them. `Accept` returns a real `*tls.Conn`, so net/http owns the handshake, its timeout, its error handling and its connection management |
| **JA4 computation** | `fingerproxy`'s `pkg/ja4` only | BUILT | Don't reinvent TLS parsing. Importing only this package keeps Prometheus and gopacket out of the binary (`DECISIONS.md`) |
| **Client-side automation probe** | JS within the challenge page | BUILT, challenge-only | Checks some automation tells only when an enforced challenge is served; ordinary page traffic has no site-wide probe. |
| **Fast state** (rate limits, session cache) | Redis | BUILT | Sub-millisecond reads with TTL; must not add latency per request |
| **Durable state** (tenant config and policy) | PostgreSQL | BUILT | Some customer configuration survives restarts; evidence and several aggregates remain in memory. |
| **Dashboard** | React/Vite static app (`dashboard/`) | BUILT for pilot | Supabase Auth and authenticated domain, evidence, stats, rules and settings API views; tenant policy has no dashboard editor. |
| **Deployment** | One TLS-terminating host for pilot | PACKAGED, not live-verified | Compose, Certbot setup and Redis are prepared; a real server, traffic and operational tests remain. |
| **Onboarding** | Customer CNAMEs their domain to us | planned | Replaces "install a Docker image": nothing for them to run, which is the whole point of hosting it |
| **Certificates** | ACME / Let's Encrypt, automated | planned | We terminate TLS for domains we don't own; manual certs don't scale past one customer |
| **Tenancy** | Tenant-scoped proxy and ownership-checked APIs | BUILT for pilot | Multiple tenant records are supported, while automated onboarding and durable cross-node telemetry are not. |
| **Billing** | Stripe subscription + usage metering | planned | Plans must carry bandwidth/request caps — their traffic is our bill |
| **Enterprise (self-hosted)** | Same binary, customer runs it | supported, not built | For customers who cannot send traffic to our cloud. Priced above hosted, sales-led. Don't build tooling for it yet; don't delete the single-tenant path either |
| **Metrics** | Prometheus client lib, off by default | planned | Optional; zero cost for clients who don't want it |

---

## How a request flows today — BUILT

1. `cmd/hakaishield` listens on `-addr`. With `-tls-cert`/`-tls-key` it
   wraps the listener in `proxy.NewCaptureListener`; without them it
   serves plain HTTP and no fingerprinting happens.
2. `Accept` wraps the raw connection so the handshake bytes are kept,
   then hands net/http a real `*tls.Conn` — **unhandshaked on
   purpose**, so the standard library performs the handshake with its
   own timeout (derived from `ReadHeaderTimeout`), its own error
   handling, and its own panic recovery.
3. `http.Server.ConnContext` puts the connection in the request
   context.
4. Per request, `JA4FromContext` reads the saved handshake off the
   connection and derives the fingerprint.
5. The proxy's `Rewrite` strips every spoofable identity header, sets
   the real ones, and forwards to the origin. The guard uses the direct peer
   by default; `-trusted-proxy-cidrs` is the explicit opt-in for reading
   `X-Forwarded-For` from a CDN/LB whose CIDRs are trusted.

---

## Why this shape (not something fancier)

- **Let the standard library do the work.** The capture listener
  deliberately owns as little as possible. Everything net/http
  already does — handshake timeouts, accept retries, panic recovery,
  connection handling — is its job, not ours (`CLAUDE.md` Section 24).
- **No Kubernetes, no microservices for v1.** One binary and two
  datastores covers a single-client or few-client deployment. Split
  only when a measured bottleneck proves it's needed.
- **No ML model in v1.** Rule/threshold scoring is explainable, fast,
  and good enough for naive-to-intermediate bots. Training a model on
  no data produces something worse than simple thresholds.
- **Explicit degraded behavior.** Missing JA4 does not create a fingerprint
  match; unreadable JA4 can contribute a bounded signal, while other evidence
  still decides. Proxy shadow mode forwards every request.

---

## Request path budget

Target: **under 15ms added to a passthrough request** (excluding the
JS-challenge path, which only suspicious traffic sees).

| Step | Budget | Measured |
|---|---|---|
| JA4 fingerprint | ~2ms | **14.3µs** — 0.7% of budget |
| Redis rate-limit check | ~2ms | built with 50ms cap and fail-open circuit; production p95/p99 not measured |
| Scoring (rule-based) | ~1ms | built; production p95/p99 not measured |
| Learned model (shadow, optional) | ~1ms | **14ns, 0 allocs** — off unless `-model` is set |
| Proxy overhead | ~5ms | not measured |
| Headroom | ~5ms | — |

The fingerprint is derived per request rather than cached per
connection. At 14.3µs that is deliberate: caching it would be
optimising 0.7% of a budget (`CLAUDE.md` Section 3 — measure first).
Re-measure before assuming this still holds.

If a layer can't hit its budget, it needs a timeout and a fail-open
fallback, not a slower default.

---

## Learned scoring, shadow only — BUILT

> New to this? `docs/SCORING_EXPLAINED.md` walks through it from zero with
> worked numbers. This section is the summary.

`pkg/signals` scores a request by adding fixed hand-chosen weights for each
check that fired and comparing the total to a fixed threshold. That is a linear
model whose coefficients nobody measured.

`pkg/decide` answers the same question from the same evidence with weights
fitted to labelled traffic. It is deliberately the smallest thing that can do
that:

```text
request
  ↓
signals.Evaluate        →  Evaluation{Score, Signals, Fired}
  ↓                          Fired is a bitmask: bit i = checks[i] fired
  ├── rules: Score ≥ 100 → block                (this is what actually runs)
  └── model: sigmoid(bias + Σ wᵢ·firedᵢ)        (recorded, never acted on)
                ↓
       Prediction{Decision, Probability, Confidence, Fired}
       Explain() → per-check contribution in log-odds
```

The bitmask and the signal names come out of the same loop over the same
`checks` list, so the model scores exactly the checks a customer is shown, and
the two can never describe different requests.

**Properties that are load-bearing, not incidental:**

- **Typed output.** `Predict` returns a `signals.Decision` from the existing
  enum. There is no string to parse, so an invalid decision cannot be produced.
- **Explainable.** `Explain` returns each fired check's exact push on the
  result in log-odds; the contributions plus the bias sum to the log-odds
  behind the reported probability. This is what answers "why was I stopped?"
- **In-process.** 14 ns and zero allocations per request — a dot product over
  a bitmask. No network call, no external service, no per-request API cost.
- **Stricter block bar than the rules.** The model will not return
  `DecisionBlock` unless at least two checks fired, however certain it is
  (`CLAUDE.md` Sections 10 and 14). A lone strong signal reaches
  `DecisionChallenge`.
- **Shadow only.** `-model <file>` loads a trained model. It scores alongside
  the rules and its opinion is recorded in the evidence trail as
  `Evidence.Model`; the decision the visitor experiences is always the rule
  scorer's. Agreements and disagreements are counted
  (`model_shadow_agree_total`, `model_shadow_disagree_total`) because the
  evidence trail is a bounded ring buffer a busy tenant overwrites in minutes.
- **A mismatched model is refused at startup.** Feature names and their order
  are part of the model file, and the fired-check vector is positional, so a
  model trained against a different check list would apply every weight to the
  wrong signal. Loading one is fatal rather than ignored.

`cmd/hakaishield-train` fits a model offline from labelled traffic, one JSON
object per line in the shape the evidence trail already records:

```text
{"signals":["ua_mismatch","header_anomaly"],"automated":true}
{"signals":[],"automated":false}
```

**Candidate observations are collected.** `-collect-labels` (with
`-db-url`) records challenge solves and first honeypot hits with source,
tenant and fired mask, but no IP, user agent, path or body. The bounded
queue drops rather than blocking visitors, and an identity cap limits
sample volume. Neither source is verified ground truth: client-provided
challenge fields can be forged, and prefetch or accessibility tools can
reach a trap. The trainer rejects database candidates by default; curated
`-in` labels are the safe input path. `-allow-unverified-labels` permits
shadow-only experiments. The model still never enforces; independent
labels, bias correction and held-out comparison remain open (items 25/26).

---

## Known architectural limits — BUILT code only

- **HTTP/1.1 only.** The capture listener does not offer h2, because
  HTTP/2 fingerprinting isn't built. Browsers fall back to HTTP/1.1.
  Adding h2 is now cheap — stdlib negotiation works precisely because
  `Accept` returns a real `*tls.Conn`.
- **A fragmented ClientHello can't be fingerprinted.** A handshake
  message split across TLS records defeats the capture (it reads one
  record). Reported as `unreadable` so it is visible, but it is not
  prevented. See `docs/RESEARCH.md`.
- **hakaishield must terminate TLS to see anything.** Behind a CDN or
  load balancer that terminates TLS first, there is no handshake to
  capture and no fingerprint — a deployment constraint, not a bug.
