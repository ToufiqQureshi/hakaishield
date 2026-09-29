import { useTheme } from '../context/ThemeContext';
import { contactEmail } from '../lib/contact';

export default function About() {
  const { theme, toggleTheme } = useTheme();

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-4xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/landing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Home</a>
          </div>
        </div>
      </nav>

      <div className="max-w-4xl mx-auto px-6 py-16">
        <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
          // about
        </p>
        <h1 className="text-4xl font-bold mb-6 tracking-tight">About hakaishield</h1>
        
        <div className="prose max-w-none space-y-6 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
          <p className="text-lg">
            hakaishield is an inline bot protection platform that reads the TLS handshake and scores the first request from every client it has never seen.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>Our mission</h2>
          <p>
            Bot protection today is broken. Enterprise tools cost $50K/month and take weeks to onboard. Free tools react after the damage is done. Neither works for mid-size teams losing revenue to scrapers, ticket hoarders, and API abusers.
          </p>
          <p>
            We're building something better: transparent, fast, and affordable bot protection that proves every decision it makes.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>How we're different</h2>
          <p>
            <strong style={{ color: 'var(--text-primary)' }}>First-request scoring.</strong> We read the live TLS ClientHello and make a decision on the first request. No prior sighting needed. No blocklists. No waiting.
          </p>
          <p>
            <strong style={{ color: 'var(--text-primary)' }}>Full evidence trail.</strong> Every block, every challenge, every pass comes with the exact signals that triggered it. No black boxes. No "trust us."
          </p>
          <p>
            <strong style={{ color: 'var(--text-primary)' }}>Shadow mode.</strong> Test against real traffic with zero risk. See what we'd block before enforcing anything.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>The team</h2>
          <p>
            hakaishield is built by <strong style={{ color: 'var(--text-primary)' }}>Toufiq Qureshi</strong>, a security engineer who's built infrastructure at scale and fought against bot protection as a security researcher.
          </p>
          <p>
            We're a small team obsessed with TLS fingerprints, behavioral signals, and proving decisions. We know what works, what doesn't, and what teams actually need.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>Our values</h2>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li><strong style={{ color: 'var(--text-primary)' }}>Transparency.</strong> We prove every decision. No black boxes.</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Privacy.</strong> We keep request evidence, not request bodies, and only for the retention period you agree.</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Speed.</strong> Decisions run inline with strict time budgets, and fail open rather than slow your site down.</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Honesty.</strong> We're building in public. See our changelog.</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>Contact</h2>
          <p>
            Questions? We'd love to hear from you.
          </p>
          <p>
            <a href="/contact" style={{ color: 'var(--accent-blue)' }}>Request a pilot</a>
            {contactEmail && <> or email <a href={`mailto:${contactEmail}`} style={{ color: 'var(--accent-blue)' }}>{contactEmail}</a></>}.
          </p>
        </div>
      </div>
    </div>
  );
}
