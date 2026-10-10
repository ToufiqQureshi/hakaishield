# Local development

## Public site preview (no credentials)

From `dashboard/` run:

```sh
npm install
npm run dev:marketing
```

Open `http://localhost:3000`. This preview includes public pages and legal drafts. Account actions intentionally show an unavailable page because signup/auth are real Supabase flows; the preview does not fake them.

## Full dashboard and authentication

Create `dashboard/.env.local` (this file is git-ignored) with:

```dotenv
VITE_SUPABASE_URL=https://<project-ref>.supabase.co
VITE_SUPABASE_ANON_KEY=<publishable-or-anon-key>
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

Get the URL and publishable/anon key from Supabase **Project Settings → API**. The publishable/anon key is intended for browser use with Supabase RLS and Auth policies enabled. Never put the `service_role` secret in `VITE_*` variables or the dashboard.

The authenticated dashboard additionally needs the Go API, PostgreSQL schema, Redis, and a Supabase Auth project with the legal consent migration applied. The backend needs the Supabase project URL to validate JWTs and a PostgreSQL connection URL (`DATABASE_URL`). Use a dedicated local/test project or local PostgreSQL; do not paste production database credentials into source control.

For Upstash, add `REDIS_URL=rediss://default:<password>@<host>:<port>` (or `UPSTASH_REDIS_URL` with the same value) to `backend/.env`, copied from the Upstash **TCP** connection tab. The Go edge uses Redis TCP over TLS. `UPSTASH_REDIS_REST_URL` and `UPSTASH_REDIS_REST_TOKEN` are valid HTTP API credentials, but this backend does not use them for its request-path Redis operations. Keep the TCP URL server-side and git-ignored. The backend's private `/ready` endpoint returns 503 until both Redis and PostgreSQL respond.

Start the dashboard with `npm run dev` after setting those values. A complete local signup/login/domain flow also needs email verification configured in Supabase Auth and a reachable backend configured with the same project's URL/database. Payments are intentionally not wired yet.
