# Backend deployment handoff

Last updated: 2026-09-26

This document records the live infrastructure inventory and the exact operator
steps for the first managed pilot. It must never contain passwords, private
keys, database URLs, Supabase keys, challenge secrets, or evidence tokens.

## Current inventory

| Item | Current value |
|---|---|
| Frontend | Cloudflare Pages project `hakaishield-dashboard` |
| Frontend hosts | `https://interviewyaar.lol` and `https://www.interviewyaar.lol` |
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
| Pilot image | `hakaishield:pilot`, image `2fd93bf41e8f`, 12.7 MB, non-root |

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

## Required values still pending

- A stable-IP decision before client DNS cutover.
- The real protected client hostname, for example `protect.client.example`.
- The client's origin URL, ideally in or near Mumbai.
- A working operator email for Let's Encrypt expiry notices.
- The approved Supabase PostgreSQL connection URL and project URL.
- Stable generated challenge/evidence secrets.
- The verified pilot tenant/owner binding.

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

The backend is not public yet. Real DNS, Let's Encrypt, Compose Redis,
PostgreSQL/Auth configuration, and client-path testing remain pending.

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
