import { useTheme } from '../context/ThemeContext';
import { AlertCircle, Check } from 'lucide-react';

export default function Subscription() {
  const { theme, toggleTheme } = useTheme();

  const plans = [
    {
      name: 'Starter',
      price: '$200',
      period: '/mo',
      description: 'For growing sites that need bot protection now.',
      features: [
        '1M requests/month',
        '3 protected domains',
        'JA4 TLS fingerprinting',
        'Multi-layer scoring',
        'Shadow mode',
        'Evidence trail (24h)',
        'JS challenge',
        'Email support',
      ],
    },
    {
      name: 'Growth',
      price: '$500',
      period: '/mo',
      description: 'For businesses where bots are a revenue leak.',
      features: [
        '10M requests/month',
        '10 protected domains',
        'Everything in Starter',
        'Behavioral scoring',
        'Deception engine',
        'Evidence trail (30 days)',
        'Custom rules',
        'SIEM integrations (roadmap)',
        'Rate limiting per endpoint',
        'Priority support',
      ],
    },
    {
      name: 'Enterprise',
      price: 'Custom',
      period: '',
      description: 'Self-hosted. For regulated industries.',
      features: [
        'Unlimited requests',
        'Unlimited domains',
        'Everything in Growth',
        'Self-hosted deployment',
        'Data residency',
        'SSO / SAML (roadmap)',
        'Dedicated account manager',
        'SLA guarantee',
        'Custom integrations',
        'Fingerprint database access',
      ],
    },
  ];

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/sign-in" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Dashboard</a>
          </div>
        </div>
      </nav>

      <div className="max-w-5xl mx-auto px-6 py-16">
        <div className="mb-12">
          <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
            // subscription
          </p>
          <h1 className="text-4xl font-bold mb-4 tracking-tight">Plans</h1>
        </div>

        {/* Billing not wired up yet — no plan/usage state exists, so nothing here pretends there is one */}
        <div className="card p-4 mb-8 flex items-center gap-3">
          <AlertCircle size={18} className="text-yellow-400 shrink-0" />
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Billing isn't wired up yet — there's no real subscription, usage metering, or invoice history to show.
            The plans below are for reference only; <a href="/contact" className="underline">contact us</a> to
            subscribe today.
          </p>
        </div>

        {/* Plans */}
        <h2 className="text-2xl font-bold mb-6 tracking-tight">Plans</h2>
        <div className="grid md:grid-cols-3 gap-px mb-8" style={{ background: 'var(--border-primary)' }}>
          {plans.map((plan, i) => (
            <div key={i} className="p-6 relative" style={{ background: 'var(--bg-primary)' }}>
              <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-muted)' }}>{plan.name.toUpperCase()}</p>
              <div className="mb-6">
                <span className="text-4xl font-bold">{plan.price}</span>
                <span className="text-sm" style={{ color: 'var(--text-muted)' }}>{plan.period}</span>
              </div>
              <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
                {plan.description}
              </p>
              <a href="/contact" className="btn-secondary w-full py-2.5 text-sm text-center block mb-6">
                {plan.name === 'Enterprise' ? 'Contact sales' : `Ask about ${plan.name}`}
              </a>
              <ul className="space-y-3 text-sm" style={{ color: 'var(--text-secondary)' }}>
                {plan.features.map((feature, j) => (
                  <li key={j} className="flex items-start gap-2">
                    <Check size={14} className="text-green-400 flex-shrink-0 mt-0.5" />
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
