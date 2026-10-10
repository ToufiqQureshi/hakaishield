import { useEffect, useState } from 'react';
import { supabase } from '../lib/supabaseClient';

export default function CommunicationPreferences() {
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');

  useEffect(() => {
    let current = true;
    supabase.rpc('get_marketing_consent').then(({ data, error }) => {
      if (!current) return;
      if (error) setMessage('Could not load saved preferences. Please try again later.');
      else setEnabled(data === true);
      setLoading(false);
    });
    return () => { current = false; };
  }, []);

  const save = async (next: boolean) => {
    setSaving(true);
    setMessage('');
    const { error } = await supabase.rpc('set_marketing_consent', { p_consented: next });
    if (error) setMessage('Could not save your preference. Please try again.');
    else {
      setEnabled(next);
      setMessage(next ? 'Product updates enabled.' : 'Marketing consent withdrawn.');
    }
    setSaving(false);
  };

  return (
    <section className="card mx-auto max-w-2xl space-y-5 p-6">
      <div>
        <h1 className="text-xl font-semibold">Email preferences</h1>
        <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>Choose whether to receive optional product news and marketing emails. Account, billing, and security messages are separate.</p>
      </div>
      <label className="flex items-start gap-3 text-sm">
        <input type="checkbox" className="mt-1" checked={enabled} disabled={loading || saving} onChange={(event) => void save(event.target.checked)} />
        Receive product news and marketing emails
      </label>
      <p role="status" aria-live="polite" className="text-sm" style={{ color: 'var(--text-secondary)' }}>{loading ? 'Loading preferences…' : saving ? 'Saving…' : message}</p>
      <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Changes are recorded with a server timestamp. See the <a className="underline" href="/privacy">Privacy Notice</a>.</p>
    </section>
  );
}
