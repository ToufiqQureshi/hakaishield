import { useEffect, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { AlertCircle } from 'lucide-react';
import { getStats, getTopOffenders, ApiError, type DashboardStats, type TopOffender } from '../lib/api';
import type { LayoutContext } from '../components/Layout';

// formatEgress renders the tenant's cost number the way the monthly
// bill reads: MB under a gigabyte, GB above. Exported for the tests —
// this is the number a customer will question, so its formatting is
// pinned, not left to whatever the browser shows.
export function formatEgress(bytes: number): string {
  return bytes >= 1024 * 1024 * 1024
    ? `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
    : `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function Overview() {
  const { selectedDomain, domainsLoading } = useOutletContext<LayoutContext>();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [offenders, setOffenders] = useState<TopOffender[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const controller = new AbortController();
    setStats(null);
    setOffenders([]);
    setError(null);
    if (!selectedDomain) {
      setLoading(false);
      return () => controller.abort();
    }
    setLoading(true);
    Promise.all([getStats(selectedDomain.id, controller.signal), getTopOffenders(selectedDomain.id, controller.signal)])
      .then(([s, o]) => {
        if (!controller.signal.aborted) {
          setStats(s);
          setOffenders(o);
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(err instanceof ApiError ? err.message : 'Could not load dashboard data.');
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [selectedDomain]);

  if (!domainsLoading && !selectedDomain) {
    return (
      <div className="card p-6 flex items-center gap-3">
        <AlertCircle size={18} className="text-yellow-400 shrink-0" />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          No protected domain yet. <a href="/domains-siem" className="underline">View pilot setup</a> to get started.
        </p>
      </div>
    );
  }

  const metrics = stats ? [
    { label: 'Total Requests', value: stats.total_requests },
    { label: 'Passed', value: stats.passed },
    { label: 'Blocked', value: stats.blocked },
    { label: 'Challenged', value: stats.challenged },
    { label: 'Deceived', value: stats.deceived },
    { label: 'Rate Limited', value: stats.rateLimited },
  ] : [];

  // Format bytes the way the cost conversation needs: a raw byte count
  // is unreadable, and this is the number the monthly bill is built on.
  const measurements = stats ? [
    { label: 'Egress', value: formatEgress(stats.egress_bytes) },
    { label: 'Challenge Solves', value: stats.challenge_solves.toLocaleString() },
    { label: 'Challenge Failures', value: stats.challenge_failures.toLocaleString() },
  ] : [];

  return (
    <div className="space-y-6 animate-in">
      {/* Mode Banner */}
      {stats && (
        <div className="flex items-center gap-3 px-4 py-3 rounded-lg" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)' }}>
          <span className={`w-2 h-2 rounded-full flex-shrink-0 ${stats.mode === 'shadow' ? 'bg-yellow-500 animate-pulse' : 'bg-green-500'}`}></span>
          <div className="flex-1 flex items-center gap-2">
            <span className={`text-sm font-semibold ${stats.mode === 'shadow' ? 'text-yellow-500' : 'text-green-500'}`}>
              {stats.mode === 'shadow' ? 'Shadow Mode' : 'Enforcing'}
            </span>
            <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {stats.mode === 'shadow' ? 'Scoring traffic but not blocking.' : 'Actively blocking and challenging threats.'}
            </span>
          </div>
          <span className={`badge ${stats.mode === 'shadow' ? 'badge-yellow' : 'badge-green'} text-[10px] hidden sm:inline-flex`}>
            {stats.mode === 'shadow' ? 'SAFE MODE' : 'LIVE'}
          </span>
        </div>
      )}

      {error && (
        <div className="card p-4 flex items-center gap-2">
          <AlertCircle size={14} className="text-red-400" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Metrics Strip */}
      {loading ? (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading…</p>
      ) : stats && (
        <>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
            {metrics.map((m, i) => (
              <div key={i} className="card p-4">
                <p className="text-xs mb-2" style={{ color: 'var(--text-muted)' }}>{m.label}</p>
                <p className="text-2xl font-bold font-mono" style={{ color: 'var(--text-primary)' }}>{m.value.toLocaleString()}</p>
              </div>
            ))}
          </div>
          {/* P1 measurement strip: traffic cost and challenge burden. */}
          <div className="grid grid-cols-3 gap-3">
            {measurements.map((m, i) => (
              <div key={i} className="card p-4">
                <p className="text-xs mb-2" style={{ color: 'var(--text-muted)' }}>{m.label}</p>
                <p className="text-xl font-bold font-mono" style={{ color: 'var(--text-primary)' }}>{m.value}</p>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Top Offenders */}
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b" style={{ borderColor: 'var(--border-primary)' }}>
          <div>
            <p className="text-xs font-mono mb-1" style={{ color: 'var(--text-muted)' }}>// top offenders</p>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Most active JA4 fingerprints</h2>
          </div>
        </div>
        <div className="overflow-x-auto">
          {offenders.length === 0 ? (
            <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>
              {loading ? 'Loading…' : 'No decisions recorded yet for this domain.'}
            </p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>JA4 Fingerprint</th>
                  <th>Hits</th>
                  <th>Blocked</th>
                </tr>
              </thead>
              <tbody>
                {offenders.map((o) => (
                  <tr key={o.ja4}>
                    <td className="font-mono text-xs" style={{ maxWidth: '260px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{o.ja4}</td>
                    <td className="font-mono text-xs">{o.hits.toLocaleString()}</td>
                    <td className="font-mono text-xs text-red-400">{o.blocked.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
