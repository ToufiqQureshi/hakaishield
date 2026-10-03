// Client-side checks for the add-domain form. They mirror the backend's
// validation (pkg/api/domains.go) so a typo is caught before a request,
// but the backend stays authoritative: it validates again and owns the
// final decision.

export function normalizeDomain(raw: string): string {
  return raw.trim().toLowerCase().replace(/\.$/, '');
}

function isHostname(host: string): boolean {
  if (host.length === 0 || host.length > 253 || !host.includes('.')) return false;
  // A bare IP literal is not a hostname we route.
  if (/^\d+\.\d+\.\d+\.\d+$/.test(host)) return false;
  return host.split('.').every((label) => /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(label) && label.length <= 63);
}

// domainInputError returns a message to show the user, or null when both
// values look usable.
export function domainInputError(domain: string, origin: string): string | null {
  if (!isHostname(normalizeDomain(domain))) {
    return 'Enter a domain you control, such as shop.example.com.';
  }

  let parsed: URL;
  try {
    parsed = new URL(origin.trim());
  } catch {
    return 'Enter a valid origin, such as https://origin.example.';
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return 'The origin must start with http:// or https://.';
  }
  if (!parsed.hostname) {
    return 'Enter a valid origin, such as https://origin.example.';
  }
  if (parsed.username || parsed.password) {
    return 'The origin must not contain credentials.';
  }
  if (parsed.search || parsed.hash) {
    return 'The origin must not contain a query string or fragment.';
  }
  return null;
}
