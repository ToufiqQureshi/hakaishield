# Deploying hakaishield

The reasoning — why this host, why not the managed ones, what bandwidth
costs — is in [`../docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md). This file is
the commands.

**Read "The one rule" in `DEPLOYMENT.md` before substituting any platform.** hakaishield
terminates TLS itself to read the ClientHello. Anything that terminates TLS
first (Cloudflare proxied, Railway, Vercel, an ALB) silently turns JA4 into a
constant and quietly guts detection. Nothing errors.

---

## What goes where

```text
One Linux server near the protected origin; size and price are chosen when
the pilot domain and traffic estimate are known. Keep the ORIGIN nearby to
avoid an extra long network hop (see "Where" in docs/DEPLOYMENT.md).
├── hakaishield   :443, in Docker
└── Redis         no published port; only hakaishield reaches it

Supabase        Postgres. Managed backups, and a dead box does not
                take the customer data with it.
Vercel / Pages  the dashboard. Static files, free, not in the request
                path — so TLS termination there does not matter.
```

Do not self-host Postgres on the same box. Redis also holds spent challenge
nonces, so Compose uses a persistent AOF volume and `noeviction`. Back up and
monitor that volume; see `../docs/DEPLOYMENT.md` for outage limits.

---

## First time

```bash
# Point the domain's DNS A record at this box before requesting a certificate.
# On a fresh box, as root:
git clone <repo> /opt/hakaishield
cd /opt/hakaishield
bash deploy/setup.sh customer.example you@example.com
```

That installs Docker and certbot, sets a default-deny firewall open on 22/80/443,
issues the certificate, copies it into a directory readable by the container's
non-root GID 65532, installs a renewal hook that refreshes that copy before
restarting hakaishield, and runs Certbot's renewal dry-run. The dry-run checks
certificate renewal; the initial copy checks the hook's file-permission path.

Then:

```bash
cp deploy/.env.example deploy/.env
$EDITOR deploy/.env                 # Set domain/origin and generate both tokens with openssl rand -hex 32
docker compose --env-file deploy/.env -f deploy/docker-compose.yml config --quiet
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --build
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f
```

Set `DATABASE_URL` and `SUPABASE_URL` as well: the client dashboard API is part
of this pilot and Compose fails if either is missing. After TLS/origin checks,
bind the client's Supabase Auth ID to the default tenant using the exact SQL in
[`../docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md).
Set `HAKAISHIELD_DASHBOARD_ORIGIN` to the exact HTTPS origin serving the static
dashboard (for example `https://dashboard.example.com`, without a trailing
slash); the API rejects arbitrary browser origins in the deployed stack.
By default the proxy preserves the protected visitor hostname when contacting
the origin. Set `HAKAISHIELD_ORIGIN_HOST_FROM_TARGET=true` only when the public
protected hostname and the origin's virtual host differ. Hosted staging origins
such as Cloudflare Pages otherwise reject the protected hostname before serving
the site.

Keep the domain's A record pointed at the box. Keep `HAKAISHIELD_CHALLENGE_SECRET`
stable across restarts; changing it invalidates active challenges and cookies.
Set `HAKAISHIELD_SAMPLE_RETENTION_DAYS` to the client-approved period (1-365;
example/default 30) before collecting candidate labels. The proxy prunes old
samples at startup and hourly, at most 10,000 rows per run, without a request
path query. After binding the pilot Auth owner in Postgres, restart the proxy
so the host-bound default tenant loads that owner and its policy drafts.

---

## It starts in shadow mode

`HAKAISHIELD_MODE=shadow` — the full pipeline runs, every decision is recorded,
and nothing is acted on. Every visitor reaches the origin untouched.

Watch it for a few days:

```bash
curl -s -H "Authorization: Bearer $EVIDENCE_TOKEN" \
  https://customer.example/api/v1/dashboard/evidence | jq '.[:20]'
```

Switch to `HAKAISHIELD_MODE=enforce` only once nothing legitimate is being
caught. Then `docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d` again.

---

## Testing it

```bash
pip install requests playwright && playwright install chromium
python3 bot-testing/ladder/ladder.py --url https://customer.example/ --token "$EVIDENCE_TOKEN"
```

Seven rungs, each adding exactly one capability over the last — stdlib client,
`requests`, a lying user agent, full browser headers, a crawl pattern, headless
Playwright, headful Playwright. **The rung where detection stops is your
answer.** Run it from your own machine, not from the server: DigitalOcean acts
on abuse reports, and outbound attack traffic from a droplet is how you lose an
account — the same is true of every provider on that list.

**Keep `-collect-labels` off while you do this.** It is deliberately absent
from the compose file. Every solved challenge and honeypot trip would become a
training sample, and the model would learn what *your* bots look like rather
than what real ones do. See [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md).

---

## Running without Docker

`hakaishield.service` runs the binary directly under systemd, with
`CAP_NET_BIND_SERVICE` so it binds :443 without being root, and the rest of
the sandbox taken away. Use that **or** compose, not both.

```bash
sudo useradd --system --no-create-home hakaishield
sudo cp deploy/hakaishield.service /etc/systemd/system/
sudo cp deploy/.env /opt/hakaishield/.env
sudo systemctl daemon-reload && sudo systemctl enable --now hakaishield
sudo journalctl -u hakaishield -f
```

---

## Before the first real visitor

- [ ] Renewal proven: `certbot renew --dry-run`
- [ ] SSH restricted to your IP, not `0.0.0.0/0`
- [ ] `EVIDENCE_TOKEN` random, and not in shell history
- [ ] `HAKAISHIELD_CHALLENGE_SECRET` random, stable, and at least 32 bytes
- [ ] Domain/Host and origin URL match this customer's site
- [ ] Started in `shadow`
- [ ] `-collect-labels` off while testing
- [ ] Billing alert set with the host
- [ ] Reboot the box once and confirm it comes back up by itself

That last one is the only way to know the restart configuration is real.

---

## When something is wrong

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs --tail=100 hakaishield
curl -sk https://localhost/__hakaishield/healthz     # from the box itself
openssl s_client -connect customer.example:443 -servername customer.example </dev/null 2>&1 | head -20
```

**Every JA4 looks identical** → something is terminating TLS in front of you.
Re-read "The one rule" in `DEPLOYMENT.md`.

**Certificate errors 90 days in** → the renewal hook never ran. That is what
the hook and `certbot renew --dry-run` checks are meant to catch. Inspect
`/etc/hakaishield/tls` and rerun
`sudo /usr/local/sbin/hakaishield-sync-certs /etc/letsencrypt/live/<domain>`
before restarting the service if the mounted copy is stale.

**Permission denied on the TLS key** → verify `/etc/hakaishield/tls` is mode
`0750` and `privkey.pem` is root:65532 mode `0640`. The container mounts only
this copy at `/run/hakaishield/tls`, not Certbot's root-only tree.

**Port 443 refused** → in Docker the process listens on 8443 and the host
publishes 443; check the port mapping before the process.
