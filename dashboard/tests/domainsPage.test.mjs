import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';

const nodeRequire = createRequire(import.meta.url);
const contextHolder = { current: null };

function load(path, stubs = {}, define = {}) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8');
  const code = transformSync(source, { loader: path.endsWith('.tsx') ? 'tsx' : 'ts', format: 'cjs', jsx: 'automatic', define }).code;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', code)((name) => stubs[name] ?? nodeRequire(name), mod, mod.exports);
  return mod.exports;
}

const api = load('../src/lib/api.ts', { './supabaseClient': { supabase: { auth: { getSession: async () => ({ data: { session: null } }) } } } }, {
  'import.meta.env.VITE_API_BASE_URL': '"https://api.example/api/v1"',
  'import.meta.env.DEV': 'false',
});

const domainOnboarding = load('../src/lib/domainOnboarding.ts');
const domainStatus = load('../src/lib/domainStatus.ts');

const { default: DomainsSiem } = load('../src/pages/DomainsSiem.tsx', {
  'react-router-dom': { useOutletContext: () => contextHolder.current },
  'lucide-react': { AlertCircle: () => null, CheckCircle2: () => null, Copy: () => null, Globe: () => null },
  '../lib/api': api,
  '../lib/domainOnboarding': domainOnboarding,
  '../lib/domainStatus': domainStatus,
});

function render(domains) {
  contextHolder.current = { domains, selectedDomain: null, domainsLoading: false, refreshDomains: async () => {} };
  const React = nodeRequire('react');
  return nodeRequire('react-dom/server').renderToStaticMarkup(React.createElement(DomainsSiem));
}

test('a pending domain shows its TXT record and a verify action, never Protected', () => {
  const html = render([{
    id: 'd1', domain: 'shop.example.com', origin: 'https://origin.example', name: 'shop.example.com', status: 'pending_verification',
    verification: { recordType: 'TXT', recordName: '_hakaishield.shop.example.com', recordValue: 'hakaishield-verify=tok' },
  }]);
  assert.match(html, /_hakaishield\.shop\.example\.com/);
  assert.match(html, /hakaishield-verify=tok/);
  assert.match(html, /Verify ownership/);
  assert.doesNotMatch(html, />Protected</);
});

test('an active domain is the only one shown as Protected', () => {
  const html = render([{ id: 'd2', domain: 'live.example.com', origin: 'https://origin.example', name: 'live.example.com', status: 'active' }]);
  assert.match(html, /Protected/);
  assert.doesNotMatch(html, /hakaishield-verify/);
});

test('the empty state points at the add form, not a dead-end pilot request', () => {
  const html = render([]);
  assert.match(html, /Add one above to get started\./);
  assert.doesNotMatch(html, /Request pilot setup/);
  assert.doesNotMatch(html, /Domain setup is managed during the pilot/);
});
