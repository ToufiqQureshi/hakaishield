-- tenants table: one row per protected domain, owned by a Supabase Auth user.
CREATE TABLE IF NOT EXISTS public.tenants (
    id VARCHAR(255) PRIMARY KEY,
    host VARCHAR(255) UNIQUE NOT NULL,
    target VARCHAR(255) NOT NULL,
    mode VARCHAR(50) NOT NULL DEFAULT 'enforce',
    evidence_token VARCHAR(255),
    owner_user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    name VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tenants_owner ON public.tenants(owner_user_id);

CREATE TABLE IF NOT EXISTS public.mitigation_rules (
    id VARCHAR(255) PRIMARY KEY,
    owner_user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    conditions_json TEXT NOT NULL,
    action VARCHAR(50) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rules_owner ON public.mitigation_rules(owner_user_id);

CREATE TABLE IF NOT EXISTS public.protection_settings (
    owner_user_id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    block_threshold INT NOT NULL DEFAULT 90,
    challenge_threshold INT NOT NULL DEFAULT 50,
    challenge_type VARCHAR(50) NOT NULL DEFAULT 'pow',
    honeypot_enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- RLS: the Go backend connects with the privileged postgres role
-- (bypasses RLS by default), so these policies exist for defense in
-- depth / to satisfy Supabase's linter, not because the app depends
-- on them for its own access path.
ALTER TABLE public.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.mitigation_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.protection_settings ENABLE ROW LEVEL SECURITY;

CREATE POLICY "owner can read own tenants" ON public.tenants
    FOR SELECT USING (auth.uid() = owner_user_id);
CREATE POLICY "owner can read own rules" ON public.mitigation_rules
    FOR SELECT USING (auth.uid() = owner_user_id);
CREATE POLICY "owner can read own settings" ON public.protection_settings
    FOR SELECT USING (auth.uid() = owner_user_id);