import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight } from 'lucide-react';
import { contactEmail, pilotRequestMailto } from '../lib/contact';

export default function Contact() {
  const { theme, toggleTheme } = useTheme();
  const [submitted, setSubmitted] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    email: '',
    company: '',
    website: '',
    monthlyVisitors: '',
    botProblem: '',
    message: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!contactEmail) return;
    window.location.href = pilotRequestMailto(contactEmail, formData);
    setSubmitted(true);
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  if (submitted) {
    return (
      <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
        <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
          <div className="max-w-3xl mx-auto px-6 h-14 flex items-center justify-between">
            <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
            <div className="flex items-center gap-4">
              <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
                {theme === 'dark' ? 'Light' : 'Dark'}
              </button>
              <a href="/landing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Home</a>
            </div>
          </div>
        </nav>

        <div className="max-w-3xl mx-auto px-6 py-24 text-center">
          <div className="w-16 h-16 rounded-full mx-auto mb-6 flex items-center justify-center" style={{ background: 'rgba(34, 197, 94, 0.1)', border: '1px solid rgba(34, 197, 94, 0.2)' }}>
            <span className="text-3xl">✓</span>
          </div>
          <h1 className="text-3xl font-bold mb-4 tracking-tight">Send the email to finish.</h1>
          <p className="text-lg mb-8" style={{ color: 'var(--text-secondary)' }}>
            Your mail app should have opened with your request filled in. Press send there and we will reply to you.
            If nothing opened, email <a href={`mailto:${contactEmail}`} className="underline">{contactEmail}</a> directly.
          </p>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            In the meantime, check out the <a href="/docs" className="underline hover:text-white transition-colors">technical docs</a> or <a href="/changelog" className="underline hover:text-white transition-colors">changelog</a>.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-3xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/landing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Home</a>
          </div>
        </div>
      </nav>

      <div className="max-w-3xl mx-auto px-6 py-16">
        <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
          // founding customer application
        </p>
        <h1 className="text-4xl font-bold mb-4 tracking-tight">Apply for founding access.</h1>
        <p className="text-lg mb-12" style={{ color: 'var(--text-secondary)' }}>
          We're looking for 10 companies to build hakaishield with. You'll get early access, founding pricing, and direct line to engineering.
        </p>

        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="grid md:grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-medium mb-2">Name</label>
              <input
                type="text"
                name="name"
                value={formData.name}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">Email</label>
              <input
                type="email"
                name="email"
                value={formData.email}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
              />
            </div>
          </div>

          <div className="grid md:grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-medium mb-2">Company</label>
              <input
                type="text"
                name="company"
                value={formData.company}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">Website</label>
              <input
                type="url"
                name="website"
                value={formData.website}
                onChange={handleChange}
                placeholder="https://"
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Monthly visitors</label>
            <select
              name="monthlyVisitors"
              value={formData.monthlyVisitors}
              onChange={handleChange}
              required
              className="w-full px-4 py-2.5 rounded text-sm"
              style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
            >
              <option value="">Select range</option>
              <option value="<100k">&lt; 100K</option>
              <option value="100k-500k">100K - 500K</option>
              <option value="500k-2m">500K - 2M</option>
              <option value="2m-10m">2M - 10M</option>
              <option value=">10m">&gt; 10M</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">What's your bot problem?</label>
            <select
              name="botProblem"
              value={formData.botProblem}
              onChange={handleChange}
              required
              className="w-full px-4 py-2.5 rounded text-sm"
              style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
            >
              <option value="">Select primary concern</option>
              <option value="pricing-scraping">Pricing scraping</option>
              <option value="inventory-hoarding">Inventory hoarding / ticket scalping</option>
              <option value="api-abuse">API abuse / bandwidth costs</option>
              <option value="account-takeover">Account takeover / credential stuffing</option>
              <option value="content-scraping">Content / listing scraping</option>
              <option value="other">Other</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Anything else? (optional)</label>
            <textarea
              name="message"
              value={formData.message}
              onChange={handleChange}
              rows={4}
              placeholder="Tell us about your specific situation, concerns, or questions..."
              className="w-full px-4 py-2.5 rounded text-sm resize-none"
              style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
            />
          </div>

          <div className="pt-4">
            <button type="submit" disabled={!contactEmail} className="group btn-primary px-6 py-3 text-sm flex items-center gap-2 disabled:opacity-60">
              Send request by email
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </button>
            <p className="text-xs mt-3" style={{ color: 'var(--text-muted)' }}>
              {contactEmail
                ? 'This opens your mail app with the request filled in. Nothing is sent until you press send.'
                : 'Online requests are not enabled yet. Contact your pilot operator using your agreed channel.'}
            </p>
          </div>
        </form>

        {contactEmail && (
          <div className="mt-16 pt-8 border-t" style={{ borderColor: 'var(--border-primary)' }}>
            <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
              Prefer to chat first? Email <a href={`mailto:${contactEmail}`} className="underline hover:text-white transition-colors">{contactEmail}</a>
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
