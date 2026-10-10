import type { LegalSection } from '../lib/legal';

type Props = { title: string; updated: string; sections: LegalSection[] };

export default function LegalDocument({ title, updated, sections }: Props) {
  return (
    <main className="min-h-screen px-5 py-10 sm:px-8 sm:py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <article className="mx-auto max-w-3xl space-y-8">
        <header className="space-y-4 border-b pb-6" style={{ borderColor: 'var(--border-primary)' }}>
          <a href="/landing" className="font-bold text-sm">hakaishield</a>
          <h1 className="text-3xl font-semibold sm:text-4xl">{title}</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Last updated: {updated}</p>
          <p className="rounded border p-4 text-sm" style={{ borderColor: 'var(--border-primary)', color: 'var(--text-secondary)' }}>
            HakaiShield is preparing for public paid service. Operator legal identity, registered address, governing law, privacy contact, and production provider details must be completed and reviewed for the launch country before paid protection is offered.
          </p>
        </header>
        <div className="space-y-8">
          {sections.map((section) => (
            <section key={section.heading} className="space-y-3">
              <h2 className="text-xl font-semibold">{section.heading}</h2>
              {section.paragraphs?.map((paragraph, index) => <p key={index} className="text-sm leading-7" style={{ color: 'var(--text-secondary)' }}>{paragraph}</p>)}
              {section.bullets && <ul className="list-disc space-y-2 pl-5 text-sm leading-7" style={{ color: 'var(--text-secondary)' }}>{section.bullets.map((bullet) => <li key={bullet}>{bullet}</li>)}</ul>}
            </section>
          ))}
        </div>
        <footer className="flex flex-wrap gap-x-5 gap-y-2 border-t pt-5 text-sm" style={{ borderColor: 'var(--border-primary)' }}>
          <a href="/terms" className="underline">Terms</a><a href="/privacy" className="underline">Privacy</a>
          <a href="/acceptable-use" className="underline">Acceptable use</a><a href="/cookies" className="underline">Cookies</a>
          <a href="/data-processing" className="underline">Data processing</a><a href="/contact" className="underline">Contact</a>
        </footer>
      </article>
    </main>
  );
}
