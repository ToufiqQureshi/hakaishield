export default function AccountUnavailable() {
  return (
    <main className="min-h-screen px-6 py-20" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-xl mx-auto space-y-5">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">Account setup is unavailable</h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          This site is the public information build. Account creation requires the dashboard, authentication service, and customer API to be deployed together. Paid protection is not available here yet.
        </p>
        <div className="flex flex-wrap gap-3">
          <a href="/landing" className="btn-secondary px-5 py-2.5 text-sm">Back to home</a>
          <a href="/contact" className="btn-primary px-5 py-2.5 text-sm">Contact us</a>
        </div>
      </div>
    </main>
  );
}
