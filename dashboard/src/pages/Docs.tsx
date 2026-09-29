import { useTheme } from '../context/ThemeContext';
import { Sun, Moon } from 'lucide-react';

export default function Docs() {
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
            <a href="/landing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Home</a>
          </div>
        </div>
      </nav>

      <div className="max-w-5xl mx-auto px-6 py-16">
        <div className="grid md:grid-cols-4 gap-12">
          {/* Sidebar */}
          <div className="md:col-span-1">
            <nav className="space-y-6 sticky top-20">
              <div>
                <p className="text-xs font-mono mb-2" style={{ color: 'var(--text-muted)' }}>OVERVIEW</p>
                <ul className="space-y-1 text-sm">
                  <li><a href="#introduction" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>Introduction</a></li>
                  <li><a href="#how-it-works" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>How it works</a></li>
                  <li><a href="#quickstart" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>Quickstart</a></li>
                </ul>
              </div>
              <div>
                <p className="text-xs font-mono mb-2" style={{ color: 'var(--text-muted)' }}>TECHNICAL</p>
                <ul className="space-y-1 text-sm">
                  <li><a href="#ja4" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>JA4 fingerprinting</a></li>
                  <li><a href="#scoring" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>Scoring engine</a></li>
                  <li><a href="#challenge" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>JS challenge</a></li>
                  <li><a href="#shadow" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>Shadow mode</a></li>
                </ul>
              </div>
              <div>
                <p className="text-xs font-mono mb-2" style={{ color: 'var(--text-muted)' }}>API</p>
                <ul className="space-y-1 text-sm">
                  <li><a href="#dashboard-api" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>Dashboard API</a></li>
                  <li><a href="#evidence" className="hover:text-white transition-colors" style={{ color: 'var(--text-secondary)' }}>Evidence endpoint</a></li>
                </ul>
              </div>
            </nav>
          </div>

          {/* Content */}
          <div className="md:col-span-3 space-y-16">
            <section id="introduction">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>OVERVIEW</p>
              <h1 className="text-3xl font-bold mb-6 tracking-tight">Introduction</h1>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  hakaishield is an inline bot protection proxy. It sits in front of your origin server, terminates TLS, and scores every incoming request based on the TLS handshake and other signals.
                </p>
                <p>
                  Unlike traditional bot protection that reacts after the fact (parsing logs, checking blocklists), hakaishield reads the live TLS ClientHello and makes a decision on the <strong className="text-white">first request</strong> from a client it has never seen.
                </p>
                <p>
                  Every decision comes with a full evidence trail — the exact signals that fired, the score, and the reasoning. No black boxes.
                </p>
              </div>
            </section>

            <section id="how-it-works">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>OVERVIEW</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">How it works</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  When a client connects to hakaishield, the following happens:
                </p>
                <ol className="list-decimal list-inside space-y-2 ml-4">
                  <li><strong className="text-white">TLS termination:</strong> hakaishield terminates the TLS connection and captures the raw ClientHello.</li>
                  <li><strong className="text-white">Fingerprinting:</strong> The ClientHello is parsed and a JA4 fingerprint is generated.</li>
                  <li><strong className="text-white">Signal collection:</strong> Multiple signals are collected (UA consistency, TLS fragmentation, etc.).</li>
                  <li><strong className="text-white">Scoring:</strong> Signals are combined into a single score (0-100).</li>
                  <li><strong className="text-white">Decision:</strong> Based on configurable thresholds, the request is allowed, challenged, or blocked.</li>
                  <li><strong className="text-white">Forwarding:</strong> If allowed, the request is forwarded to your origin with hakaishield headers added.</li>
                </ol>
              </div>
            </section>

            <section id="quickstart">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>OVERVIEW</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">Quickstart</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  To protect your site with hakaishield:
                </p>
                <div className="font-mono text-sm p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p style={{ color: 'var(--text-muted)' }}># 1. Point your DNS to hakaishield</p>
                  <p>api.yoursite.com → A → your HakaiShield proxy IP</p>
                  <p className="mt-4" style={{ color: 'var(--text-muted)' }}># 2. Verify domain ownership</p>
                  <p>We confirm ownership and issue the TLS certificate with you</p>
                  <p className="mt-4" style={{ color: 'var(--text-muted)' }}># 3. Start with shadow mode</p>
                  <p>See what we'd block before enforcing anything</p>
                </div>
                <p>
                  No code changes, no SDK, no install. During the pilot we do this setup with you.
                </p>
              </div>
            </section>

            <section id="ja4">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>TECHNICAL</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">JA4 fingerprinting</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  JA4 is a TLS fingerprinting method that captures the characteristics of a TLS ClientHello in a compact, hashable format.
                </p>
                <p>
                  A JA4 fingerprint looks like this:
                </p>
                <div className="font-mono text-sm p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  t13d1516h2_8daaf6152771_0271d189196b
                </div>
                <p>
                  This encodes the TLS version, cipher suites, extensions, and other handshake parameters. Different browsers and bot libraries produce different fingerprints, making it possible to distinguish real browsers from automated clients.
                </p>
                <p>
                  hakaishield captures the raw ClientHello, generates the JA4 fingerprint, and uses it as one of many signals in the scoring engine.
                </p>
              </div>
            </section>

            <section id="scoring">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>TECHNICAL</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">Scoring engine</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  The scoring engine combines multiple signals into a single score (0-100). Higher scores indicate higher likelihood of being a bot.
                </p>
                <p>
                  Current signals include:
                </p>
                <ul className="list-disc list-inside space-y-1 ml-4">
                  <li><strong className="text-white">JA4 fragmentation:</strong> Real browsers don't fragment their ClientHello. Bots sometimes do.</li>
                  <li><strong className="text-white">UA mismatch:</strong> The User-Agent claims Chrome, but the TLS handshake says otherwise.</li>
                  <li><strong className="text-white">Header order:</strong> Non-browser-like header ordering.</li>
                </ul>
                <p>
                  The score is compared against configurable thresholds:
                </p>
                <div className="font-mono text-sm p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p>Score &lt; 50 → <span className="text-green-400">PASS</span></p>
                  <p>Score 50-90 → <span className="text-yellow-400">CHALLENGE</span></p>
                  <p>Score &gt; 90 → <span className="text-red-400">BLOCK</span></p>
                </div>
              </div>
            </section>

            <section id="challenge">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>TECHNICAL</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">JS challenge</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  When a request scores in the challenge range, hakaishield serves a JS challenge instead of forwarding to the origin.
                </p>
                <p>
                  The challenge requires the client to:
                </p>
                <ol className="list-decimal list-inside space-y-2 ml-4">
                  <li>Parse the challenge page</li>
                  <li>Compute the SHA-256 of a server-issued nonce</li>
                  <li>Render a canvas fingerprint</li>
                  <li>Submit the proof back to hakaishield</li>
                </ol>
                <p>
                  On success, a signed cookie is set and the client is redirected back to the original page. This raises the bar beyond plain HTTP clients.
                </p>
              </div>
            </section>

            <section id="shadow">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>TECHNICAL</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">Shadow mode</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  Shadow mode runs the full scoring pipeline but doesn't block anything. Every request is scored and logged, but all requests are forwarded to the origin.
                </p>
                <p>
                  This is the safe way to test hakaishield against real traffic. You can see exactly what it would block, measure false positives, and tune thresholds before enforcing.
                </p>
                <p>
                  Shadow mode is clearly indicated in the dashboard and logs, so you never accidentally think you're blocking when you're not.
                </p>
              </div>
            </section>

            <section id="dashboard-api">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>API</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">Dashboard API</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  hakaishield exposes two read-only endpoints for the dashboard:
                </p>
                <div className="font-mono text-sm p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p>GET /api/v1/dashboard/stats</p>
                  <p className="mt-2" style={{ color: 'var(--text-muted)' }}># Returns running totals: requests seen, passed, challenged, blocked</p>
                  <p className="mt-4">GET /api/v1/dashboard/evidence</p>
                  <p className="mt-2" style={{ color: 'var(--text-muted)' }}># Returns the last 1000 decisions with full details</p>
                  <p className="mt-2" style={{ color: 'var(--text-muted)' }}># Requires Authorization: Bearer &lt;token&gt;</p>
                </div>
              </div>
            </section>

            <section id="evidence">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>API</p>
              <h2 className="text-2xl font-bold mb-6 tracking-tight">Evidence endpoint</h2>
              <div className="space-y-4 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                <p>
                  The evidence endpoint returns the last 1000 decisions (24h max), newest first. Each entry includes:
                </p>
                <ul className="list-disc list-inside space-y-1 ml-4">
                  <li>Timestamp</li>
                  <li>JA4 fingerprint</li>
                  <li>Signals fired</li>
                  <li>Score</li>
                  <li>Decision (pass/challenge/block)</li>
                  <li>Whether it was enforced</li>
                </ul>
                <p>
                  This is the full audit trail. Every decision is logged with the exact reasoning.
                </p>
              </div>
            </section>
          </div>
        </div>
      </div>
    </div>
  );
}
