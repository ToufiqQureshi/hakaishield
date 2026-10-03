import { useState, type FormEvent } from 'react';
import { useOutletContext } from 'react-router-dom';
import { AlertCircle, CheckCircle2, Copy, Globe } from 'lucide-react';
import { ApiError, createDomain, verifyDomain } from '../lib/api';
import { domainInputError } from '../lib/domainOnboarding';
import { domainState } from '../lib/domainStatus';
import type { LayoutContext } from '../components/Layout';

// A single DNS value with a copy button. Kept local because it is only
// used here and carries its own "copied" feedback.
function CopyValue({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <span className="inline-flex items-center gap-2">
      <code className="text-xs break-all" style={{ color: 'var(--text-primary)' }}>{value}</code>
      <button
        type="button"
        onClick={() => {
          navigator.clipboard?.writeText(value).then(() => setCopied(true)).catch(() => {});
        }}
        className="inline-flex items-center gap-1 text-xs shrink-0"
        style={{ color: 'var(--accent-blue)' }}
      >
        <Copy size={12} />
        {copied ? 'Copied' : 'Copy'}
      </button>
    </span>
  );
}

export default function DomainsSiem() {
  const { domains, domainsLoading, refreshDomains } = useOutletContext<LayoutContext>();
  const [domain, setDomain] = useState('');
  const [origin, setOrigin] = useState('');
  const [adding, setAdding] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [verifyingId, setVerifyingId] = useState<string | null>(null);
  const [verifyResult, setVerifyResult] = useState<{ id: string; message: string; ok: boolean } | null>(null);

  const pending = domains.filter((d) => d.status === 'pending_verification' && d.verification);

  const onAdd = async (event: FormEvent) => {
    event.preventDefault();
    setNotice(null);
    const error = domainInputError(domain, origin);
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);
    setAdding(true);
    try {
      await createDomain(domain.trim().toLowerCase(), origin.trim());
      setDomain('');
      setOrigin('');
      setNotice('Domain added. Publish the DNS record below, then verify ownership.');
      await refreshDomains();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Could not add the domain. Try again.');
    } finally {
      setAdding(false);
    }
  };

  const onVerify = async (id: string) => {
    setVerifyResult(null);
    setVerifyingId(id);
    try {
      await verifyDomain(id);
      setVerifyResult({ id, message: 'Ownership verified. Setup can now continue.', ok: true });
      await refreshDomains();
    } catch (err) {
      setVerifyResult({ id, message: err instanceof ApiError ? err.message : 'Could not verify the domain. Try again.', ok: false });
    } finally {
      setVerifyingId(null);
    }
  };

  return (
    <div className="space-y-6 animate-in">
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Domains</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Add a site and prove you control it.</p>
      </div>

      <form onSubmit={onAdd} className="card p-5 space-y-4">
        <div className="flex items-start gap-3">
          <Globe size={18} style={{ color: 'var(--text-muted)' }} className="shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Protect a domain</p>
            <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              Adding a domain does not route traffic yet. You prove ownership with a DNS TXT record, then setup is
              completed before any traffic is sent here.
            </p>
          </div>
        </div>

        <div className="grid sm:grid-cols-2 gap-3">
          <label className="text-xs space-y-1" style={{ color: 'var(--text-secondary)' }}>
            Domain you control
            <input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="shop.example.com"
              className="w-full px-3 py-2 rounded text-sm"
              style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
            />
          </label>
          <label className="text-xs space-y-1" style={{ color: 'var(--text-secondary)' }}>
            Origin server
            <input
              value={origin}
              onChange={(e) => setOrigin(e.target.value)}
              placeholder="https://origin.example"
              className="w-full px-3 py-2 rounded text-sm"
              style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
            />
          </label>
        </div>

        {formError && (
          <p className="flex items-center gap-2 text-xs text-red-400"><AlertCircle size={14} />{formError}</p>
        )}
        {notice && (
          <p className="flex items-center gap-2 text-xs text-green-400"><CheckCircle2 size={14} />{notice}</p>
        )}

        <button type="submit" disabled={adding} className="btn-primary px-4 py-2 text-sm disabled:opacity-60">
          {adding ? 'Adding…' : 'Add domain'}
        </button>
      </form>

      {pending.map((d) => (
        <div key={d.id} className="card p-5 space-y-3">
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{d.domain}</h2>
            <span className="badge badge-yellow">Verify ownership</span>
          </div>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Add this DNS record at your DNS provider. It can take a few minutes to appear, then verify.
          </p>
          <div className="space-y-2 font-mono text-xs">
            <div className="flex flex-wrap items-center gap-2">
              <span style={{ color: 'var(--text-muted)' }}>{d.verification!.recordType} name</span>
              <CopyValue value={d.verification!.recordName} />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span style={{ color: 'var(--text-muted)' }}>value</span>
              <CopyValue value={d.verification!.recordValue} />
            </div>
          </div>
          <button
            type="button"
            onClick={() => onVerify(d.id)}
            disabled={verifyingId === d.id}
            className="btn-secondary px-4 py-2 text-sm disabled:opacity-60"
          >
            {verifyingId === d.id ? 'Checking DNS…' : 'Verify ownership'}
          </button>
          {verifyResult?.id === d.id && (
            <p className={`text-xs ${verifyResult.ok ? 'text-green-400' : 'text-red-400'}`}>{verifyResult.message}</p>
          )}
        </div>
      ))}

      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center gap-2" style={{ borderColor: 'var(--border-primary)' }}>
          <Globe size={16} style={{ color: 'var(--text-muted)' }} />
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Your domains</h2>
        </div>
        {domainsLoading ? (
          <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>Loading domains…</p>
        ) : domains.length === 0 ? (
          <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>
            No domain is connected to this account. Add one above to get started.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="data-table">
              <thead><tr><th>Domain</th><th>Origin</th><th>Status</th></tr></thead>
              <tbody>
                {domains.map((d) => {
                  const state = domainState(d.status);
                  return (
                    <tr key={d.id}>
                      <td className="text-sm font-medium">{d.domain}</td>
                      <td className="font-mono text-xs">{d.origin}</td>
                      <td>
                        <span className={`badge ${state.protected ? 'badge-green' : 'badge-yellow'}`}>{state.label}</span>
                        <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>{state.detail}</p>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
