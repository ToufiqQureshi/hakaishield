import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight } from 'lucide-react';
import { supabase } from '../lib/supabaseClient';

export default function SignUp() {
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    name: '',
    email: '',
    password: '',
    company: '',
  });
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [checkEmail, setCheckEmail] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const { data, error: signUpError } = await supabase.auth.signUp({
        email: formData.email,
        password: formData.password,
        options: {
          data: { name: formData.name, company: formData.company || undefined },
        },
      });
      if (signUpError) throw signUpError;
      if (data.session) {
        // Email confirmation is off for this project, or Supabase
        // auto-confirmed — a usable session came back immediately.
        navigate('/onboarding');
      } else {
        // Confirmation email sent; there's no session until the user
        // clicks the link, so there's nothing to redirect into yet.
        setCheckEmail(true);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create account. Try again.');
    } finally {
      setSubmitting(false);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  return (
    <div className="min-h-screen flex items-center justify-center py-12" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
        </div>

        <div className="card p-8">
          {checkEmail ? (
            <>
              <h1 className="text-2xl font-bold mb-2 tracking-tight">Check your email</h1>
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                We sent a confirmation link to <strong>{formData.email}</strong>. Click it to activate your
                account, then <a href="/sign-in" className="underline" style={{ color: 'var(--accent-blue)' }}>sign in</a>.
              </p>
            </>
          ) : (
          <>
          <h1 className="text-2xl font-bold mb-2 tracking-tight">Create your account</h1>
          <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
            Create your account, then we set up your domain with you
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Full name</label>
              <input
                type="text"
                name="name"
                value={formData.name}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="John Doe"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Work email</label>
              <input
                type="email"
                name="email"
                value={formData.email}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="you@company.com"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Password</label>
              <input
                type="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                required
                minLength={8}
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="At least 8 characters"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Company name (optional)</label>
              <input
                type="text"
                name="company"
                value={formData.company}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="Acme Inc."
              />
            </div>

            <div className="flex items-start gap-2 pt-2">
              <input type="checkbox" id="terms" required className="mt-1 rounded" />
              <label htmlFor="terms" className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                I agree to the{' '}
                <a href="/terms" className="underline" style={{ color: 'var(--accent-blue)' }}>Terms of Service</a>
                {' '}and{' '}
                <a href="/privacy" className="underline" style={{ color: 'var(--accent-blue)' }}>Privacy Policy</a>
              </label>
            </div>

            {error && (
              <p className="text-sm" style={{ color: 'var(--accent-red, #ef4444)' }}>{error}</p>
            )}

            <button type="submit" disabled={submitting} className="group btn-primary w-full py-2.5 text-sm flex items-center justify-center gap-2 disabled:opacity-60">
              {submitting ? 'Creating account…' : 'Create account'}
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </button>
          </form>

          <div className="mt-6 pt-6 border-t text-center" style={{ borderColor: 'var(--border-primary)' }}>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Already have an account?{' '}
              <a href="/sign-in" className="font-medium" style={{ color: 'var(--accent-blue)' }}>Sign in</a>
            </p>
          </div>
          </>
          )}
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
