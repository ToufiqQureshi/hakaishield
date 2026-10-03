export type DomainState = {
  label: string;
  detail: string;
  protected: boolean;
};

// Treat every unknown state as unprotected. The dashboard must never claim a
// domain is protected just because a row exists in the database.
export function domainState(status: string): DomainState {
  if (status === 'active') {
    return {
      label: 'Protected',
      detail: 'Traffic is routed through HakaiShield.',
      protected: true,
    };
  }
  if (status === 'pending_verification') {
    return {
      label: 'Verify ownership',
      detail: 'This domain is not routing through HakaiShield yet. Add the DNS TXT record, then verify.',
      protected: false,
    };
  }
  if (status === 'verified') {
    return {
      label: 'Ownership verified',
      detail: 'Ownership is proven, but traffic is not routing through HakaiShield until setup completes.',
      protected: false,
    };
  }
  return {
    label: 'Setup needs review',
    detail: 'Protection is not confirmed. Contact the pilot operator.',
    protected: false,
  };
}
