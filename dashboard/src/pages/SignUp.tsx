import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight } from 'lucide-react';
import { supabase } from '../lib/supabaseClient';
import { buildSignupConsent } from '../lib/legalConsent';

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
  const [canResend, setCanResend] = useState(false);
  const [resendAttempt, setResendAttempt] = useState(0);
  const [resending, setResending] = useState(false);
  const [resendMessage, setResendMessage] = useState<string | null>(null);
  const [acceptTerms, setAcceptTerms] = useState(false);
  const [productUpdates, setProductUpdates] = useState(false);

  useEffect(() => {
    if (!checkEmail) return;
    setCanResend(false);
    const timer = window.setTimeout(() => setCanResend(true), 60_000);
    return () => window.clearTimeout(timer);
  }, [checkEmail, resendAttempt]);

  const resendConfirmation = async () => {
    if (!canResend || resending) return;
    setResending(true);
    setResendMessage(null);
    setResendAttempt((attempt) => attempt + 1);
    try {
      const { error: resendError } = await supabase.auth.resend({
        type: 'signup',
        email: formData.email,
        options: { emailRedirectTo: `${window.location.origin}/sign-in` },
      });
      if (resendError) throw resendError;
      setResendMessage('Request accepted. Check your inbox and spam folder.');
    } catch (err) {
      setResendMessage(err instanceof Error ? err.message : 'Could not resend the confirmation link. Try again later.');
    } finally {
      setResending(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!acceptTerms) {
      setError('Please agree to the Terms of Service and acknowledge the Privacy Notice to create an account.');
      return;
    }
    setSubmitting(true);
    try {
      const consentRecordedAt = new Date().toISOString();
      const { data, error: signUpError } = await supabase.auth.signUp({
        email: formData.email,
        password: formData.password,
        options: {
          // Without this the confirmation link returns to Supabase's Site
          // URL, which is still http://localhost:3000 for this project.
          emailRedirectTo: `${window.location.origin}/sign-in`,
          data: {
            name: formData.name,
            company: formData.company || undefined,
            legal_consent: buildSignupConsent(productUpdates, consentRecordedAt),
          },
        },
      });
      if (signUpError) throw signUpError;
      if (data.session) {
        // Email confirmation is off for this project, or Supabase
        // auto-confirmed — a usable session came back immediately.
        navigate('/domains-siem');
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
                Check <strong>{formData.email}</strong> for a confirmation link, including your spam folder.
                Click it to activate your account, then <a href="/sign-in" className="underline" style={{ color: 'var(--accent-blue)' }}>sign in</a>.
              </p>
              <button type="button" onClick={resendConfirmation} disabled={!canResend || resending} className="btn-secondary mt-5 px-4 py-2 text-sm disabled:opacity-60">
                {resending ? 'Requesting…' : 'Resend confirmation link'}
              </button>
              {!canResend && <p className="mt-2 text-xs" style={{ color: 'var(--text-muted)' }}>You can request another link after one minute.</p>}
              {resendMessage && <p role="status" className="mt-2 text-xs" style={{ color: 'var(--text-secondary)' }}>{resendMessage}</p>}
            </>
          ) : (
          <>
          <h1 className="text-2xl font-bold mb-2 tracking-tight">Create your account</h1>
          <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
            Create your account, then add a domain you control.
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="signup-name" className="block text-sm font-medium mb-2">Full name</label>
              <input
                id="signup-name"
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
              <label htmlFor="signup-email" className="block text-sm font-medium mb-2">Work email</label>
              <input
                id="signup-email"
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
              <label htmlFor="signup-password" className="block text-sm font-medium mb-2">Password</label>
              <input
                id="signup-password"
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
              <label htmlFor="signup-company" className="block text-sm font-medium mb-2">Company name (optional)</label>
              <input
                id="signup-company"
                type="text"
                name="company"
                value={formData.company}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="Acme Inc."
              />
            </div>

            <div className="space-y-3 pt-2">
              <div className="flex items-start gap-2">
                <input type="checkbox" id="terms" required checked={acceptTerms} onChange={(event) => setAcceptTerms(event.target.checked)} className="mt-1 rounded" />
                <label htmlFor="terms" className="text-xs leading-5" style={{ color: 'var(--text-secondary)' }}>
                  I agree to the <a href="/terms" target="_blank" rel="noreferrer" className="underline" style={{ color: 'var(--accent-blue)' }}>Terms of Service</a> and acknowledge that I have read the <a href="/privacy" target="_blank" rel="noreferrer" className="underline" style={{ color: 'var(--accent-blue)' }}>Privacy Notice</a>. Required to create an account.
                </label>
              </div>
              <div className="flex items-start gap-2">
                <input type="checkbox" id="product-updates" checked={productUpdates} onChange={(event) => setProductUpdates(event.target.checked)} className="mt-1 rounded" />
                <label htmlFor="product-updates" className="text-xs leading-5" style={{ color: 'var(--text-secondary)' }}>
                  I agree to receive optional product news and marketing emails. You can withdraw this consent by contacting support; account and security messages are unaffected.
                </label>
              </div>
              <p className="text-xs leading-5" style={{ color: 'var(--text-muted)' }}>
                Account and security messages may still be sent. Also see our <a href="/acceptable-use" target="_blank" rel="noreferrer" className="underline">Acceptable Use Policy</a> and <a href="/cookies" target="_blank" rel="noreferrer" className="underline">Cookie Notice</a>.
              </p>
            </div>

            {error && (
              <p role="alert" className="text-sm" style={{ color: 'var(--accent-red, #ef4444)' }}>{error}</p>
            )}

            <button type="submit" disabled={submitting || !acceptTerms} className="group btn-primary w-full py-2.5 text-sm flex items-center justify-center gap-2 disabled:opacity-60">
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
