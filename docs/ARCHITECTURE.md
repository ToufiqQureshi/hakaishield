# Architecture — how a request becomes a decision

Code is the source of truth: `backend/pkg/signals/score.go` holds the
`checks` table; `backend/pkg/core/guard.go` runs the request path.

## Shape

```text
Visitor ──TLS──► hakaishield (one Go binary) ──► customer origin
                   │
                   ├─ capture   keep raw ClientHello      core/capture.go
                   ├─ JA4       handshake → fingerprint   fingerproxy pkg/ja4
                   ├─ tenant    Host → tenant             tenant/tenant.go
                   ├─ score     9 checks → risk score     signals/score.go
                   ├─ policy    tenant policy, shadow     policy/, policyprovider/
                   ├─ act       allow/challenge/rate-limit/deceive/block
                   └─ evidence  why, per request          evidence/
                   │
                   ├─► Redis     rate counters, nonces, JA4 lists
                   └─► Postgres  tenants, policy, labels (Supabase)

Dashboard: static React app + Supabase Auth → authenticated /api/v1.
```

## Request flow

1. Listener wraps each connection to keep the ClientHello, then hands
   net/http a real `*tls.Conn`. net/http owns handshake, timeouts and
   panics. Only `http/1.1` is offered.
2. `Rewrite` (not `Director`) strips every client-IP header the visitor
   sent (`X-Forwarded-*`, `X-Real-IP`, `CF-Connecting-IP`…) and sets our
   own. `X-Forwarded-For` is read only behind `-trusted-proxy-cidrs`,
   rightmost untrusted hop.
3. Host → tenant. Unknown hosts: max 8 concurrent DB lookups per node,
   same host shares one lookup, misses cached 30 s.
4. Score, apply tenant policy, act, record evidence.
5. In `shadow` mode everything is scored and recorded, all forwarded.

## What the origin receives

| Header | Meaning |
|---|---|
| `X-HakaiShield-JA4` | JA4 of the visitor's handshake |
| `X-HakaiShield-JA4: unreadable` | TLS present but ClientHello unparseable |
| absent | plain HTTP, nothing to fingerprint |
| `X-Real-IP`, `X-Forwarded-*` | set by us; visitor values stripped |

The origin gets the visitor's `Host` by default. For split-host setups the
operator may set `-origin-host-from-target=true` (Compose:
`HAKAISHIELD_ORIGIN_HOST_FROM_TARGET`) to send the target URL's host instead;
never derived from visitor input.

Treat these as an API. Changing them breaks customers.

## The nine scored checks

| Check | Weight | Fires when |
|---|---|---|
| `fragmented_handshake` | 50 | JA4 `unreadable` |
| `ua_mismatch` | 50 | browser UA but TLS 1.0/1.1, unreadable, or scraper JA4 |
| `header_anomaly` | 25 | browser UA but no `Sec-Fetch-*` / `Sec-CH-UA*` |
| `ja4_blocklist` | 100 | JA4 on the verified scraper list |
| `scripting_tool` | 100 | UA names itself (curl, python-requests, …) |
| `velocity_spike` | 50 | per-IP rate over its bucket (see below) |
| `ja4_velocity_spike` | 50 | one non-browser JA4 > 50 req/s across IPs |
| `crawl_pattern` | 50 | > 60 distinct pages in 60 s (HyperLogLog) |
| `honeypot_trap` | 50 | fetched the hidden trap link, (tenant, IP, JA4), 6 h |

Velocity buckets per second: login 10, API 100, checkout 20, nav 20,
assets 300.

Decision: score ≥ 100 block (or deceive if tenant opted in); 1–99
challenge; 0 allow. Strict policy challenges everything below 100.

Only `ja4_blocklist` and `scripting_tool` can block alone. Both were
verified against real captures. Everything else needs a second signal.

Not scored, recorded as `shadowSignals` only: Chromium client-hint
contradictions; after an enforced solve, WebGPU f16 absence, repeated
canvas output, pointer inactivity, legacy automation globals.

False-positive guards: verified search crawlers (reverse+forward DNS,
6 h cache, 64 concurrent lookups); honest `bot`/`spider` UAs skip
browser-impersonation checks; common-browser JA4s skip JA4 velocity.

Redis failure: all rate/crawl checks fail open together via one shared
circuit (1 s, single probe). Know that when reading a quiet dashboard.

## Challenge

- Server picks difficulty 1–3 leading hex zeros from the score; it rides
  inside the HMAC-signed token, so the client cannot lower it. Capped at
  3 to stay safe on mid-range phones.
- Client returns SHA-256 answer + 300×150 canvas PNG (size-capped,
  must be non-blank). Plus automation probes (`webdriver`, `__pwInitScripts`).
- Nonces are single-use via Redis (`noeviction`, AOF); local bounded
  fallback if Redis is down. Verify is capped at 64 concurrent (503 over).
- A passed cookie skips repeat challenges but every request is still
  scored. Fresh hard-block evidence still blocks; velocity can rate-limit.
- A solve is **not** proof of a human. Any client can compute PoW.

## Deception and honeypot

Deceived traffic gets HTML (200, < 512 KiB) rewritten to discourage
ingestion, with an invisible trap link. A fetch of the trap is evidence,
not a blocklist write. Per node, in memory, capped at 50k.

## Tenant policy

Append-only versioned snapshots per domain. Every write checks owner and
expected version under a row lock. Preview, rollback, shadow summary and
an activation gate exist. Route labels (≤ 64 exact paths) move a path into
the login/checkout velocity bucket only after activation.

## Learned model (`pkg/decide`) — shadow only

- Same 9 checks as a bitmask → logistic regression → probability,
  typed decision, per-check contribution. 14 ns, 0 allocs, in-process.
- Rules decide. The model's opinion is only recorded (`-model`).
- Never blocks on one signal. Feature list is stored in the model file;
  a mismatch is fatal at startup (the mask is positional).
- Artifact v2 carries provenance: check-list hash, dataset hash, options,
  holdout summary, approver.
- Training is a manual command, never automatic. A bot can farm challenge
  solves to inject "human" labels; auto-retrain gives it a write channel.
- Labels: challenge solves are human *candidates*, honeypot hits are bot
  *candidates*. Max 5 samples per (tenant, IP, JA4) per hour. Trainer
  rejects DB candidates unless `-allow-unverified-labels`. Samples pruned
  hourly (default 30 days).
- Traps: never label from our own score (model copies our guesses);
  strip `honeypot_trap` from samples it labelled (model learns the label
  back); judge on the time-split holdout, not training numbers.
- Before it may enforce: independent labels, bias decision made
  (`STATUS.md`), holdout beats the rule scorer at equal or lower human
  harm, disagreements reviewed by a person.

## Latency budget

Target under 15 ms added per passthrough request. Measured: JA4 14 µs,
model 14 ns. Redis capped at 50 ms with fail-open. Proxy overhead and
production p95/p99 not measured yet.

## Known limits

- HTTP/1.1 only (no h2 fingerprint yet).
- A ClientHello split across TLS records cannot be fingerprinted; it
  shows as `unreadable` and scores, but is not reassembled.
- Anything that terminates TLS in front of us (CDN, ALB) silently turns
  JA4 into a constant. See `DEPLOYMENT.md`.
- Evidence, honeypot and shadow stats are per node, in memory.
