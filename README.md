<p align="center"><b>hakaishield</b></p>

# hakaishield

**HakaiShield** is an inline bot-traffic analysis and protection proxy. It
records why it would allow, challenge or block each evaluated request. The
current release is a managed, single-domain pilot; real-client detection and
availability are not measured yet.

Whether you're fighting credential stuffing, scalpers, aggressive scrapers, or API abuse, HakaiShield sits directly in the request path and evaluates the very first request from any client using advanced JA4 TLS fingerprinting.

### Key Capabilities:
- **TLS Fingerprinting (JA4):** Reads the ClientHello directly and combines it with request evidence; a fingerprint is evidence, not a guaranteed bot identity.
- **Scoring Engine:** Produces an explainable allow, challenge, rate-limit, deceive or block decision.
- **Shadow Mode Testing:** Records proposed decisions while forwarding visitor traffic to the origin.
- **Evidence-Based Decisions:** Authenticated stats and evidence endpoints show which signals contributed to a decision.
- **Self-serve domain registration:** Signed-in customers add a domain and prove ownership with a `_hakaishield.<domain>` DNS TXT record; an operator completes TLS/DNS cutover and activates it. Automatic ACME onboarding and multi-region availability are future work.

> **Deploying it?** [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) — hakaishield
> terminates TLS itself to read the ClientHello, so any platform that
> terminates TLS first silently degrades detection. That document says which
> ones those are, and what bandwidth costs as traffic grows.
>
> **New here, or need to explain this to someone?**
> [`docs/STATUS.md`](docs/STATUS.md) says what works, what does not, and
> what comes next.

## Why this exists

Bot protection today comes in two shapes, and neither fits a mid-size
company:

- **Enterprise** (Akamai, DataDome, HUMAN, Kasada): $1,500–$50,000+/
  month, quote-only, and your traffic runs through their cloud.
- **Free and self-hosted** (CrowdSec, Coraza, ModSecurity): genuinely
  free, but they parse **server logs** — they react to an IP *after*
  it has already misbehaved somewhere, and lean on shared blocklists.

hakaishield is neither. It sits in the request path, reads the live TLS
ClientHello, and scores the **first request** from a client it has
never seen — then tells you exactly why it decided what it decided.

That matters most when bots cost you money directly rather than just
noise: scraped pricing, hoarded ticket inventory, copied listings,
usage-billed API calls.

**Enterprise:** if you can't send traffic to someone else's cloud
(regulated sector, data-residency rules), the same product runs in
your own infrastructure. That's a contract, not a signup — talk to
us.

hakaishield is closed-source, commercial software (a paid service —
see License below). Internally it uses proven open-source
*libraries* (TLS/JA4 fingerprinting, behavioral scoring, JS
challenges) instead of reinventing hard, already-solved problems.

## What it is

hakaishield is one product: a reverse proxy + dashboard that sits in
front of your site. We run it — you point DNS at us and there is
nothing to install. (Enterprise customers run the same binary
themselves.) Internally the code is split into small, focused packages
(fingerprinting, scoring, challenge, etc.) for the usual reasons —
easier to test, easier to read, easier to fix — not because it's
meant to be reused elsewhere.

## Try it locally

The hosted service is how customers use hakaishield. The commands below
run the same binary on your own machine for development.

```bash
cd backend && go build -o hakaishield .

# plain HTTP — proxies traffic, no fingerprinting (nothing to capture)
./hakaishield -addr :8080 -target http://127.0.0.1:9000

# with TLS — terminates TLS and fingerprints every connection
./hakaishield -addr :8443 -target http://127.0.0.1:9000 \
            -tls-cert cert.pem -tls-key key.pem
```

| Flag | Meaning |
|---|---|
| `-addr` | Address to listen on (default `:8080`) |
| `-target` | The origin server to protect, e.g. `http://127.0.0.1:9000` |
| `-tls-cert`, `-tls-key` | Your certificate and key. **Fingerprinting only works with these** — hakaishield has to terminate TLS to see the handshake. |
| `-evidence-token` | Bearer token for the per-request evidence endpoint. Leave it unset and that endpoint does not exist at all. |
| `-mode` | `enforce` (default) acts on scores. `shadow` scores and records everything but blocks nothing — see below. Any other value refuses to start. |
| `-collect-labels` | Collect candidate labels from solved challenges and honeypot hits for later review. Needs `-db-url`. Records only which checks fired — no IP, user agent, path or body. Off by default. |
| `-durable-evidence` | Persist the evidence trail to `-db-url` so it survives a restart. Off by default: it adds a batched (off the request path) database write per decision. Reads still come from the in-memory ring, which is hydrated on startup. Needs `-db-url`. |
| `-evidence-retention-days` | Delete durable evidence older than this many days, 1–365. Used only with `-durable-evidence`. |
| `-model` | A trained decision model (see below) to score alongside the rules. It records what it would have decided and never affects a decision. Unset leaves it off. A model that does not match this build's checks refuses to start. |
| `-db-url` | PostgreSQL URL for your Supabase project's database (Project Settings → Database in the Supabase dashboard). Required, along with `-supabase-url`, for the dashboard's domains/rules/settings API. |
| `-supabase-url` | Your Supabase project URL (e.g. `https://xxxx.supabase.co`). Used to verify dashboard session JWTs against that project's published JWKS — no shared secret needed. Required, along with `-db-url`, for that same API. |

| Env var | Meaning |
|---|---|
| `SENTRY_DSN` | Optional. A handler panic is always recovered and logged either way (the process never crashes); setting this also reports it to Sentry so it surfaces as an alert instead of a line in a log nobody is watching. Unset by default — no signup required to run hakaishield. |

### Shadow mode

`-mode shadow` runs the full scoring pipeline and records what it
*would* have done, while forwarding every request to your origin
untouched. Nothing your visitors do can be broken by a score while it
is on, which makes it the safe way to see what hakaishield finds in
your real traffic before enforcing anything.

It is deliberately hard to miss that it is on: a startup log line,
`"mode":"shadow"` on every stats response, `"enforced":false` on every
evidence record, and in the dashboard a status badge plus a banner —
with the counters relabelled "Would block" / "Would challenge" /
"Would pass".

Two read-only endpoints are served alongside your traffic:

| Endpoint | What it gives you |
|---|---|
| `GET /api/v1/dashboard/stats` | Running totals: requests seen, passed, challenged, blocked, plus `mode` and `enforcing` so the counts can't be read out of context. No per-visitor data, so it needs no token. |
| `GET /api/v1/dashboard/evidence` | The last 1000 decisions (24h max), newest first: timestamp, JA4, which signals fired, score, decision, and whether it was `enforced`. Accepts `?limit=N`. **Requires `Authorization: Bearer <-evidence-token>`.** |

### Learned scoring (shadow only)

> Working on this? Read "Learned model" in
> [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) first.

hakaishield's scoring weights are chosen by hand: a fragmented handshake
is worth 50, a header anomaly 25, and so on. Those are reasonable
guesses, but they are guesses.

`-model` loads a model that answers the same question from the same
signals with weights fitted to operator-reviewed labelled traffic. It returns a
typed decision, an estimated probability, a confidence, and a breakdown
of exactly what each signal contributed — so it stays as explainable as
the rules it sits beside.

It costs 14 nanoseconds and zero allocations per request: it is a dot
product over the checks that already ran, in-process, with no network
call and no per-request API charge.

**It never decides anything.** It scores alongside the rule engine and
its opinion is recorded in the evidence trail under `model`, next to the
decision your visitor actually got. That is how a model earns the right
to enforce — by being compared against the rules on your real traffic
first. It also will not block on a single signal, however certain it is.

**Collecting candidate data.** `-collect-labels` accumulates it from
your own traffic, with no work on your part:

```bash
./hakaishield -target https://example.com -db-url "$DATABASE_URL" -collect-labels
```

A solved challenge is a human *candidate*: its canvas and automation fields
are client supplied and can be forged. A honeypot hit is an automated
candidate; prefetch and accessibility tools can also hit the trap. Only which
checks fired is stored: no IP, user agent, path or body. An identity is capped
at five samples per hour, which limits volume but does not verify a label.

Collecting costs about 100 nanoseconds per labelled request and never
blocks a visitor: samples go onto a bounded queue and are dropped, and
counted, rather than making someone wait on a database.

**Training** requires a curated JSONL file with independently checked labels:

```bash
go run ./cmd/hakaishield-train -in verified-labels.jsonl -out model.json
./hakaishield -target https://example.com -model model.json
```

Use one JSON object per line in the shape the evidence trail records:

```bash
# {"signals":["ua_mismatch","header_anomaly"],"automated":true}
# {"signals":[],"automated":false}
go run ./cmd/hakaishield-train -in labelled.jsonl -out model.json
```

The trainer holds back a fifth of the data and reports how the model did
on traffic it was *not* trained on, along with false positives and false
negatives separately — a model that does well on the data it was fitted
to and poorly on the rest has memorised your sample, not learned your
traffic.

The hard part is the `automated` label, not the training. It has to come
from something that actually knows, independently of hakaishield's own
score — labelling from that score would only teach the model to repeat
the guesses it exists to improve on.

The database candidates can be used for shadow-only experiments with
`-allow-unverified-labels`; the trainer refuses them by default. A verified
good-bot lookup is not a usable training label either. The collection
pipeline and the bar a model has
to clear before it may decide anything are in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

The evidence endpoint is off unless you set a token, and it never gets
wildcard CORS — it returns visitor fingerprints, and left open it would
also tell a bot whether its own fingerprint is being flagged. Run it
over TLS; on a plain-HTTP deployment the token travels in the clear.

Your origin then receives each request with:

| Header | Meaning |
|---|---|
| `X-HakaiShield-JA4` | The caller's JA4 fingerprint, e.g. `t13d1516h2_8daaf6152771_e5627efa2ab1` |
| `X-HakaiShield-JA4: unreadable` | TLS, but the handshake couldn't be read — unusual, and worth treating as suspicious |
| *(absent)* | Not a TLS connection, so there was nothing to fingerprint |
| `X-Real-IP`, `X-Forwarded-For` | The real caller's address |

hakaishield strips all of these from the incoming request before setting
its own, so a visitor can't forge them.

## Dashboard (frontend)

`dashboard/` is a React + Vite UI (19 pages: marketing, auth, billing,
and the operator dashboard itself — evidence logs, mitigation rules,
protection settings, domains).

**Auth runs on Supabase.** Signup, signin, sign-out, email
verification, and password reset all go straight from
`dashboard/src/lib/supabaseClient.ts` to Supabase Auth — this backend
never sees a password. It only verifies the session JWT Supabase
already issued (`pkg/auth`, against that project's published JWKS)
before serving domains, mitigation rules, protection settings, live
dashboard stats, top-offender JA4s, and evidence logs over
`dashboard/src/lib/api.ts`.

See `docs/DECISIONS.md`'s 2026-09-21 Supabase-migration entry for
exactly what's real vs. still a placeholder — in short: payments,
SIEM export, WAF toggles, and the traffic-over-time chart are not
built. Those sections say so in the UI rather than showing fake data.

```bash
cd dashboard
npm install
npm run dev        # local dev server — set VITE_SUPABASE_URL, VITE_SUPABASE_ANON_KEY,
                    # and VITE_API_BASE_URL (see .env.example)
npm run typecheck  # tsc --noEmit
npm run build      # production build
```

The backend side needs `-db-url` (your Supabase project's Postgres
connection string) and `-supabase-url` set to serve the domains/rules/
settings API at all (see `docs/PROGRESS.md`'s Supabase-migration
entry); without both, `hakaishield` still runs as a proxy, just
without that API. Both also read from `$DATABASE_URL`/`$SUPABASE_URL`
if the flags are left unset, including from a `backend/.env` file
(gitignored — copy `backend/.env.example`) loaded automatically at
startup, so you don't have to pass them on the command line every run.

## Documentation

Five files in `docs/`: `STATUS.md` (start here), `ARCHITECTURE.md`,
`DEPLOYMENT.md`, `DECISIONS.md`, `PROGRESS.md`.
`CLAUDE.md` holds the enterprise stability guidelines every change must follow.

## Responsible use

hakaishield is a defensive security tool. It is built to protect
websites from unwanted automated traffic — it is not, and will never
include, tooling whose purpose is to help automation evade detection.

## License

**Proprietary — All Rights Reserved.** hakaishield is closed-source
commercial software. No license to copy, modify, distribute, or use
this code is granted except as agreed directly with the owner. It is
not an open-source project, even though it uses open-source libraries
internally (see below).
