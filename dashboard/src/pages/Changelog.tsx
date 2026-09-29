import { useTheme } from '../context/ThemeContext';
import { Sun, Moon } from 'lucide-react';

export default function Changelog() {
  const { theme, toggleTheme } = useTheme();

  const entries = [
    {
      date: '2026-09-17',
      title: 'Shadow mode implemented',
      description: 'You can now run hakaishield in shadow mode. It scores everything, logs everything, but blocks nothing. Perfect for testing against real traffic before enforcing.',
      tags: ['feature', 'safety'],
    },
    {
      date: '2026-09-16',
      title: 'Dashboard API defined',
      description: 'Defined the dashboard API endpoints. Stats endpoint shows running totals. Evidence endpoint shows the last 1000 decisions with full details.',
      tags: ['api', 'dashboard'],
    },
    {
      date: '2026-09-15',
      title: '100% backend test coverage',
      description: 'Achieved 100% test coverage on the backend. Every function, every edge case, every error path is tested. Mutation testing confirms the tests actually catch bugs.',
      tags: ['testing', 'quality'],
    },
    {
      date: '2026-09-14',
      title: 'JS challenge with canvas proof',
      description: 'The JS challenge now requires a canvas fingerprint in addition to the SHA-256 proof. This raises the bar for bots that can run JS but don\'t have a real browser engine.',
      tags: ['feature', 'security'],
    },
    {
      date: '2026-09-13',
      title: 'Scoring engine v1',
      description: 'Multi-signal scoring engine is live. Combines JA4 fragmentation, UA mismatch, and other signals into a single score. Configurable thresholds for challenge and block.',
      tags: ['feature', 'core'],
    },
    {
      date: '2026-09-12',
      title: 'UA consistency check',
      description: 'Flags requests where the User-Agent claims a modern browser but the TLS handshake says otherwise. Catches bots that spoof UA but can\'t spoof TLS.',
      tags: ['feature', 'detection'],
    },
    {
      date: '2026-09-11',
      title: 'TLS/JA4 fingerprinting',
      description: 'hakaishield now terminates TLS, captures the ClientHello, and generates JA4 fingerprints. This is the foundation for all our detection.',
      tags: ['feature', 'core'],
    },
    {
      date: '2026-09-10',
      title: 'Reverse proxy skeleton',
      description: 'Basic reverse proxy is working. Forwards requests to the origin, adds X-HakaiShield headers. Graceful shutdown on SIGINT/SIGTERM.',
      tags: ['feature', 'core'],
    },
    {
      date: '2026-09-09',
      title: 'Project started',
      description: 'Started building hakaishield. Goal: inline bot protection that reads the TLS handshake and scores the first request. No prior sighting needed.',
      tags: ['milestone'],
    },
  ];

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
          // changelog
        </p>
        <h1 className="text-4xl font-bold mb-4 tracking-tight">What we've built.</h1>
        <p className="text-lg mb-12" style={{ color: 'var(--text-secondary)' }}>
          Every feature, every fix, every improvement. Updated as we ship.
        </p>

        <div className="space-y-8">
          {entries.map((entry, i) => (
            <div key={i} className="pb-8 border-b" style={{ borderColor: 'var(--border-primary)' }}>
              <div className="flex items-center gap-4 mb-3">
                <span className="text-sm font-mono" style={{ color: 'var(--text-muted)' }}>{entry.date}</span>
                <div className="flex gap-2">
                  {entry.tags.map(tag => (
                    <span key={tag} className="text-xs px-2 py-0.5 rounded" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-muted)' }}>
                      {tag}
                    </span>
                  ))}
                </div>
              </div>
              <h2 className="text-xl font-semibold mb-2">{entry.title}</h2>
              <p style={{ color: 'var(--text-secondary)' }}>{entry.description}</p>
            </div>
          ))}
        </div>

        <div className="mt-12 text-center">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Want to see what's next? <a href="/landing#how" className="underline hover:text-white transition-colors">Check the roadmap →</a>
          </p>
        </div>
      </div>
    </div>
  );
}
