import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight } from 'lucide-react';

export default function Landing() {
  const { theme, toggleTheme } = useTheme();
  const [visitors, setVisitors] = useState(1000000);
  const [botPct, setBotPct] = useState(30);
  const [aov, setAov] = useState(50);

  const leak = Math.floor(visitors * (botPct / 100) * 0.15 * aov / 100);
  const roi = Math.floor(((leak - 200) / 200) * 100);

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="hidden md:flex items-center gap-8 text-sm" style={{ color: 'var(--text-secondary)' }}>
            <a href="#how" className="hover:text-white transition-colors">How it works</a>
            <a href="/pricing" className="hover:text-white transition-colors">Pricing</a>
            <a href="#proof" className="hover:text-white transition-colors">Evidence</a>
          </div>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/sign-in" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Dashboard</a>
            <a href="/contact" className="btn-primary text-xs py-1.5 px-3">Request a pilot</a>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="max-w-5xl mx-auto px-6 pt-32 pb-24">
        <div className="max-w-3xl">
          <p className="text-sm mb-6 font-mono animate-fade-in-left stagger-1" style={{ color: 'var(--text-muted)' }}>
            // inline bot protection
          </p>
          <h1 className="text-5xl md:text-7xl font-bold leading-[0.95] mb-8 tracking-tight animate-fade-in-up stagger-2">
            We read the TLS<br />
            handshake. Then<br />
            we decide.
          </h1>
          <p className="text-xl md:text-2xl leading-relaxed mb-10 max-w-2xl animate-fade-in-up stagger-3" style={{ color: 'var(--text-secondary)' }}>
            hakaishield scores the first request from every client it has never seen. No prior sighting. No blocklists. No waiting.
          </p>
          <div className="flex items-center gap-6 animate-fade-in-up stagger-4">
            <a href="/contact" className="group btn-primary px-6 py-3 text-sm flex items-center gap-2 hover-lift">
              Request a pilot
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </a>
            <span className="text-sm" style={{ color: 'var(--text-muted)' }}>
              From $200/mo · shadow mode first · no card
            </span>
          </div>
        </div>

        <div className="mt-20 pt-8 border-t" style={{ borderColor: 'var(--border-primary)' }}>
          <div className="flex items-center gap-3">
            <span className="w-2 h-2 rounded-full bg-yellow-400 animate-pulse"></span>
            <span className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>Currently in development</span>
            <span className="text-sm" style={{ color: 'var(--text-muted)' }}>· Looking for 10 founding customers</span>
          </div>
        </div>
      </section>

      {/* Problem */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-24">
          <div className="grid md:grid-cols-12 gap-12">
            <div className="md:col-span-4">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
                // the problem
              </p>
              <h2 className="text-3xl font-bold tracking-tight">
                Bot protection is either too expensive or too late.
              </h2>
            </div>
            <div className="md:col-span-8 space-y-8">
              <p className="text-lg leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                Enterprise tools (Akamai, DataDome, HUMAN) cost $1,500–$50,000/month, require your traffic in their cloud, and take weeks to onboard.
              </p>
              <p className="text-lg leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                Free tools (CrowdSec, Coraza) parse server logs — they react <em>after</em> the damage is done. They need a prior sighting before they can block anything.
              </p>
              <p className="text-lg leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                Neither works for mid-size teams losing revenue to scrapers, ticket hoarders, and API abusers.
              </p>
              <div className="pt-6 border-t" style={{ borderColor: 'var(--border-primary)' }}>
                <p className="text-lg font-medium">
                  hakaishield reads the live TLS ClientHello and scores the <span className="font-bold">first request</span>. Then it tells you exactly why.
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* How it works */}
      <section id="how" className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-24">
          <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
            // how it works
          </p>
          <h2 className="text-3xl md:text-4xl font-bold mb-16 tracking-tight max-w-2xl">
            Point your DNS. We handle the rest.
          </h2>

          <div className="space-y-16">
            <div className="grid md:grid-cols-12 gap-8 items-start">
              <div className="md:col-span-1">
                <span className="text-4xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>01</span>
              </div>
              <div className="md:col-span-5">
                <h3 className="text-xl font-semibold mb-2">Point your DNS</h3>
                <p style={{ color: 'var(--text-secondary)' }}>Point your domain at your HakaiShield proxy. No code changes, no SDK, no install. We verify ownership and set up TLS with you.</p>
              </div>
              <div className="md:col-span-6">
                <div className="font-mono text-xs p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p style={{ color: 'var(--text-muted)' }}>$ dig api.yoursite.com</p>
                  <p className="mt-2">api.yoursite.com. → A → your HakaiShield proxy IP</p>
                  <p className="mt-1" style={{ color: 'var(--text-muted)' }}>→ TLS terminated by HakaiShield, not a CDN</p>
                </div>
              </div>
            </div>

            <div className="grid md:grid-cols-12 gap-8 items-start">
              <div className="md:col-span-1">
                <span className="text-4xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>02</span>
              </div>
              <div className="md:col-span-5">
                <h3 className="text-xl font-semibold mb-2">We read the handshake</h3>
                <p style={{ color: 'var(--text-secondary)' }}>Every connection's TLS ClientHello is captured and fingerprinted using JA4. We see the raw handshake before anything else.</p>
              </div>
              <div className="md:col-span-6">
                <div className="font-mono text-xs p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p style={{ color: 'var(--text-muted)' }}>// incoming TLS ClientHello</p>
                  <p>JA4: t13d1516h2_8daaf6152771_0271d189196b</p>
                  <p>TLS: 1.3 | AES_256_GCM_SHA384</p>
                  <p>SNI: api.yoursite.com</p>
                </div>
              </div>
            </div>

            <div className="grid md:grid-cols-12 gap-8 items-start">
              <div className="md:col-span-1">
                <span className="text-4xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>03</span>
              </div>
              <div className="md:col-span-5">
                <h3 className="text-xl font-semibold mb-2">Score & decide</h3>
                <p style={{ color: 'var(--text-secondary)' }}>Multi-layer scoring: TLS fingerprint, UA consistency, behavioral signals. No prior sighting needed — we decide on first contact.</p>
              </div>
              <div className="md:col-span-6">
                <div className="font-mono text-xs p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p style={{ color: 'var(--text-muted)' }}>// scoring signals</p>
                  <p><span className="text-red-400">✗</span> UA Mismatch — claims Chrome, handshake disagrees</p>
                  <p><span className="text-red-400">✗</span> TLS Fragmentation — real browsers don't fragment</p>
                  <p className="mt-2 pt-2 border-t" style={{ borderColor: 'var(--border-primary)' }}>
                    Score: <span className="text-red-400 font-bold">87/100</span> → <span className="text-red-400">BLOCK</span>
                  </p>
                </div>
              </div>
            </div>

            <div className="grid md:grid-cols-12 gap-8 items-start">
              <div className="md:col-span-1">
                <span className="text-4xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>04</span>
              </div>
              <div className="md:col-span-5">
                <h3 className="text-xl font-semibold mb-2">Act & prove</h3>
                <p style={{ color: 'var(--text-secondary)' }}>Allow, challenge, block, or deceive. Every decision comes with a full evidence trail — the exact signals, score, and reasoning.</p>
              </div>
              <div className="md:col-span-6">
                <div className="font-mono text-xs p-4 rounded" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                  <p style={{ color: 'var(--text-muted)' }}>// decision logged</p>
                  <p>IP: 185.220.101.42</p>
                  <p>Decision: BLOCK</p>
                  <p>Origin reached: <span className="text-red-400">no</span></p>
                  <p>Latency added: 2.3ms</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Evidence */}
      <section id="proof" className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-24">
          <div className="grid md:grid-cols-12 gap-12">
            <div className="md:col-span-5">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
                // evidence trail
              </p>
              <h2 className="text-3xl md:text-4xl font-bold mb-6 tracking-tight">
                We prove every decision.
              </h2>
              <p className="text-lg leading-relaxed mb-6" style={{ color: 'var(--text-secondary)' }}>
                Every block, every challenge, every pass — logged with the exact signals that triggered it. Show your team, your board, your customers exactly what happened and why.
              </p>
              <p className="text-lg leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                No black boxes. No "trust us." Full transparency.
              </p>
            </div>
            <div className="md:col-span-7">
              <div className="font-mono text-xs p-5 rounded-lg space-y-2" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)' }}>
                <p style={{ color: 'var(--text-muted)' }}>// example evidence log</p>
                <p className="mt-3">Request ID: req_8f3a2b1c</p>
                <p>IP: 185.220.101.42 (DE)</p>
                <p>JA4: t13d1516h2_8daaf6152771_0271d189196b</p>
                <p>UA: Mozilla/5.0 (Windows NT 10.0; Win64; x64)</p>
                <p className="mt-3 pt-3 border-t" style={{ borderColor: 'var(--border-primary)' }}>
                  <span style={{ color: 'var(--text-muted)' }}>Signals fired:</span>
                </p>
                <p>  <span className="text-red-400">[UA Mismatch]</span> claims Chrome, TLS says otherwise</p>
                <p>  <span className="text-red-400">[TLS Fragmentation]</span> ClientHello split across records</p>
                <p>  <span className="text-yellow-400">[Header Order]</span> non-browser ordering detected</p>
                <p className="mt-3 pt-3 border-t" style={{ borderColor: 'var(--border-primary)' }}>
                  Score: <span className="text-red-400 font-bold">87/100</span>
                </p>
                <p>Decision: <span className="text-red-400 font-bold">BLOCK</span></p>
                <p>Origin reached: <span className="text-red-400">no</span></p>
                <p>Latency: 2.3ms</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Founding Customer Program */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-24">
          <div className="max-w-3xl">
            <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
              // founding customers
            </p>
            <h2 className="text-3xl md:text-4xl font-bold mb-6 tracking-tight">
              We're looking for 10 founding customers.
            </h2>
            <p className="text-lg leading-relaxed mb-6" style={{ color: 'var(--text-secondary)' }}>
              hakaishield is in active development. We're looking for 10 companies to build this with us. You'll get early access, direct line to engineering, and founding customer pricing.
            </p>
            <p className="text-lg leading-relaxed mb-8" style={{ color: 'var(--text-secondary)' }}>
              In return, we need your honest feedback. What works, what doesn't, what's missing. This is how we build something that actually solves the problem.
            </p>

            <div className="space-y-4 mb-8">
              <div className="flex gap-4">
                <span className="text-2xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>01</span>
                <div>
                  <p className="font-semibold mb-1">Early access</p>
                  <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Be among the first to use hakaishield. Shape the roadmap with your feedback.</p>
                </div>
              </div>
              <div className="flex gap-4">
                <span className="text-2xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>02</span>
                <div>
                  <p className="font-semibold mb-1">Founding pricing</p>
                  <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Founding customers start at $200/mo.</p>
                </div>
              </div>
              <div className="flex gap-4">
                <span className="text-2xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>03</span>
                <div>
                  <p className="font-semibold mb-1">Direct support</p>
                  <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Weekly calls with the engineering team. Your problems get solved fast.</p>
                </div>
              </div>
              <div className="flex gap-4">
                <span className="text-2xl font-bold font-mono" style={{ color: 'var(--text-muted)' }}>04</span>
                <div>
                  <p className="font-semibold mb-1">Zero risk</p>
                  <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Start with shadow mode. See what we find before blocking anything.</p>
                </div>
              </div>
            </div>

            <a href="/contact" className="group btn-primary px-6 py-3 text-sm inline-flex items-center gap-2">
              Apply for founding access
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </a>
            <p className="text-sm mt-4" style={{ color: 'var(--text-muted)' }}>
              No credit card required
            </p>
          </div>
        </div>
      </section>

      {/* ROI Calculator */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-24">
          <div className="grid md:grid-cols-12 gap-12">
            <div className="md:col-span-5">
              <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
                // the math
              </p>
              <h2 className="text-3xl md:text-4xl font-bold mb-6 tracking-tight">
                How much are bots costing you?
              </h2>
              <div className="space-y-6">
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span style={{ color: 'var(--text-muted)' }}>Monthly visitors</span>
                    <span className="font-mono font-medium">{visitors.toLocaleString()}</span>
                  </div>
                  <input type="range" min="100000" max="10000000" step="100000" value={visitors} onChange={(e) => setVisitors(Number(e.target.value))} className="w-full" />
                </div>
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span style={{ color: 'var(--text-muted)' }}>Bot traffic</span>
                    <span className="font-mono font-medium">{botPct}%</span>
                  </div>
                  <input type="range" min="10" max="60" step="5" value={botPct} onChange={(e) => setBotPct(Number(e.target.value))} className="w-full" />
                </div>
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span style={{ color: 'var(--text-muted)' }}>Avg order value</span>
                    <span className="font-mono font-medium">${aov}</span>
                  </div>
                  <input type="range" min="10" max="500" step="10" value={aov} onChange={(e) => setAov(Number(e.target.value))} className="w-full" />
                </div>

                <div className="pt-6 border-t grid grid-cols-2 gap-4" style={{ borderColor: 'var(--border-primary)' }}>
                  <div>
                    <p className="text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Revenue at risk</p>
                    <p className="text-2xl font-bold font-mono text-red-400">${leak.toLocaleString()}</p>
                  </div>
                  <div>
                    <p className="text-xs mb-1" style={{ color: 'var(--text-muted)' }}>ROI</p>
                    <p className="text-2xl font-bold font-mono text-green-400">{roi}%</p>
                  </div>
                </div>
              </div>
            </div>
            <div className="md:col-span-7 flex items-center">
              <div>
                <p className="text-lg leading-relaxed mb-6" style={{ color: 'var(--text-secondary)' }}>
                  Most teams don't realize how much revenue they're losing to bots until they measure it. Pricing scrapers, ticket hoarders, API abusers — they're all costing you real money every day.
                </p>
                <p className="text-lg leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                  Use this calculator to estimate your exposure. This is a rough illustration, not a measurement. Run hakaishield in shadow mode to see the real numbers in your own traffic.
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 py-32 text-center">
          <h2 className="text-4xl md:text-5xl font-bold mb-6 tracking-tight">
            Stop losing revenue<br />to bots.
          </h2>
          <p className="text-xl mb-10 max-w-xl mx-auto" style={{ color: 'var(--text-secondary)' }}>
            Start with shadow mode. See what we'd block. Then decide.
          </p>
          <a href="/contact" className="group btn-primary px-8 py-4 text-base inline-flex items-center gap-2">
            Request a pilot
            <ArrowRight size={16} className="transition-transform group-hover:translate-x-0.5" />
          </a>
          <p className="text-sm mt-6" style={{ color: 'var(--text-muted)' }}>
            No credit card · Shadow mode first · Nothing is blocked until you decide
          </p>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t py-8" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6">
          <div className="grid md:grid-cols-4 gap-8 mb-8">
            <div>
              <p className="text-sm font-bold tracking-tight mb-3">hakaishield</p>
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                Inline bot protection that reads the TLS handshake and scores the first request.
              </p>
            </div>
            <div>
              <p className="text-xs font-semibold mb-3" style={{ color: 'var(--text-secondary)' }}>Product</p>
              <ul className="space-y-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                <li><a href="/pricing" className="hover:text-white transition-colors">Pricing</a></li>
                <li><a href="/changelog" className="hover:text-white transition-colors">Changelog</a></li>
                <li><a href="/docs" className="hover:text-white transition-colors">Documentation</a></li>
                <li><a href="/contact" className="hover:text-white transition-colors">Contact</a></li>
              </ul>
            </div>
            <div>
              <p className="text-xs font-semibold mb-3" style={{ color: 'var(--text-secondary)' }}>Company</p>
              <ul className="space-y-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                <li><a href="/about" className="hover:text-white transition-colors">About</a></li>
              </ul>
            </div>
            <div>
              <p className="text-xs font-semibold mb-3" style={{ color: 'var(--text-secondary)' }}>Legal</p>
              <ul className="space-y-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                <li><a href="/terms" className="hover:text-white transition-colors">Terms of Service</a></li>
                <li><a href="/privacy" className="hover:text-white transition-colors">Privacy Policy</a></li>
              </ul>
            </div>
          </div>
          <div className="pt-6 border-t flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>© 2026 hakaishield. All rights reserved.</p>
            <div className="flex gap-4 text-xs" style={{ color: 'var(--text-muted)' }}>
              <a href="/sign-in" className="hover:text-white transition-colors">Sign in</a>
              <a href="/sign-up" className="hover:text-white transition-colors">Sign up</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
