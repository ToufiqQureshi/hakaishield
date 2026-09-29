import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight } from 'lucide-react';
import { supabase } from '../lib/supabaseClient';

export default function ForgotPassword() {
  const { theme, toggleTheme } = useTheme();
  const [email, setEmail] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const { error: resetError } = await supabase.auth.resetPasswordForEmail(email, {
        redirectTo: `${window.location.origin}/reset-password`,
      });
      if (resetError) throw resetError;
      setSubmitted(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not send reset email. Try again.');
    } finally {
      setSubmitting(false);
    }
  };

  if (submitted) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
        <div className="w-full max-w-md px-6">
          <div className="text-center mb-8">
            <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
          </div>

          <div className="card p-8 text-center">
            <div className="w-16 h-16 rounded-full mx-auto mb-6 flex items-center justify-center" style={{ background: 'rgba(34, 197, 94, 0.1)', border: '1px solid rgba(34, 197, 94, 0.2)' }}>
              <span className="text-3xl">✓</span>
            </div>
            <h1 className="text-2xl font-bold mb-2 tracking-tight">Check your email</h1>
            <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
              We've sent a password reset link to <strong>{email}</strong>
            </p>
            <p className="text-xs mb-6" style={{ color: 'var(--text-muted)' }}>
              Didn't receive the email? Check your spam folder or{' '}
              <button onClick={() => setSubmitted(false)} className="underline" style={{ color: 'var(--accent-blue)' }}>try again</button>
            </p>
            <a href="/sign-in" className="text-sm" style={{ color: 'var(--accent-blue)' }}>
              ← Back to sign in
            </a>
          </div>

          <div className="mt-6 text-center">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light mode' : 'Dark mode'}
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
        </div>

        <div className="card p-8">
          <h1 className="text-2xl font-bold mb-2 tracking-tight">Reset your password</h1>
          <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
            Enter your email and we'll send you a reset link
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="you@company.com"
              />
            </div>

            {error && (
              <p className="text-sm" style={{ color: 'var(--accent-red, #ef4444)' }}>{error}</p>
            )}

            <button type="submit" disabled={submitting} className="group btn-primary w-full py-2.5 text-sm flex items-center justify-center gap-2 disabled:opacity-60">
              {submitting ? 'Sending…' : 'Send reset link'}
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </button>
          </form>

          <div className="mt-6 pt-6 border-t text-center" style={{ borderColor: 'var(--border-primary)' }}>
            <a href="/sign-in" className="text-sm" style={{ color: 'var(--accent-blue)' }}>
              ← Back to sign in
            </a>
          </div>
        </div>

        <div className="mt-6 text-center">
          <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
            {theme === 'dark' ? 'Light mode' : 'Dark mode'}
          </button>
        </div>
      </div>
    </div>
  );
}
