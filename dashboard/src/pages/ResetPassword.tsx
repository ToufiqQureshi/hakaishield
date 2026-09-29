import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { supabase } from '../lib/supabaseClient';

// Landing page for the password-reset email. Supabase signs the user in
// from the link, so this page only has to set the new password.
export default function ResetPassword() {
  const navigate = useNavigate();
  const [hasSession, setHasSession] = useState<boolean | null>(null);
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    supabase.auth.getSession().then(({ data }) => {
      if (!cancelled) setHasSession(Boolean(data.session));
    });
    const { data: sub } = supabase.auth.onAuthStateChange((_event, session) => {
      if (!cancelled) setHasSession(Boolean(session));
    });
    return () => {
      cancelled = true;
      sub.subscription.unsubscribe();
    };
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (password !== confirm) {
      setError('The two passwords do not match.');
      return;
    }
    setSubmitting(true);
    try {
      const { error: updateError } = await supabase.auth.updateUser({ password });
      if (updateError) throw updateError;
      navigate('/', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update the password. Try again.');
    } finally {
      setSubmitting(false);
    }
  };

  const inputStyle = { background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' };

  return (
    <div className="min-h-screen flex items-center justify-center" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
        </div>
        <div className="card p-8">
          {hasSession === false ? (
            <>
              <h1 className="text-2xl font-bold mb-2 tracking-tight">This link has expired</h1>
              <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
                Reset links work once and expire after a short time.
              </p>
              <a href="/forgot-password" className="btn-primary inline-block px-5 py-2.5 text-sm">Send a new link</a>
            </>
          ) : (
            <>
              <h1 className="text-2xl font-bold mb-2 tracking-tight">Choose a new password</h1>
              <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>At least 8 characters.</p>
              <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-2" htmlFor="new-password">New password</label>
                  <input id="new-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)}
                    required minLength={8} autoComplete="new-password" className="w-full px-4 py-2.5 rounded text-sm" style={inputStyle} />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-2" htmlFor="confirm-password">Confirm password</label>
                  <input id="confirm-password" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)}
                    required minLength={8} autoComplete="new-password" className="w-full px-4 py-2.5 rounded text-sm" style={inputStyle} />
                </div>
                {error && <p role="alert" className="text-sm" style={{ color: 'var(--accent-red, #ef4444)' }}>{error}</p>}
                <button type="submit" disabled={submitting || hasSession !== true} className="group btn-primary w-full py-2.5 text-sm flex items-center justify-center gap-2 disabled:opacity-60">
                  {submitting ? 'Saving…' : 'Save new password'}
                  <ArrowRight size={14} />
                </button>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
