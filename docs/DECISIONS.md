# Decisions — what we chose, and why

One or two lines each. Full reasoning: `git log -S "<title>" -- docs/`.
New entry format: `- **Choice** — why. Rejected: … (date)`

## Product

- **Hosted SaaS; self-hosted is priced Enterprise** — customer installs
  nothing. Cost: their traffic is our bill, our outage is their outage.
  Keep the single-tenant path; no Enterprise tooling yet. (09-16)
- **Not "cheap DataDome"** — the floor is free (CrowdSec, Cloudflare free).
  We win on first-request TLS scoring with checkable reasons. (09-16)
- **Moat = maintained browser fingerprint database**, not the code. (09-16)
- **Closed-source, proprietary.** No LICENSE granting rights. (09-14)
- **MVP scope: naive-to-intermediate bots.** Residential proxy networks
  are fought by fingerprint and aggregate rate, not IP; the goal is
  raising attacker cost, not 100%. (09-14)
- **Managed pilot: one operator-provisioned domain**, self-serve domain
  creation off until ownership proof + auto TLS exist. (09-24)
- **No DNSBL/IP reputation for the pilot** — cost, false positives on
  shared IPs. Revisit only if labelled misses show a gap. (09-24)
- **Removed fake billing UI** instead of leaving it as a known gap. (09-21)

## Detection

- **Multi-signal, additive score; block at 100.** Only `ja4_blocklist`
  and `scripting_tool` (verified on real captures) may block alone. (09-15)
- **Unreadable handshake is a signal** (`unreadable`), not "no
  fingerprint" — fragmentation evasion becomes visible. (09-14)
- **UA check is structural**, not a browser database — until the
  database exists (item 19). (09-14)
- **Endpoint-class velocity buckets** (login 10, API 100, checkout 20,
  nav 20, assets 300 /s) with one shared path classifier. (09-25)
- **Honeypot scored, not blocklisted** — keyed (tenant, IP, JA4); JA4
  alone is too coarse, IP alone hits CGNAT. 6 h TTL, 50k cap. (09-20)
- **Browser probes and client hints stay observation-only** — client
  claims, can be forged, need measured false positives first. (09-24)
- **Good bots verified by reverse+forward DNS**, exact suffix match.
  Web Bot Auth (RFC 9421) is the future path. (09-19)
- **Automation probe lives in the challenge page**, not injected
  site-wide (bytes and CSP cost on every page). (09-16)
- **Rejected:** crash-the-browser traps, debugger hooks, making pages
  heavier to tax bots (our egress, hurts real users). (09-20)

## Challenge

- **SHA-256 PoW + canvas proof**, not math-only. (09-15)
- **Server-chosen difficulty 1–3**, signed in the token, capped for
  phones. Anubis showed fixed difficulty loses over time. (09-23)
- **PNG bounded and validated** (300×150, non-blank, size cap);
  failed-attempt cookie has a server-enforced 15 min expiry. (09-23)
- **Passed cookie ≠ trusted forever** — every later request is rescored;
  hard-block evidence still blocks. (09-24)
- **Stable ≥ 32-byte secret; nonces in Redis `noeviction` + AOF**, never
  evict a live nonce to admit a new one. (09-24)

## Proxy and infrastructure

- **Go, one binary.** No microservices or Kubernetes until measured. (09-14)
- **net/http owns the TLS handshake**; our listener only keeps the
  bytes. Deleted our hand-rolled timeout/retry/panic code. (09-14)
- **`ReverseProxy.Rewrite`, not `Director`** — Rewrite strips visitor
  `X-Forwarded-*`. Also strip every other client-IP header. (09-14)
- **Import only fingerproxy `pkg/ja4`** — avoids Prometheus/gopacket. (09-14)
- **h2 offered and fingerprinted, weight 0.** Preface read bounded by
  `ReadHeaderTimeout`, 16 frames, 16 KiB/frame; clients that never send
  the preface are dropped, preconnects are served. One shared
  `http2.Server` via `ConfigureServer`, or `Shutdown` hangs on open h2
  conns. `ServeTLS`/`Protocols` rejected: they redo TLS and lose the
  ClientHello. (09-28)
- **XFF only behind `-trusted-proxy-cidrs`**, rightmost untrusted hop. (09-22)
- **Redis signals fail open via one shared circuit** (1 s, one probe). (09-21)
- **Unknown-host DB lookups: 8 concurrent, same host shared**, overflow
  answered not-found and not cached. (09-25)
- **Unknown JWT `kid`: short negative cache, serialized JWKS refresh** —
  forged tokens can't force outbound calls. (09-22)
- **Supabase Auth**, not our own bcrypt/JWT. (09-21)
- **Bare-path routes, handlers check method** (Go 1.22 patterns dropped). (09-21)
- **Origin Host rewrite is opt-in** (`-origin-host-from-target`), for
  split-host staging whose origin routes by vhost; default keeps the
  visitor Host. (09-28)
- **CI: gosec, errorlint, bodyclose, nilerr, sqlclosecheck, govulncheck,
  20 s fuzz.** Rejected: nilaway (17 findings, 0 real), contextcheck/noctx,
  CodeQL (paid for private), per-PR ClusterFuzzLite (minutes). (09-25)

## Detection additions

- **Crawler UA claims don't skip `crawl_pattern`**; only DNS-verified
  crawlers are exempt. Unverified crawlers >60 pages/min may be
  challenged under enforce — review in shadow. (09-25)
- **Stealth-browser signals are `shadowSignals`, not weight-0 checks.**
  Keeps `FeatureVersion` stable and no model/sample break; promote one
  to a scored check only after its FP rate is measured. (09-29)
- **Datacenter: offline ip2asn table + 16 hosting ASNs**, loaded once at
  startup (~14k ranges, 0.7 MB, 0.9 s). No external lookup per request.
  VPN hosts (M247) and Cloudflare (WARP) excluded: real people. Azure/
  Google corporate egress will still fire. (09-29)
- **h2 family check uses only verified captures**: Chrome 153, Edge 154,
  Patchright Chrome (`masp`), Firefox 142 (`mpas`). Safari not captured,
  so not checked. Patchright greets exactly like Chrome — this catches UA
  spoofers only. (09-29)
- **Session key = tenant + IP + UA mod 16.** A full UA hash let one IP
  mint a Redis key per request by rotating UA; Redis is `noeviction`, so
  that could break challenge replay storage. (09-29)
- **Beacon sends five booleans on pagehide**, nothing else: no timings,
  coordinates, keys or IDs. Forgeable, so it never grants trust; faking
  input defeats `beacon_no_interaction` (verified with Playwright). (09-29)
- **Form POSTs don't count as page loads**; otherwise a direct login
  POST with `Sec-Fetch-Dest: document` would excuse itself. (09-29)

## Policy and data

- **Tenant policy = append-only versioned snapshots**, owner + version
  checked under row lock; old account-wide rules stay shadow. (09-23)
- **Route labels only act after policy activation.** (09-25)
- **Evidence in memory, token-gated, 1000 entries / 24 h.** (09-16)
- **Egress bytes and challenge burden are per-tenant stats.** (09-25)

## Learned scoring

- **Linear model over existing checks**, not a new model or an LLM. (09-22)
- **Auto-collected labels are candidates, not ground truth.** (09-23)
- **Model v2 needs provenance; promotion only if holdout beats rules.** (09-25)
- **Never auto-retrain** — it gives attackers a write channel. (09-22)

## Deployment

- **DigitalOcean Bangalore**, origin in the same region. (09-23)
- **Nothing that terminates TLS in front of us.** (09-22)
- **Dashboard on Cloudflare Pages** (Direct Upload, Node 24, build gate on
  Supabase/API env); marketing-only until the backend API is live. Pages is
  never in front of the proxy. See `dashboard/FRONTEND.md`. (09-25)
- **Sideband decision API when bandwidth dominates** (item 27). (09-22)

## Threats we have studied (research memory)

- **Patchright / Scrapling (real patched browser)** — passes JA4, UA,
  headers and `webdriver` probes. Only behavioural timing and a browser
  fingerprint DB close it. Patchright admits init-script timing is open.
- **ClientHello split across TLS records** — one line for an attacker;
  shows as `unreadable`. Real fix is reassembly (upstream fingerproxy).
- **Residential proxy pools** (72M+ IPs) — IP reputation useless; use
  fingerprint + cross-IP aggregate rate.
- **Vendors reach high block rates by stacking 5+ layers** and
  continuous trust (Kasada re-solves PoW every 60–180 s).
- **AI agents** — market moved from "bot or not" to "which agent, is it
  allowed, prove it". Mid-market has only allow/block today (item 11b).
- **Useful open source (read for ideas, MIT):** bot-signal (FP carve-out
  list), go-away (Go rule engine), FCaptcha (Web Bot Auth), Anubis
  (fixed PoW failure). No code copied; porting code needs `NOTICES.md`.
