# Backend deployment handoff

Last updated: 2026-09-28

This document records the live infrastructure inventory and the exact operator
steps for the first managed pilot. It must never contain passwords, private
keys, database URLs, Supabase keys, challenge secrets, or evidence tokens.

## Current inventory

| Item | Current value |
|---|---|
| Frontend | Cloudflare Pages project `hakaishield-dashboard` |
| Frontend hosts | `https://interviewyaar.lol` and `https://www.interviewyaar.lol` |
| Protected staging host | `https://shield.interviewyaar.lol` (DNS-only A record) |
| AWS region | Mumbai, `ap-south-1` |
| EC2 instance | `i-04a0ded0c9d9c7434` (`hakaishield-backend`) |
| AMI | Ubuntu Server 24.04 LTS, x86_64, `ami-006f82a1d5a27da54` |
| Instance type | `t2.micro` (1 vCPU, 1 GiB RAM; pilot testing only) |
| Root disk | 30 GiB gp3 |
| Public IPv4 | `13.233.137.100` (auto-assigned; changes after stop/start) |
| Public DNS | `ec2-13-233-137-100.ap-south-1.compute.amazonaws.com` |
| Security group | `sg-0711fb9b8cc94a163` |
| Key-pair name | `hakaishield-backend` |
| Protection mode | `shadow` for initial traffic |
| Redis | Docker Compose private network; port 6379 is not published |
| PostgreSQL/Auth | Supabase; credentials stay only in the server environment |
| Server bootstrap | Docker 29.1.3, Compose 2.40.3, Certbot 2.9.0, UFW active |
| Swap | 2 GiB `/swapfile`, persistent through `/etc/fstab` |
| Deployed source | `4491c38b779fdb6dc0506f62c2633dd6955a025d` |
| Pilot image | `hakaishield:4491c38b`, image `3657065e30c2`, non-root |
| TLS | Let's Encrypt certificate; expires 2026-12-25; renewal dry-run passed |

The private SSH key is operator-held and must not be copied into this repository.
The restricted working copy is `%USERPROFILE%\.ssh\hakaishield-backend.pem`;
keep it in a private local folder and back it up securely.

## Network policy

The EC2 security group must allow:

- TCP 22 from the operator's current public IP `/32` only.
- TCP 80 from the internet for Let's Encrypt HTTP-01 issue and renewal.
- TCP 443 from the internet for protected visitor traffic.
- No public Redis, PostgreSQL, Docker daemon, or application admin port.

If the operator's ISP address changes, update the SSH source rule to the new
`/32` before attempting to connect. Do not broaden SSH to `0.0.0.0/0`.

## Remaining launch decisions

- A stable-IP decision before client DNS cutover.
- The verified pilot tenant/owner binding.
- The real client's protected hostname and origin after staging approval.
- Capacity sizing after bounded shadow-traffic measurements.

## Verified on the EC2 host

On 2026-09-26 the release artifact was copied to `/opt/hakaishield` and the
production Dockerfile built successfully on the instance. The first build used
about 914 MiB RAM and 920 MiB swap while compiling `go-redis`, confirming that
the swap file is required on this 1 GiB host. The final image runs as
`nonroot:nonroot`, uses `/hakaishield` as its entrypoint, and its `-h` startup
check passed.

A localhost-only HTTPS smoke test used a temporary self-signed certificate and
dummy origin. `GET /__hakaishield/healthz` returned `status: ok`, and a proxied
request returned the origin marker `HAKAI_ORIGIN_OK` in shadow mode. The first
attempt correctly failed when the TLS key was `0600 ubuntu:ubuntu`; the retry
used the production `root:65532` / `0640` key permission and passed. Temporary
containers, processes, certificates, and files were removed after the test.

The public staging path is now live. `shield.interviewyaar.lol` resolves directly
to the EC2 address, has a valid Let's Encrypt certificate, and returned HTTP
200 from `/__hakaishield/healthz` on 2026-09-26. Compose reported the backend
running with zero restarts and Redis healthy; Redis returned `PONG`. Startup
logs confirmed PostgreSQL connectivity, the authenticated domains/rules/settings
API, origin `https://interviewyaar.lol`, and shadow mode with no blocking or
challenges.

Two startup defects were found during the real deployment and fixed with tests:

- `65950aa6`: look up the default owner by protected host instead of treating
  the string `default` as a tenant UUID.
- `4491c38b`: cast Supabase's UUID `owner_user_id` to text before applying the
  empty owner fallback.

Full Go tests, vet, build, `golangci-lint`, and focused mutation checks passed
for these changes. The deployment was intentionally paused after the first
successful public health check at the operator's request. Restart recovery,
the proxied homepage, TLS inspection, bounded load testing, tenant ownership,
and release-branch integration have not yet been signed off.

`interviewyaar.lol` is the HakaiShield frontend. Do not point it at EC2. The
protected client hostname needs a DNS-only A record to the EC2 public IPv4 so
the visitor ClientHello reaches HakaiShield directly. A proxied Cloudflare
record, ALB, or CloudFront would terminate TLS and invalidate raw JA4 evidence.

## Connection template

From PowerShell, after the public IP is known:

```powershell
ssh -i "$env:USERPROFILE\.ssh\hakaishield-backend.pem" ubuntu@13.233.137.100
```

If OpenSSH rejects broad Windows file permissions, move the key into a private
operator folder and restrict its ACL before retrying. Never upload the key to
GitHub, the server, a ticket, or chat.

## Deployment sequence

1. Confirm the instance is `Running` and both EC2 status checks pass.
2. Verify SSH using the public IPv4 and the `ubuntu` account.
3. Confirm the 2 GiB swap file is active before building containers; 1 GiB RAM
   is too tight for reliable Docker builds without swap.
4. Copy the repository release to `/opt/hakaishield` without local `.env`, test
   artifacts, graph output, or private keys.
5. Point the protected hostname's DNS-only A record to the EC2 public IPv4.
6. Run `deploy/setup.sh <protected-hostname> <operator-email>` as root.
7. Create `/opt/hakaishield/deploy/.env` with mode `shadow` and the required
   values listed by `deploy/docker-compose.yml`.
8. Validate Compose configuration, build, and start the Redis and HakaiShield
   services.
9. Confirm HTTPS, raw TLS/JA4 variation, origin proxying, health, logs, Redis
   recovery, reboot recovery, and certificate renewal.
10. Run normal-browser, headless-browser, velocity, replay, malformed-request,
    challenge, false-positive, and bounded load tests before any enforcement.

## Next session checklist

1. Recheck Compose state, restart count, sanitized logs, the public health
   endpoint, and the proxied `/` response before changing anything.
2. Inspect the public certificate subject, SAN, issuer, and expiry.
3. Restart the backend through Compose and confirm health and Redis recovery.
4. Run a small controlled HTTPS load test suitable for a `t2.micro`; record
   latency percentiles and errors instead of making an unmeasured capacity claim.
5. Verify or create the Supabase tenant/owner binding for
   `shield.interviewyaar.lol`, then test authenticated dashboard APIs.
6. Keep detection and any learned model in shadow mode until labelled real
   traffic measures recall, human false-positive rate, and challenge burden.
7. Integrate commits `65950aa6` and `4491c38b` from `codex/aws-deploy-fix` into
   `release/client-pilot-hardening` without staging unrelated dirty HTTP/2 or
   worktree changes; rerun the required checks and push the release branch.
8. Allocate a stable address or another stable ingress before giving DNS to a
   paying client because the current auto-assigned IPv4 changes on stop/start.

The 2026-09-28 recheck found the health endpoint and certificate valid, but
the proxied root returned Cloudflare 403 because the staging hostname differs
from the Pages origin's virtual host. The explicit
`HAKAISHIELD_ORIGIN_HOST_FROM_TARGET=true` deployment option fixes this setup;
its default remains false so normal client origins continue receiving the
protected visitor hostname. The operator's public IP also changed, so EC2 SSH
source must be updated from the old `/32` to the current operator `/32` before
the new image can be deployed and the remaining checks can run.

## Capacity note

`t2.micro` is suitable for bring-up and light shadow traffic. Its 1 GiB memory
does not support a production load claim or meaningful hard load test. Measure
CPU credit balance, memory, swap, p95/p99 latency, origin errors, and egress. If
the pilot exceeds the box, resize to at least 2 GiB RAM before enforcement.

## Rollback

- Keep `HAKAISHIELD_MODE=shadow` during initial validation.
- If the proxy affects the client path, restore the protected hostname's prior
  DNS target and stop the Compose project.
- Supabase remains external, and Redis contains disposable velocity/challenge
  state, so replacing the EC2 instance does not remove the primary database.

## Secrets policy

Only environment-variable names and placeholders belong in Git. Store live
values in the server's root-readable deployment environment (or a future AWS
secret store), generate unique random values, and rotate any value exposed in
chat, logs, screenshots, shell history, or commits.
