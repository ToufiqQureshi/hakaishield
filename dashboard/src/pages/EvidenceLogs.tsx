import { useEffect, useMemo, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Search, Filter, ChevronDown, ChevronUp, AlertCircle } from 'lucide-react';
import { getEvidenceLogs, ApiError, type EvidenceEntry } from '../lib/api';
import EvidenceSignals from '../components/EvidenceSignals';
import type { LayoutContext } from '../components/Layout';

const decisions = ['allow', 'challenge', 'block', 'deceive', 'rate_limit'];

function getDecisionBadge(decision: string) {
  switch (decision) {
    case 'allow': return 'badge-green';
    case 'block': return 'badge-red';
    case 'challenge': return 'badge-yellow';
    case 'deceive': return 'badge-orange';
    case 'rate_limit': return 'badge-orange';
    default: return 'badge-gray';
  }
}

function getScoreColor(score: number) {
  if (score >= 80) return 'text-red-400';
  if (score >= 50) return 'text-yellow-400';
  if (score >= 30) return 'text-orange-400';
  return 'text-green-400';
}

export default function EvidenceLogs() {
  const { selectedDomain } = useOutletContext<LayoutContext>();
  const [logs, setLogs] = useState<EvidenceEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [decisionFilter, setDecisionFilter] = useState<string>('ALL');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');

  useEffect(() => {
    const controller = new AbortController();
    setLogs([]);
    setError(null);
    if (!selectedDomain) {
      setLoading(false);
      return () => controller.abort();
    }
    setLoading(true);
    getEvidenceLogs(selectedDomain.id, controller.signal)
      .then((entries) => {
        if (!controller.signal.aborted) setLogs(entries);
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(err instanceof ApiError ? err.message : 'Could not load evidence logs.');
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [selectedDomain]);

  const filteredLogs = useMemo(() => {
    const filtered = logs.filter((log) => {
      const matchesSearch = searchQuery === '' || log.ja4.toLowerCase().includes(searchQuery.toLowerCase());
      const matchesDecision = decisionFilter === 'ALL' || log.decision === decisionFilter;
      return matchesSearch && matchesDecision;
    });
    filtered.sort((a, b) => {
      const diff = new Date(a.time).getTime() - new Date(b.time).getTime();
      return sortDir === 'desc' ? -diff : diff;
    });
    return filtered;
  }, [logs, searchQuery, decisionFilter, sortDir]);

  return (
    <div className="space-y-4 animate-in">
      {/* Header */}
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Evidence Logs</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>The last 1,000 decisions (up to 24h) for the selected domain</p>
      </div>

      {error && (
        <div className="card p-4 flex items-center gap-2">
          <AlertCircle size={14} className="text-red-400" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>{error}</p>
        </div>
      )}

      {/* Filters Bar */}
      <div className="card p-3">
        <div className="flex flex-col md:flex-row gap-3">
          <div className="flex-1 relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--text-muted)' }} />
            <input
              type="text"
              placeholder="Search by JA4 fingerprint..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-9 pr-3 py-2 text-sm rounded-md focus:outline-none focus:ring-1"
              style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
            />
          </div>
          <div className="relative">
            <select
              value={decisionFilter}
              onChange={(e) => setDecisionFilter(e.target.value)}
              className="appearance-none pl-3 pr-8 py-2 text-sm rounded-md focus:outline-none cursor-pointer"
              style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
            >
              <option value="ALL">All Decisions</option>
              {decisions.map(d => <option key={d} value={d}>{d}</option>)}
            </select>
            <Filter size={12} className="absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none" style={{ color: 'var(--text-muted)' }} />
          </div>
        </div>
      </div>

      {/* Log Table */}
      <div className="card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th className="cursor-pointer select-none" onClick={() => setSortDir(sortDir === 'desc' ? 'asc' : 'desc')}>
                  <div className="flex items-center gap-1">
                    Timestamp
                    {sortDir === 'desc' ? <ChevronDown size={12} /> : <ChevronUp size={12} />}
                  </div>
                </th>
                <th>JA4 Fingerprint</th>
                <th>Scored / observed signals</th>
                <th>Score</th>
                <th>Decision</th>
                <th>Enforced</th>
              </tr>
            </thead>
            <tbody>
              {!loading && filteredLogs.length === 0 && (
                <tr>
                  <td colSpan={6} className="text-center text-xs py-6" style={{ color: 'var(--text-muted)' }}>
                    {selectedDomain ? 'No decisions recorded yet.' : 'No domain selected.'}
                  </td>
                </tr>
              )}
              {filteredLogs.slice(0, 100).map((log, i) => (
                <tr key={i}>
                  <td className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>{new Date(log.time).toLocaleString()}</td>
                  <td className="font-mono text-[10px]" style={{ maxWidth: '260px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--text-secondary)' }}>
                    {log.ja4 || '—'}
                  </td>
                  <td><EvidenceSignals signals={log.signals} shadowSignals={log.shadowSignals} /></td>
                  <td><span className={`font-mono text-xs font-medium ${getScoreColor(log.score)}`}>{log.score}</span></td>
                  <td><span className={`badge ${getDecisionBadge(log.decision)}`}>{log.decision}</span></td>
                  <td className="text-xs" style={{ color: log.enforced ? 'var(--text-primary)' : 'var(--text-muted)' }}>
                    {log.enforced ? 'Yes' : 'Shadow'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="px-4 py-3 border-t flex items-center justify-between text-xs" style={{ borderColor: 'var(--border-primary)', color: 'var(--text-muted)' }}>
          <span>Showing {Math.min(100, filteredLogs.length)} of {filteredLogs.length} entries</span>
        </div>
      </div>
    </div>
  );
}
