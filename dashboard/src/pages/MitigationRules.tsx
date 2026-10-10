import { useEffect, useState, type FormEvent } from 'react';
import { AlertCircle, Lock, Plus, ToggleLeft, ToggleRight, Trash2 } from 'lucide-react';
import { ApiError, createRule, listRules, setRuleEnabled, type CustomRule, type ManagedRule, type RuleCondition } from '../lib/api';

const fields = ['JA4 Fingerprint', 'IP Address', 'Request Path', 'Request Method', 'Threat Score'] as const;
const actions = ['BLOCK', 'CHALLENGE', 'LOG', 'RATE_LIMIT', 'PASS'] as const;
const operators: Record<string, string[]> = {
  'JA4 Fingerprint': ['EQUALS', 'CONTAINS', 'MATCHES'],
  'IP Address': ['EQUALS', 'CONTAINS', 'MATCHES'],
  'Request Path': ['EQUALS', 'CONTAINS', 'MATCHES'],
  'Request Method': ['EQUALS'],
  'Threat Score': ['EQUALS', '>', '<', '>=', '<='],
};
const blankCondition = (): RuleCondition => ({ field: fields[0], operator: 'EQUALS', value: '' });

export default function MitigationRules() {
  const [managed, setManaged] = useState<ManagedRule[]>([]);
  const [custom, setCustom] = useState<CustomRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [builderOpen, setBuilderOpen] = useState(false);
  const [name, setName] = useState('');
  const [conditions, setConditions] = useState<RuleCondition[]>([blankCondition()]);
  const [action, setAction] = useState<(typeof actions)[number]>('BLOCK');
  const [saving, setSaving] = useState(false);
  const [changing, setChanging] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listRules().then((data) => {
      if (!cancelled) { setManaged(data.managedRules); setCustom(data.customRules); }
    }).catch((err) => {
      if (!cancelled) setError(err instanceof ApiError ? err.message : 'Could not load rules.');
    }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  const updateCondition = (index: number, patch: Partial<RuleCondition>) => {
    setConditions((previous) => previous.map((condition, i) => {
      if (i !== index) return condition;
      const next = { ...condition, ...patch };
      if (patch.field) next.operator = operators[patch.field][0];
      return next;
    }));
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setSaving(true);
    try {
      const saved = await createRule(name.trim(), conditions.map((item) => ({ ...item, value: item.value.trim() })), action);
      setCustom((previous) => [saved, ...previous]);
      setName(''); setConditions([blankCondition()]); setAction('BLOCK'); setBuilderOpen(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not save the rule.');
    } finally { setSaving(false); }
  };

  const toggle = async (rule: CustomRule) => {
    setError(null);
    setChanging(rule.id);
    try {
      await setRuleEnabled(rule.id, !rule.enabled);
      setCustom((previous) => previous.map((item) => item.id === rule.id ? { ...item, enabled: !item.enabled } : item));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not update the rule.');
    } finally { setChanging(null); }
  };

  return <div className="space-y-6 animate-in">
    <div className="flex items-start justify-between gap-4">
      <div><h1 className="text-lg font-semibold">Mitigation rules</h1><p className="text-sm" style={{ color: 'var(--text-muted)' }}>Inspect built-in detection and prepare your own rules.</p></div>
      <button type="button" onClick={() => setBuilderOpen((open) => !open)} className="btn-primary text-xs flex items-center gap-1.5"><Plus size={12} /> Custom rule</button>
    </div>
    <div className="card p-4 flex items-start gap-3">
      <AlertCircle size={18} className="text-yellow-400 shrink-0 mt-0.5" />
      <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Custom account rules run in shadow only. Enabling a rule records its proposed matches; it never changes live traffic. Domain policy activation is a separate workflow and is not available here yet.</p>
    </div>
    {error && <p role="alert" className="card p-4 text-sm text-red-400">{error}</p>}
    {builderOpen && <form onSubmit={save} className="card p-5 space-y-5 animate-in">
      <div className="flex items-center justify-between"><h2 className="text-sm font-semibold">Create custom rule</h2><button type="button" onClick={() => setBuilderOpen(false)} aria-label="Close rule builder">×</button></div>
      <label className="block text-xs" style={{ color: 'var(--text-muted)' }}>Rule name
        <input value={name} onChange={(event) => setName(event.target.value)} required maxLength={128} placeholder="Block suspicious fingerprints" className="block w-full mt-1 px-3 py-2 text-sm rounded" />
      </label>
      <div className="space-y-2"><p className="text-xs" style={{ color: 'var(--text-muted)' }}>Match all conditions</p>
        {conditions.map((item, index) => <div key={index} className="flex flex-wrap sm:flex-nowrap items-center gap-2">
          <span className="text-xs w-8" style={{ color: 'var(--text-muted)' }}>{index === 0 ? 'IF' : 'AND'}</span>
          <select aria-label={`Condition ${index + 1} field`} value={item.field} onChange={(event) => updateCondition(index, { field: event.target.value })} className="px-2 py-2 text-xs rounded">{fields.map((field) => <option key={field}>{field}</option>)}</select>
          <select aria-label={`Condition ${index + 1} operator`} value={item.operator} onChange={(event) => updateCondition(index, { operator: event.target.value })} className="px-2 py-2 text-xs rounded">{operators[item.field].map((operator) => <option key={operator}>{operator}</option>)}</select>
          <input aria-label={`Condition ${index + 1} value`} value={item.value} onChange={(event) => updateCondition(index, { value: event.target.value })} required maxLength={512} placeholder="Value" className="min-w-0 flex-1 px-2 py-2 text-xs rounded font-mono" />
          {conditions.length > 1 && <button type="button" aria-label={`Remove condition ${index + 1}`} onClick={() => setConditions((previous) => previous.filter((_, i) => i !== index))} className="text-red-400"><Trash2 size={14} /></button>}
        </div>)}
        <button type="button" disabled={conditions.length >= 16} onClick={() => setConditions((previous) => [...previous, blankCondition()])} className="text-xs text-blue-400 flex items-center gap-1 disabled:opacity-50"><Plus size={12} /> Add condition</button>
      </div>
      <div><p className="text-xs mb-2" style={{ color: 'var(--text-muted)' }}>Then</p><div className="flex flex-wrap gap-2">{actions.map((candidate) => <button key={candidate} type="button" onClick={() => setAction(candidate)} aria-pressed={action === candidate} className={`px-3 py-1.5 text-xs rounded border ${action === candidate ? 'border-blue-400 text-blue-400' : 'border-gray-700'}`}>{candidate.replace('_', ' ')}</button>)}</div></div>
      <div className="rounded p-3 font-mono text-xs break-words" style={{ background: 'var(--code-bg)', border: '1px solid var(--border-primary)' }}>IF {conditions.map((item) => `${item.field} ${item.operator} ${item.value || '…'}`).join(' AND ')} THEN {action}</div>
      <div className="flex justify-end gap-2"><button type="button" onClick={() => setBuilderOpen(false)} className="btn-secondary text-xs">Cancel</button><button type="submit" disabled={saving} className="btn-primary text-xs disabled:opacity-50">{saving ? 'Saving…' : 'Save shadow draft'}</button></div>
    </form>}
    <div className="card overflow-hidden">
      <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}><div><h2 className="text-sm font-semibold">Managed detection</h2><p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Built-in checks maintained by HakaiShield</p></div><span className="badge badge-blue">{managed.filter((rule) => rule.enabled).length} enabled</span></div>
      {loading ? <p className="p-5 text-xs">Loading…</p> : managed.length === 0 ? <p className="p-5 text-xs">No managed checks available.</p> : managed.map((rule) => <div key={rule.id} className="px-5 py-3 border-b flex items-start gap-3" style={{ borderColor: 'var(--border-primary)' }}><Lock size={16} className="shrink-0 mt-0.5" /><div><p className="text-sm font-medium">{rule.name}</p><p className="text-xs" style={{ color: 'var(--text-muted)' }}>{rule.description}</p></div></div>)}
    </div>
    <div className="card overflow-hidden">
      <div className="px-5 py-4 border-b" style={{ borderColor: 'var(--border-primary)' }}><h2 className="text-sm font-semibold">Custom rule drafts</h2><p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Enabling a rule adds it to shadow evaluation for your account.</p></div>
      {custom.length === 0 ? <p className="p-5 text-xs">No custom rules saved yet.</p> : custom.map((rule) => <div key={rule.id} className="px-5 py-3 border-b flex items-center justify-between gap-3" style={{ borderColor: 'var(--border-primary)' }}><div className="min-w-0"><p className="text-sm font-medium">{rule.name}</p><p className="text-xs font-mono break-all" style={{ color: 'var(--text-muted)' }}>{rule.conditions.map((item) => `${item.field} ${item.operator} ${item.value}`).join(' AND ')} → {rule.action}</p></div><button type="button" onClick={() => toggle(rule)} disabled={changing === rule.id} aria-label={`${rule.enabled ? 'Disable' : 'Enable'} ${rule.name} draft`} aria-pressed={rule.enabled} className="shrink-0 disabled:opacity-50">{rule.enabled ? <ToggleRight size={24} className="text-blue-400" /> : <ToggleLeft size={24} />}</button></div>)}
    </div>
  </div>;
}
