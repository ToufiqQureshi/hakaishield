# Public SaaS launch plan

Status: **not ready for public paid signup**. Customer onboarding and activation exist in code; the live multi-domain edge and payment flow are not deployed. This document is the release gate for a customer who signs up, pays, connects a domain, and receives working protection without operator intervention.

## Required customer journey

1. A customer creates an account and signs in. Email confirmation is currently disabled, so account email ownership is not established.
2. The customer chooses a published plan and completes provider-hosted checkout. The server grants access only after a verified provider webhook or a provider API reconciliation, never from the browser return URL.
3. The customer registers a domain and proves ownership with the existing DNS TXT challenge. Plan limits apply to the account and to each domain. An unpaid account cannot activate protection.
4. The customer points the domain at the edge. The system verifies the DNS target, obtains a certificate for that exact verified host, checks the origin over HTTPS, and marks the domain active only after a real end-to-end probe succeeds.
5. A customer sees live traffic, decisions, usage, subscription status, renewal failures, and an explicit rollback path in the dashboard. Enforce mode is enabled only after a reviewed shadow period.
6. Cancellation or payment failure enters a documented grace period. Traffic continues safely during that period; the customer is warned. An expired account is not silently left marked protected.

### Customer connection options

- The general-purpose mode is a guided **single hostname DNS record change** (A/AAAA or CNAME as appropriate). Customers keep their registrar, authoritative DNS provider, nameservers, email records, and all unrelated subdomains. Recommend starting with `www` or a staging subdomain, verify the origin and certificate, then switch the production hostname with a tested rollback record. Apex/root domains need provider-supported ALIAS/ANAME or A/AAAA guidance.
- DNS-only changes do not protect a hostname unless its web requests reach HakaiShield. Make this explicit before activation; never call an account protected until the live route probe succeeds.
- For customers already proxying through Cloudflare, a later no-DNS-edit connector can install a Worker route on their existing proxied hostname. This still requires a proxied DNS record and customer-authorized Worker access. Cloudflare exposes JA4 in Workers only with Bot Management enabled on Enterprise; without that, HakaiShield must document reduced signals or the customer can use the direct HakaiShield DNS mode.
- A JavaScript tag or out-of-path analytics connector can report observations but cannot reliably block requests before they reach the origin. Do not market that as equivalent protection.
- A Cloudflare Worker connector needs a separate ingress/decision architecture; the current Go reverse proxy is not a Worker integration. Keep it as a planned connection mode until request signing, data minimization, deployment, rollback, and multi-tenant isolation are implemented and tested.

## Implementation order

### 0. Legal identity, privacy, and consent (launch blocker)

- Complete the operator legal name, registered address, country, governing law, support and privacy contacts in the public Terms and Privacy Notice; have counsel review them for the operator country and intended customer regions.
- Confirm published data categories, purposes, lawful bases, retention configuration, hosting regions, subprocessors, international transfer mechanisms, and deletion behavior match production.
- Review the Acceptable Use Policy, Cookie and Browser Storage Notice, and proposed Data Processing Addendum; publish the executed DPA and required transfer terms before processing customer visitor data in regions that require them.
- Applied `supabase/migrations/202610050001_legal_consent.sql` to the connected Supabase project on 2026-10-06. Transactional database checks proved valid signup consent events, rejection of missing consent, and authenticated marketing preference updates. A real email/signup journey still needs testing.
- Add a renewed-consent flow for material document changes and retain the prior version users accepted.
- Do not enable paid checkout until these details are complete and the required review is recorded.

### 1. Billing and entitlements

- Select a supported merchant provider and create its live and test accounts. Define actual price IDs, currency, tax treatment, refund policy, grace period, and per-plan limits before presenting a checkout button.
- Store the provider customer/subscription IDs, plan, status, paid-through time, and webhook event IDs in Postgres. Make webhook processing signature-verified, idempotent, and resilient to duplicate or out-of-order delivery. Reconcile against the provider API on a schedule.
- Expose an authenticated billing status and checkout endpoint. Use provider-hosted checkout and customer portal; do not collect card data in the dashboard.
- Enforce entitlements at domain creation and activation. Meter request count and egress durably by account and billing period. Apply a documented limit policy that does not abruptly take customer sites offline.

### 2. Automated domain activation

- Implemented in the edge API: a verified owner can request activation; HakaiShield probes the public origin with the protected Host header, requests a certificate only for a verified tenant hostname, then atomically moves that tenant to active. Failures leave it verified. Newly activated tenants start in monitor/shadow mode. `ACME_CACHE_DIR` must be durable and `ACME_EMAIL` configured.
- Still required: deploy the edge with raw customer TLS reaching this process. TLS-ALPN-01 cannot pass through a Cloudflare Tunnel or ordinary Cloudflare-proxied customer record because Cloudflare terminates visitor TLS first. A Cloudflare-for-SaaS custom-hostname frontend or DNS-01 automation is a separate architecture and remains unimplemented. Do not claim the current setup supports tunneled customer hostnames or preserves customer JA4 through a TLS-terminating proxy.
- The activation API now validates that every public A/AAAA answer points to its configured edge IPv4 before it probes the origin or requests a certificate. Before broad activation, add per-account provisioning rate limits, certificate renewal alerts, and a deployed staging exercise with a real HTTPS request through the edge.
- Keep the pilot's default certificate and customer ACME certificates on persistent storage. Do not allow arbitrary SNI to trigger certificate orders. Recheck ownership and remove stale inactive domains.
- Define migration and rollback for the existing `default` pilot tenant and single-certificate deployment.

### 3. Reliable operation

- Move traffic counters and dashboard history to durable, tenant-scoped storage. The current in-memory stats and 1,000-entry evidence ring cannot support paid usage reporting or survive node loss.
- Add readiness checks for Postgres, Redis, certificate state, and origin reachability; metrics and alerts for TLS issuance/renewal, error rate, latency, queue drops, Redis failover, and storage growth.
- Test at least two edge nodes, shared challenge/certificate state, failover, and a rolling deploy. Keep TLS passthrough ahead of edge nodes; a TLS-terminating CDN or load balancer breaks the JA4 claim.
- Run load and soak tests with representative request and response sizes, including large bodies, HTTP/2, slow clients, and origin failures. Establish capacity and egress cost limits per plan.

### 4. Release verification

- Exercise the complete journey in the payment provider's test mode: signup, email verification, successful and failed checkout, duplicate and delayed webhooks, cancellation, and renewal.
- Exercise domain ownership, wrong DNS, certificate issuance/renewal, SNI/Host mismatch, origin failure, shadow decisions, challenge, enforce, and rollback against a real staging domain and origin.
- Run tenant isolation, SSRF, rate-limit, and abuse tests; review legal terms, privacy/retention, support contacts, incident response, backups, and restore drills before accepting public payments.

### 5. Model training (last phase)

- After real traffic and reliable labels are available, train and evaluate the existing fast, explainable Go traffic model on held-out data. Run it in shadow first; review false positives before any enforcement change.
- Keep LLM fine-tuning on Hugging Face Jobs optional for offline analyst workflows such as incident summaries. Do not put an LLM call in the live request path. Train only on data approved for that use.

## Current evidence and blockers

- Dashboard typecheck, 42 frontend tests, and production and marketing builds pass locally. Backend `go test -p 1 ./...` and `go vet -p 1 ./...` pass with the pinned Go 1.26.6 toolchain. Two HTTP/2 tests fail under the host's Go 1.27.1, so upgrades need compatibility work. See `SHIP_READINESS_2026-10-05.md` for the current release decision.
- Auth, dashboard, DNS TXT ownership proof, per-tenant proxy routing, and shadow mode exist. `docs/STATUS.md` says no real client traffic has been served or measured.
- `deploy/docker-compose.yml` retains a default single-domain certificate and now has persistent ACME storage for customer certificates. Verified domains can activate through the API, but DNS/TLS cutover is untested on a real customer hostname.
- `dashboard/src/pages/Payment.tsx` explicitly says payments are unavailable. `dashboard/src/pages/Subscription.tsx` shows reference plans only. No provider integration, subscription state, invoices, or billing limits exist.
- The connected Supabase project has both migrations applied, and the local Go API connects to it and Upstash TCP Redis. Its application tables were closed to anonymous PostgREST access on 2026-10-06. A browser sign-in succeeded after manual account confirmation; automated email delivery has not been proven. Merchant approval, production domain/DNS, edge infrastructure, custom SMTP, and a real staging origin remain launch dependencies.
- The owner intends to sell internationally but has not selected a country of incorporation or an active merchant account. Customer geography does not determine merchant eligibility. Keep checkout disabled until the merchant country, provider, currency, tax obligations, and approved account are known. A test-mode integration cannot be represented as live billing.

Do not label this release public SaaS ready until every release verification item passes on deployed infrastructure with a test customer, then a controlled real customer.
