import { useTheme } from '../context/ThemeContext';
import { AlertCircle } from 'lucide-react';

// Billing isn't wired up (no Stripe integration exists — see
// docs/DECISIONS.md's dashboard-wiring entry). This page used to
// collect a real-looking card number into React state and claim
// "Secure payment processed by Stripe" while doing nothing but a
// console.log and a fake redirect — a real user could have typed a
// real card number into a form that went nowhere and lied about it
// succeeding. Replaced with an honest "not available" notice
// (CLAUDE.md Section 27) instead of collecting payment details this
// product cannot actually process.
export default function Payment() {
  const { theme, toggleTheme } = useTheme();

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/pricing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>← Back to pricing</a>
          </div>
        </div>
      </nav>

      <div className="max-w-xl mx-auto px-6 py-24 text-center">
        <AlertCircle size={32} className="mx-auto mb-4 text-yellow-400" />
        <h1 className="text-2xl font-bold mb-2 tracking-tight">Payments aren't set up yet</h1>
        <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
          Checkout is not available. We do not collect card details or activate paid protection on this page.
        </p>
        <a href="/contact" className="btn-primary inline-block px-6 py-2.5 text-sm">Contact us</a>
      </div>
    </div>
  );
}
