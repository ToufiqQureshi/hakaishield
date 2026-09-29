import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight, Sun, Moon } from 'lucide-react';

export default function Pricing() {
  const { theme, toggleTheme } = useTheme();
  const [openFaq, setOpenFaq] = useState<number | null>(null);

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
            <a href="/" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Dashboard</a>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="max-w-5xl mx-auto px-6 pt-24 pb-16">
        <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
          // pricing
        </p>
        <h1 className="text-4xl md:text-5xl font-bold mb-6 tracking-tight max-w-2xl">
          Simple pricing.<br />No surprises.
        </h1>
        <p className="text-lg max-w-xl" style={{ color: 'var(--text-secondary)' }}>
          Every plan includes a bandwidth cap. Start with shadow mode — see what we'd block before enforcing anything.
        </p>
      </section>

      {/* Plans */}
      <section className="max-w-5xl mx-auto px-6 pb-20">
        <div className="grid md:grid-cols-3 gap-px" style={{ background: 'var(--border-primary)' }}>
          {/* Starter */}
          <div className="p-8" style={{ background: 'var(--bg-primary)' }}>
            <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-muted)' }}>STARTER</p>
            <div className="mb-6">
              <span className="text-4xl font-bold">$200</span>
              <span className="text-sm" style={{ color: 'var(--text-muted)' }}>/mo</span>
            </div>
            <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
              For growing sites that need bot protection now.
            </p>
            <a href="/contact" className="btn-secondary w-full py-2.5 text-sm text-center block mb-8">
              Request a pilot
            </a>
            <ul className="space-y-3 text-sm" style={{ color: 'var(--text-secondary)' }}>
              <li>1M requests/month</li>
              <li>3 protected domains</li>
              <li>JA4 TLS fingerprinting</li>
              <li>Multi-layer scoring</li>
              <li>Shadow mode</li>
              <li>Evidence trail (24h)</li>
              <li>JS challenge</li>
              <li>Email support</li>
            </ul>
          </div>

          {/* Growth */}
          <div className="p-8 relative" style={{ background: 'var(--bg-tertiary)' }}>
            <div className="absolute top-4 right-4">
              <span className="text-[10px] font-mono px-2 py-0.5 rounded" style={{ background: 'var(--accent-blue)', color: 'white' }}>POPULAR</span>
            </div>
            <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-muted)' }}>GROWTH</p>
            <div className="mb-6">
              <span className="text-4xl font-bold">$500</span>
              <span className="text-sm" style={{ color: 'var(--text-muted)' }}>/mo</span>
            </div>
            <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
              For businesses where bots are a revenue leak.
            </p>
            <a href="/contact" className="btn-primary w-full py-2.5 text-sm text-center block mb-8">
              Request a pilot
            </a>
            <ul className="space-y-3 text-sm" style={{ color: 'var(--text-secondary)' }}>
              <li>10M requests/month</li>
              <li>10 protected domains</li>
              <li>Everything in Starter</li>
              <li>Behavioral scoring</li>
              <li>Deception engine</li>
              <li>Evidence trail (30 days)</li>
              <li>Custom rules</li>
              <li>SIEM integrations (roadmap)</li>
              <li>Rate limiting per endpoint</li>
              <li>Priority support</li>
            </ul>
          </div>

          {/* Enterprise */}
          <div className="p-8" style={{ background: 'var(--bg-primary)' }}>
            <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-muted)' }}>ENTERPRISE</p>
            <div className="mb-6">
              <span className="text-4xl font-bold">Custom</span>
            </div>
            <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
              Self-hosted. For regulated industries.
            </p>
            <a href="/contact" className="btn-secondary w-full py-2.5 text-sm text-center block mb-8">
              Talk to us
            </a>
            <ul className="space-y-3 text-sm" style={{ color: 'var(--text-secondary)' }}>
              <li>Unlimited requests</li>
              <li>Unlimited domains</li>
              <li>Everything in Growth</li>
              <li>Self-hosted deployment</li>
              <li>Data residency</li>
              <li>SSO / SAML (roadmap)</li>
              <li>Dedicated account manager</li>
              <li>SLA guarantee</li>
              <li>Custom integrations</li>
              <li>Fingerprint database access</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Comparison */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-20">
          <div className="max-w-2xl mb-12">
            <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
              // comparison
            </p>
            <h2 className="text-3xl font-bold tracking-tight">
              Why not the alternatives?
            </h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                  <th className="text-left py-3 pr-4 font-medium" style={{ color: 'var(--text-muted)' }}></th>
                  <th className="text-center py-3 px-4 font-semibold">hakaishield</th>
                  <th className="text-center py-3 px-4" style={{ color: 'var(--text-muted)' }}>Enterprise</th>
                  <th className="text-center py-3 px-4" style={{ color: 'var(--text-muted)' }}>Free / OSS</th>
                </tr>
              </thead>
              <tbody style={{ color: 'var(--text-secondary)' }}>
                <tr className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                  <td className="py-3 pr-4">First-request detection</td>
                  <td className="text-center text-green-400">✓</td>
                  <td className="text-center text-green-400">✓</td>
                  <td className="text-center text-red-400">✕</td>
                </tr>
                <tr className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                  <td className="py-3 pr-4">Evidence trail</td>
                  <td className="text-center text-green-400">✓</td>
                  <td className="text-center" style={{ color: 'var(--text-muted)' }}>Partial</td>
                  <td className="text-center text-red-400">✕</td>
                </tr>
                <tr className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                  <td className="py-3 pr-4">Shadow mode</td>
                  <td className="text-center text-green-400">✓</td>
                  <td className="text-center text-red-400">✕</td>
                  <td className="text-center text-red-400">✕</td>
                </tr>
                <tr className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                  <td className="py-3 pr-4">Self-hosted option</td>
                  <td className="text-center text-green-400">✓</td>
                  <td className="text-center text-red-400">✕</td>
                  <td className="text-center text-green-400">✓</td>
                </tr>
                <tr className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                  <td className="py-3 pr-4">Setup time</td>
                  <td className="text-center font-mono text-xs text-green-400">Guided, days</td>
                  <td className="text-center font-mono text-xs">Weeks</td>
                  <td className="text-center font-mono text-xs">Hours</td>
                </tr>
                <tr>
                  <td className="py-3 pr-4">Starting price</td>
                  <td className="text-center font-mono text-xs text-green-400">$200/mo</td>
                  <td className="text-center font-mono text-xs">$1,500+/mo</td>
                  <td className="text-center font-mono text-xs">$0</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-3xl mx-auto px-6 py-20">
          <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
            // faq
          </p>
          <h2 className="text-3xl font-bold mb-10 tracking-tight">Questions.</h2>
          <div className="space-y-0">
            {[
              { q: 'How long does onboarding take?', a: 'Onboarding is guided during the pilot. We verify domain ownership, set up TLS, connect your origin and run shadow mode with you. No code changes, no SDK, no install.' },
              { q: 'What if hakaishield blocks a real user?', a: 'Shadow mode lets you run hakaishield without blocking anything first. See exactly what it would block, tune thresholds, then enforce. Every decision comes with a full evidence trail.' },
              { q: 'How is this different from Cloudflare?', a: 'Cloudflare is a CDN that happens to have bot detection. hakaishield is purpose-built — we read the live TLS handshake and score the first request, not react to logs after the fact.' },
              { q: 'Do you support HTTP/2?', a: 'Yes. HakaiShield serves HTTP/2 and records the HTTP/2 connection fingerprint as evidence alongside JA4.' },
              { q: 'What about false positives?', a: 'Our scoring engine uses multiple signals — no single check can trigger a block. Shadow mode lets you measure false positives against real traffic before enforcing.' },
              { q: 'Can I self-host?', a: 'Yes — that\'s the Enterprise plan. Same binary, your infrastructure. For teams in regulated sectors who can\'t send traffic to our cloud.' },
            ].map((faq, i) => (
              <div key={i} className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
                <button
                  onClick={() => setOpenFaq(openFaq === i ? null : i)}
                  className="w-full text-left py-5 flex items-center justify-between"
                >
                  <span className="text-sm font-medium">{faq.q}</span>
                  <span className="text-lg" style={{ color: 'var(--text-muted)' }}>
                    {openFaq === i ? '−' : '+'}
                  </span>
                </button>
                {openFaq === i && (
                  <p className="text-sm pb-5 pr-8" style={{ color: 'var(--text-secondary)' }}>{faq.a}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-24 text-center">
          <h2 className="text-3xl md:text-4xl font-bold mb-4 tracking-tight">Ready?</h2>
          <p className="text-lg mb-8" style={{ color: 'var(--text-secondary)' }}>
            No credit card. Start in shadow mode and see what we find.
          </p>
          <a href="/contact" className="group btn-primary px-8 py-3 text-sm inline-flex items-center gap-2">
            Request a pilot
            <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
          </a>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t py-8" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 flex items-center justify-between">
          <span className="text-sm font-bold tracking-tight">hakaishield</span>
          <div className="flex gap-6 text-xs" style={{ color: 'var(--text-muted)' }}>
            <a href="/landing">Home</a>
            <a href="/">Dashboard</a>
            <a href="/docs">Docs</a>
            <span>© 2026</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
