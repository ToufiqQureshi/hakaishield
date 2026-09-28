# HakaiShield dashboard

React/Vite dashboard for the managed single-client pilot. It displays real
domain status, request totals, evidence and saved rule drafts from the Go API.
It does not activate domains or process payments. Customers can save exact
login/checkout route tags as versioned shadow policy drafts in Settings; those
tags change live velocity buckets only after the policy activation gate.
Evidence rows distinguish scored signals from yellow "Observed" candidates.
The proxy's shadow mode does not serve challenge pages, so challenge-only
observations appear only for visitors who later solve an enforced challenge.

## Local development

Use Node 24. Copy `.env.example` to `.env.local` and set your Supabase project
URL and anon key. The API defaults to `http://localhost:8080/api/v1` only in
Vite development mode.

```bash
npm ci
npm run dev
npm run typecheck
npm test
npm run build
```

## Pilot deployment

The dashboard is a static React/Vite app. Its Cloudflare Pages project is
`hakaishield-dashboard` (Direct Upload). The first production deployment is the
public marketing build while the backend API is not live. It includes landing,
pricing, docs, contact and legal pages; authenticated dashboard routes are not
exposed in that build.
The chosen production hostname is `https://interviewyaar.lol`.
From `dashboard/`, use Node 24.19.0 and `npm ci`. Run
`npm run build:pages:marketing` for the public site. Once the protected client
API is live, set the public configuration below and use `npm run build:pages`
for the full authenticated dashboard.
Upload `dist/` for a preview with:

```bash
npx wrangler pages deploy dist --project-name hakaishield-dashboard --branch=release/client-pilot-hardening
```

Use `--branch=main` for the production upload. Direct Upload
does not automatically publish GitHub pushes.

Set these **build-time** variables in the build environment before running the
full `build:pages` (Vite embeds them in public JavaScript). The marketing build
does not need or bundle them:
Setting them only in Cloudflare Pages project settings does not change a bundle
built and uploaded locally.

- `VITE_SUPABASE_URL`: the same project used by the Go backend.
- `VITE_SUPABASE_ANON_KEY`: its public anon key.
- `VITE_API_BASE_URL`: `https://<protected-client-domain>/api/v1`; the
  current backend serves its dashboard API on the protected domain.
- `VITE_PILOT_CONTACT_EMAIL`: a working operator inbox. Until it is set,
  the contact page shows no send action.

`build:pages` refuses missing/placeholder values, non-HTTPS URLs, malformed API
paths, and Supabase server secret keys. Never provide a `service_role` or
`sb_secret_` key: only the public anon/publishable key belongs in this bundle.
Cloudflare Pages serves React's deep links from `index.html` when there is no
top-level `404.html`; no redirect rule is needed. Add the final Pages/custom
domain origin and `/sign-in` password-reset URL to the Supabase Auth allowlist.
Set `HAKAISHIELD_DASHBOARD_ORIGIN` on the backend to that exact HTTPS origin for
authenticated API CORS. Set `DATABASE_URL` and `SUPABASE_URL` in `deploy/.env`
so the authenticated backend API is enabled.

The first client domain is provisioned by the operator after ownership, TLS,
origin and shadow-mode checks. See `../docs/DEPLOYMENT.md` for the
gate and account-to-domain binding. Restart the proxy after binding the
default owner so owner-scoped route drafts can load. A pending row does not
protect traffic.
