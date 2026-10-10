-- The Go API is the only reader/writer for these application tables. Supabase
-- grants browser roles access to new public tables by default; without this
-- migration, evidence and account rows can be read through PostgREST.
-- Keep the auth.users trigger's SECURITY DEFINER consent path separate.
DO $migration$
DECLARE
    table_name TEXT;
    sequence_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'tenants', 'mitigation_rules', 'protection_settings',
        'users', 'tenant_policy_revisions', 'training_samples',
        'evidence_records', 'legal_consent_events'
    ] LOOP
        IF to_regclass(format('public.%I', table_name)) IS NOT NULL THEN
            EXECUTE format(
                'REVOKE ALL ON TABLE public.%I FROM PUBLIC, anon, authenticated',
                table_name
            );
            EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', table_name);
        END IF;
    END LOOP;

    FOREACH sequence_name IN ARRAY ARRAY[
        'training_samples_id_seq', 'evidence_records_id_seq',
        'legal_consent_events_id_seq'
    ] LOOP
        IF to_regclass(format('public.%I', sequence_name)) IS NOT NULL THEN
            EXECUTE format(
                'REVOKE ALL ON SEQUENCE public.%I FROM PUBLIC, anon, authenticated',
                sequence_name
            );
        END IF;
    END LOOP;
END;
$migration$;

-- Tables created later by backend InitSchema must also stay private.
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
    REVOKE ALL ON TABLES FROM anon, authenticated;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public
    REVOKE ALL ON SEQUENCES FROM anon, authenticated;
