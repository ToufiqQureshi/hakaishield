import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useOutletContext } from 'react-router-dom';
import { AlertCircle, Shield } from 'lucide-react';
import { ApiError, getTenantPolicy, saveTenantPolicy, type PolicyRevision } from '../lib/api';
import { routeLabelDraft } from '../lib/routeLabels';
import type { LayoutContext } from '../components/Layout';

export default function ProtectionSettings() {
  const { selectedDomain, domainsLoading } = useOutletContext<LayoutContext>();
  const domainId = selectedDomain?.id ?? null;
  const currentDomain = useRef(domainId);
  currentDomain.current = domainId;
  const requestGeneration = useRef(0);
  const [revision, setRevision] = useState<PolicyRevision | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadedTenantId, setLoadedTenantId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [path, setPath] = useState('');
  const [className, setClassName] = useState<'login' | 'checkout'>('login');

  useEffect(() => {
    requestGeneration.current++;
    setRevision(null);
    setError(null);
    setNotice(null);
    setLoadedTenantId(null);
    setSaving(false);
    if (!domainId) return;
    const controller = new AbortController();
    setLoading(true);
    getTenantPolicy(domainId, controller.signal)
      .then((next) => {
        if (controller.signal.aborted) return;
        setRevision(next);
        setLoadedTenantId(domainId);
      })
      .catch((cause) => {
        if (!controller.signal.aborted) setError(cause instanceof ApiError ? cause.message : 'Could not load route tags.');
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [domainId]);

  const loaded = domainId !== null && loadedTenantId === domainId;

  async function saveRoute(routePath: string, routeClass: 'login' | 'checkout' | null) {
    if (!domainId || !loaded || saving) return;
    let document;
    try {
      document = routeLabelDraft(revision?.document ?? null, routePath, routeClass);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Invalid route tag.');
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    const generation = requestGeneration.current;
    try {
      const next = await saveTenantPolicy(domainId, revision?.version ?? 0, document);
      if (currentDomain.current !== domainId || requestGeneration.current !== generation) return;
      setRevision(next);
      setPath('');
      setNotice('Saved as a shadow draft. It does not change live traffic.');
    } catch (cause) {
      if (currentDomain.current !== domainId || requestGeneration.current !== generation) return;
      setError(cause instanceof ApiError && cause.status === 409
        ? 'Policy changed elsewhere. Refresh this page before saving again.'
        : cause instanceof Error ? cause.message : 'Could not save route tag.');
    } finally {
      if (currentDomain.current === domainId && requestGeneration.current === generation) setSaving(false);
    }
  }

  function addRoute(event: FormEvent) {
    event.preventDefault();
    void saveRoute(path, className);
  }

  const active = revision?.document.mode === 'enforce';
  const routes = Object.entries(revision?.document.routeClasses ?? {}).sort(([a], [b]) => a.localeCompare(b));

  return (
    <div className="space-y-6 animate-in">
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Protection settings</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Policy and route tags for {selectedDomain?.domain ?? 'your domain'}.</p>
      </div>
      <div className="card p-6 space-y-4 max-w-2xl">
        <Shield size={22} className="text-blue-400" />
        <h2 className="text-base font-semibold">Domain policy</h2>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Route tags are exact paths. Saving creates a shadow draft. Activation is not available in this dashboard yet.
        </p>
        <div className="flex items-start gap-2 rounded p-3" style={{ background: 'var(--bg-tertiary)' }}>
          <AlertCircle size={16} className="text-yellow-400 shrink-0 mt-0.5" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Login routes use a 10 requests/second per IP bucket; checkout routes use 20. Shared networks may need review before enforcement.
          </p>
        </div>
        {domainsLoading || loading ? <p role="status" className="text-sm">Loading route tags…</p> : !domainId ? <p className="text-sm">No active domain is selected.</p> : null}
        {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
        {notice && <p role="status" className="text-sm text-green-400">{notice}</p>}
        {active && <p className="text-sm text-yellow-400">This policy is active. Create a new shadow revision before changing route tags.</p>}
        {domainId && loaded && (
          <>
            <ul className="space-y-2 text-sm">
              {routes.length === 0 && <li style={{ color: 'var(--text-muted)' }}>No custom route tags yet.</li>}
              {routes.map(([routePath, routeClass]) => (
                <li key={routePath} className="flex items-center justify-between gap-3">
                  <span><code>{routePath}</code> · {routeClass}</span>
                  {!active && <button type="button" className="text-xs underline" disabled={saving} onClick={() => void saveRoute(routePath, null)}>Remove</button>}
                </li>
              ))}
            </ul>
            {!active && (
              <form onSubmit={addRoute} className="flex flex-wrap items-end gap-2">
                <label className="text-xs">Exact route path
                  <input value={path} onChange={(event) => setPath(event.target.value)} placeholder="/account/signin" maxLength={256} className="block mt-1 rounded px-3 py-2 text-sm" required />
                </label>
                <label className="text-xs">Class
                  <select value={className} onChange={(event) => setClassName(event.target.value as 'login' | 'checkout')} className="block mt-1 rounded px-3 py-2 text-sm">
                    <option value="login">Login</option><option value="checkout">Checkout</option>
                  </select>
                </label>
                <button type="submit" disabled={saving} className="btn-secondary px-4 py-2 text-sm">{saving ? 'Saving…' : 'Save shadow draft'}</button>
              </form>
            )}
          </>
        )}
        <a href="/contact" className="text-xs underline">Need help with a policy?</a>
      </div>
    </div>
  );
}
