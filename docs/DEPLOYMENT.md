# Deployment — where, what it costs, how to launch

Step-by-step server runbook (AWS/EC2, Compose, Certbot, staging):
`deploy/RUNBOOK.md`. Dashboard build and Pages: `dashboard/FRONTEND.md`.
Marketing site is live on Cloudflare Pages at `interviewyaar.lol`.

Prices checked 2026-09-22/23. Re-check before trusting them.
Step-by-step commands: `deploy/README.md`.

## The one rule

We read the raw TLS ClientHello, so **we must terminate TLS ourselves**.
Port 443 must reach our process unterminated.

Anything that terminates TLS first gives us its handshake, not the
visitor's. JA4 becomes a constant, `ua_mismatch` and `ja4_blocklist` go
quiet, and nothing fails loudly. It just gets worse.

| Platform | Verdict |
|---|---|
| Vercel, Netlify, Railway | No — terminate TLS |
| Cloudflare proxied (orange cloud) | No — use DNS-only (grey cloud) |
| AWS ALB, CloudFront | No — terminate TLS |
| AWS NLB in **TCP** mode | OK if a load balancer is ever needed |
| Fly.io with TCP passthrough | OK |
| Any VPS with a public IP | OK — this is the requirement |

The dashboard is static and not in the request path, so it can live on
Vercel or Cloudflare Pages for free.

## Where: DigitalOcean Bangalore (decided 2026-09-23)

| | Latency (India) | Included | Overage | 4 GB box |
|---|---|---|---|---|
| **DO Bangalore** | 5–40 ms | 4 TB | ~$10/TB | $24/mo |
| Hetzner Singapore | 55–70 ms | 0.5–1 TB | ~$8/TB | ~€13/mo |
| Hetzner EU | 130–180 ms | 20 TB | ~$1/TB | ~€7.5/mo |
| AWS Mumbai | 5–40 ms | 100 GB | ~$92/TB | ~$15/mo + egress |

Same cost as Hetzner Singapore, but in India. Gotcha: Hetzner's famous
€1/TB is the EU rate; Singapore is €7.40/TB. Always check the region's
own rate.

**Rule: the origin must sit in the same region as the proxy.** Otherwise
the visitor pays the distance three times, not once.

Setup: 2 vCPU / 4 GB droplet, Ubuntu LTS. Redis in Docker on the box
(no public port). Postgres on Supabase. Certbot with renewal hook.
Firewall: 443, 80 (renewal), 22 (your IP only). No LB, no CDN in front.

Moving providers is an afternoon: new box, DNS, `setup.sh`.

## Cost: bandwidth is the business model

We are a full reverse proxy, so every response byte leaves our box and
we pay egress on it. DataDome and Akamai never touch the body (sideband
call or same-hop edge module).

At 100 KB per response: 10M req/mo ≈ 1 TB (~$10 on DO, ~$81 on AWS);
1B req/mo ≈ 100 TB (~$1,000 on DO, ~$6,900 on AWS).

Fine into tens of millions of requests. Beyond that the fix is
architecture, not a cheaper box: a sideband decision API (item 27).

Ways to cut the bill, biggest first:
1. Block early — a 100-byte block instead of a 100 KB page.
2. Keep responses compressed; never decompress and resend.
3. On AWS: no NAT Gateway, one AZ, Graviton.
4. Billing alarm before the first visitor.

No per-request paid calls in the hot path: no LLM, no external
reputation API, no remote inference, no DNSBL lookup.

Durable evidence (`-durable-evidence`) is the one optional extra cost:
one small Postgres row per decision, batched off the request path. It is
storage and write IO, not bandwidth — set `-evidence-retention-days` and
budget for it. Reads still come from the in-memory ring, so the flag is
safe to leave off until a client needs history that survives a restart.

## Launch a pilot

1. Point the domain's A record at the server **before**
   `deploy/setup.sh` (Certbot HTTP-01 needs it).
2. `deploy/.env` from the example. Secrets via `openssl rand -hex 32`.
   Never commit it. Validate:
   `docker compose --env-file deploy/.env -f deploy/docker-compose.yml config --quiet`
3. Start with `HAKAISHIELD_MODE=shadow`. Keep `-collect-labels` **off**
   while you attack your own site, or the model learns your test bots.
4. Check: JA4 varies by client, Host routing, real browser journeys,
   health, Redis, evidence auth, restart, `certbot renew --dry-run`.
5. Run `bot-testing/ladder/ladder.py` only against your own site.
6. Enforce only after a reviewed shadow sample and client sign-off.
   Keep the previous image and mode ready for rollback.

### Bind the client's dashboard account

A client can now register a domain and prove ownership themselves: the
dashboard creates a `pending_verification` row, the client publishes a
`_hakaishield.<domain>` TXT record, and the verify endpoint flips it to
`verified` (token cleared). Verified is not protected — TLS, the DNS A
record and activation are still operator work until item 21's ACME
lands. The signed-in client may already own that row, in which case only
the manual activation below is needed. Otherwise create the Supabase
Auth user, bind ownership, and on a fresh pilot database:

```sql
INSERT INTO public.tenants
    (id, host, target, mode, owner_user_id, name, status)
VALUES
    ('default', 'customer.example', 'https://origin.example', 'shadow',
     '<Supabase Auth user ID>', 'customer.example', 'active');
```

An existing `default` row or host conflict means stop and inspect; never
overwrite. Restart the proxy afterwards; it checks the row matches
`HAKAISHIELD_DOMAIN` / `HAKAISHIELD_ORIGIN` and loads the owner.

## Known limits of the pilot setup

- One node only: evidence and shadow stats are in memory; Redis outage
  falls back to per-node nonces (cross-node replay not prevented).
- Redis AOF `everysec` can lose the last second of nonces on a crash.
- Nothing here has run against live traffic yet.
