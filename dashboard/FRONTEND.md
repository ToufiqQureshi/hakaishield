# Frontend deployment handoff

**Snapshot:** 2026-09-25, branch `release/client-pilot-hardening`.
This is the operational handoff for the HakaiShield website on Cloudflare Pages.
`docs/STATUS.md` remains the whole-product status and
`deploy/RUNBOOK.md` and `docs/STATUS.md` (launch gate) remain the backend/client launch checklist.

## Current deployed state

- Cloudflare account owns the active `interviewyaar.lol` zone. The separate
  Direct Upload Pages project is `hakaishield-dashboard`.
- The tested public marketing build was uploaded to Pages production (`main`
  deployment label) and preview (`release/client-pilot-hardening` label).
  Pages production URL: <https://hakaishield-dashboard.pages.dev/>.
  Preview tested at <https://95f3940e.hakaishield-dashboard.pages.dev/>.
- `interviewyaar.lol` was added as a Pages custom domain. At the last check,
  Cloudflare reported `pending`, `CNAME record not set`, and certificate
  validation `pending`. HTTPS on the custom hostname timed out; **do not call
  that hostname live until it returns the landing page over valid TLS**.
- The first production build is **marketing only**: `/`, `/landing`, `/pricing`,
  `/changelog`, `/docs`, `/contact`, `/about`, `/terms`, and `/privacy`.
  Unknown paths redirect to `/`. The sign-in/dashboard code is absent from this
  build because the Go API is not deployed. The main `App.tsx` still provides
  the full authenticated dashboard for a later build.
- No public contact email or online contact submission is configured during
  testing. The contact page explicitly says to use an agreed channel.
- This Pages deployment does not run bot detection. The inline Go proxy still
  needs its own server and direct visitor TLS to preserve ClientHello/JA4.

## Code and build commands

`dashboard/.node-version` pins Node 24.19.0. `src/main.tsx` loads
`MarketingApp.tsx` only when Vite mode is `marketing`; the normal production
entry still loads `App.tsx`. `Landing.tsx` hides Sign in in marketing mode.

```bash
cd dashboard
npm ci
npm test
npm run typecheck
npm run build:pages:marketing
npx wrangler pages deploy dist --project-name hakaishield-dashboard --branch=release/client-pilot-hardening
# After preview verification, publish the same approved dist as production:
npx wrangler pages deploy dist --project-name hakaishield-dashboard --branch=main
```

This is **Direct Upload**: pushing GitHub does not deploy Pages. The
`dashboard/.env` file is local and ignored by Git; never commit it. The
marketing bundle has no Supabase URL/key or backend URL, even if a local `.env`
exists. `tests/marketingBuild.test.mjs` builds in a temporary directory with
those variables forced empty and checks that the public bundle excludes auth
configuration.

For the future full dashboard, set these public build-time variables and run
`npm run build:pages` instead. That command refuses missing/placeholder values,
non-HTTPS URLs, malformed API paths and Supabase server keys.

| Variable | Value source | Status |
| --- | --- | --- |
| `VITE_SUPABASE_URL` | Same Supabase project as backend | Available in local backend `.env`; do not copy private DB values |
| `VITE_SUPABASE_ANON_KEY` | Supabase public anon/publishable key only | Owner supplied; never use `service_role` or `sb_secret_` |
| `VITE_API_BASE_URL` | `https://<protected-client-domain>/api/v1` | Unknown until client backend host is deployed |
| `VITE_PILOT_CONTACT_EMAIL` | Public working inbox, optional | Intentionally unset during testing |

The Go backend currently serves its dashboard API on the protected client
hostname. `api.interviewyaar.lol` is **not** configured by current Compose,
certificate or tenant routing. Do not point the frontend at it without an
explicit backend change. In the later full release, allow the exact dashboard
origin `https://interviewyaar.lol` in backend
`HAKAISHIELD_DASHBOARD_ORIGIN`, and add that origin and
`https://interviewyaar.lol/sign-in` to Supabase Auth redirect URLs. Test
sign-in, password reset, domain list, stats and evidence against the real API.

## Verification completed

- `npm test`: 17/17 pass, including a marketing build with backend/Supabase
  variables empty.
- `npm run typecheck` and `npm run build:pages:marketing`: pass.
- Preview returned HTTP 200 for `/`, `/pricing`, `/contact` and `/sign-in`
  (Pages SPA fallback); its JavaScript asset returned 200.
- Headless Chrome rendered the landing text from the preview, without a sign-in
  link. The marketing bundle scan found no Supabase/API configuration.
- The custom root domain is **not verified** while its CNAME/certificate is
  pending. The backend and a real authenticated browser journey are not live.

## Exact next steps

1. In Cloudflare DNS for `interviewyaar.lol`, set a proxied CNAME at `@`
   pointing to `hakaishield-dashboard.pages.dev`. Replace any old root
   A/AAAA/CNAME that conflicts. The Wrangler OAuth token has Pages write but
   lacks DNS record permission; DNS API returned an authentication error, so
   this one record must be changed in the Cloudflare UI or with a scoped DNS
   Edit token. The owner has been asked to do it.
2. Poll Pages custom domain until both domain verification and certificate
   validation are active. Check `https://interviewyaar.lol/`, `/pricing`,
   `/contact` and a browser render over valid TLS. If it remains pending,
   inspect the Pages custom-domain status and root DNS/CNAME conflict.
3. Keep public contact email unset while testing, per owner instruction.
4. When the backend/client domain exists, deploy the proxy per
   `deploy/RUNBOOK.md`; keep the protected visitor domain off Cloudflare
   HTTP proxying so the Go process sees raw TLS. Then build the **full**
   dashboard with the real HTTPS API URL, test it on a Pages preview, and upload
   to production only after authenticated flows and backend CORS work.

The repository contains concurrent uncommitted HTTP/2 work and unrelated
`graphify-out/`/test scratch files. Preserve them; stage only the intended
frontend/docs files for this change.
