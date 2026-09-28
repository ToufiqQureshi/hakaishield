import { ArrowRight, Fingerprint, SearchCheck, Shield } from 'lucide-react';

export default function Landing() {
  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <nav className="border-b" style={{ borderColor: 'var(--border-primary)' }}>
        <div className="max-w-5xl mx-auto px-6 h-16 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm">hakaishield</a>
          <div className="flex items-center gap-5 text-sm">
            <a href="#how" style={{ color: 'var(--text-secondary)' }}>How it works</a>
            {import.meta.env.MODE !== 'marketing' && (
              <a href="/sign-in" style={{ color: 'var(--text-secondary)' }}>Sign in</a>
            )}
            <a href="/contact" className="btn-primary px-4 py-2 text-xs">Request a pilot</a>
          </div>
        </div>
      </nav>

      <main>
        <section className="max-w-5xl mx-auto px-6 py-24 md:py-32">
          <p className="text-sm font-mono mb-5" style={{ color: 'var(--text-muted)' }}>// inline traffic intelligence</p>
          <h1 className="text-5xl md:text-7xl font-bold tracking-tight leading-tight max-w-4xl">
            See automated traffic.<br />Decide with evidence.
          </h1>
          <p className="text-lg mt-7 max-w-2xl" style={{ color: 'var(--text-secondary)' }}>
            HakaiShield sits in front of your site, reads the TLS handshake, scores requests,
            and records why it would allow, challenge or block them.
          </p>
          <a href="/contact" className="btn-primary inline-flex items-center gap-2 px-6 py-3 text-sm mt-9">
            Discuss a pilot <ArrowRight size={16} />
          </a>
        </section>

        <section id="how" className="border-y" style={{ borderColor: 'var(--border-primary)', background: 'var(--bg-secondary)' }}>
          <div className="max-w-5xl mx-auto px-6 py-16">
            <h2 className="text-2xl font-semibold mb-8">What the pilot includes</h2>
            <div className="grid md:grid-cols-3 gap-4">
              {[
                { icon: Fingerprint, title: 'TLS and request signals', copy: 'JA4, headers, client claims and server-observed request patterns contribute to scoring.' },
                { icon: SearchCheck, title: 'Reviewable evidence', copy: 'See the signals and decisions recorded for traffic routed through the pilot proxy.' },
                { icon: Shield, title: 'Shadow-first rollout', copy: 'We test the real site and review false positives before enabling blocking.' },
              ].map(({ icon: Icon, title, copy }) => (
                <div key={title} className="card p-6">
                  <Icon size={22} className="text-blue-400 mb-5" />
                  <h3 className="font-semibold mb-2">{title}</h3>
                  <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>{copy}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="max-w-5xl mx-auto px-6 py-20">
          <h2 className="text-2xl font-semibold mb-3">A managed setup for the first client</h2>
          <p className="text-sm max-w-2xl" style={{ color: 'var(--text-secondary)' }}>
            We verify domain ownership, connect your origin, configure TLS, and run a shadow traffic review together.
            We confirm the site works before enforcement. Self-service domain activation is not available during the pilot.
          </p>
          <a href="/contact" className="inline-flex items-center gap-2 underline text-sm mt-6">Contact the operator <ArrowRight size={14} /></a>
        </section>
      </main>
    </div>
  );
}
