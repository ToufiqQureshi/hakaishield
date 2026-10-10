import { LEGAL_VERSIONS } from './legal';

export function buildSignupConsent(marketingOptIn: boolean, acceptedAt: string) {
  return {
    terms_version: LEGAL_VERSIONS.terms,
    privacy_version: LEGAL_VERSIONS.privacy,
    accepted_at: acceptedAt,
    product_updates: marketingOptIn,
    product_updates_updated_at: acceptedAt,
  };
}
