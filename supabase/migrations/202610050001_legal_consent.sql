-- Durable signup agreement records are created by the database, not by
-- client-side inserts. Bump these document versions with a migration when
-- terms or privacy language changes materially.
CREATE TABLE IF NOT EXISTS public.legal_consent_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    document_type TEXT NOT NULL CHECK (document_type IN ('terms', 'privacy', 'marketing')),
    document_version TEXT NOT NULL,
    consented BOOLEAN NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source TEXT NOT NULL CHECK (source IN ('signup', 'preferences'))
);

CREATE INDEX IF NOT EXISTS legal_consent_user_latest_idx
    ON public.legal_consent_events (user_id, document_type, recorded_at DESC, id DESC);

ALTER TABLE public.legal_consent_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.legal_consent_events FORCE ROW LEVEL SECURITY;
REVOKE ALL ON public.legal_consent_events FROM PUBLIC, anon, authenticated;
REVOKE ALL ON SEQUENCE public.legal_consent_events_id_seq FROM PUBLIC, anon, authenticated;

CREATE OR REPLACE FUNCTION public.capture_signup_legal_consent()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $function$
DECLARE
    consent JSONB := NEW.raw_user_meta_data -> 'legal_consent';
    wants_marketing BOOLEAN;
BEGIN
    IF consent IS NULL
       OR consent ->> 'terms_version' IS DISTINCT FROM '2026-10-05'
       OR consent ->> 'privacy_version' IS DISTINCT FROM '2026-10-05' THEN
        RAISE EXCEPTION 'Current Terms and Privacy Notice acceptance is required';
    END IF;

    wants_marketing := COALESCE(consent -> 'product_updates' = 'true'::JSONB, FALSE);

    INSERT INTO public.legal_consent_events (user_id, document_type, document_version, consented, source)
    VALUES
        (NEW.id, 'terms', '2026-10-05', TRUE, 'signup'),
        (NEW.id, 'privacy', '2026-10-05', TRUE, 'signup'),
        (NEW.id, 'marketing', '2026-10-05', wants_marketing, 'signup');

    RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS capture_signup_legal_consent ON auth.users;
CREATE TRIGGER capture_signup_legal_consent
    AFTER INSERT ON auth.users
    FOR EACH ROW EXECUTE FUNCTION public.capture_signup_legal_consent();

CREATE OR REPLACE FUNCTION public.set_marketing_consent(p_consented BOOLEAN)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $function$
DECLARE
    current_user_id UUID := auth.uid();
BEGIN
    IF current_user_id IS NULL THEN
        RAISE EXCEPTION 'Authentication required';
    END IF;

    INSERT INTO public.legal_consent_events (user_id, document_type, document_version, consented, source)
    VALUES (current_user_id, 'marketing', '2026-10-05', p_consented, 'preferences');
END;
$function$;

CREATE OR REPLACE FUNCTION public.get_marketing_consent()
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
SECURITY DEFINER
SET search_path = ''
AS $function$
    SELECT COALESCE((
        SELECT event.consented
        FROM public.legal_consent_events AS event
        WHERE event.user_id = auth.uid() AND event.document_type = 'marketing'
        ORDER BY event.recorded_at DESC, event.id DESC
        LIMIT 1
    ), FALSE);
$function$;

REVOKE ALL ON FUNCTION public.capture_signup_legal_consent() FROM PUBLIC, anon, authenticated;
REVOKE ALL ON FUNCTION public.set_marketing_consent(BOOLEAN) FROM PUBLIC, anon;
REVOKE ALL ON FUNCTION public.get_marketing_consent() FROM PUBLIC, anon;
GRANT EXECUTE ON FUNCTION public.set_marketing_consent(BOOLEAN) TO authenticated;
GRANT EXECUTE ON FUNCTION public.get_marketing_consent() TO authenticated;
