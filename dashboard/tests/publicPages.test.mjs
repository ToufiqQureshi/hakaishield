import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';

const nodeRequire = createRequire(import.meta.url);

function load(path, stubs = {}, define = {}) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8');
  const code = transformSync(source, { loader: path.endsWith('.tsx') ? 'tsx' : 'ts', format: 'cjs', jsx: 'automatic', define }).code;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', code)((name) => stubs[name] ?? nodeRequire(name), mod, mod.exports);
  return mod.exports;
}

const envEmail = (value) => ({ 'import.meta.env.VITE_PILOT_CONTACT_EMAIL': JSON.stringify(value) });
const form = {
  name: 'Asha', email: 'asha@shop.example', company: 'Shop & Co', website: 'https://shop.example',
  monthlyVisitors: '500k-2m', botProblem: 'pricing-scraping', message: 'Scrapers hit /prices\nevery minute',
};

test('pilot request mailto carries every form field to the operator inbox', () => {
  const { pilotRequestMailto } = load('../src/lib/contact.ts', {}, envEmail('ops@operator.example'));
  const url = new URL(pilotRequestMailto('ops@operator.example', form));
  assert.equal(url.protocol, 'mailto:');
  assert.equal(decodeURIComponent(url.pathname), 'ops@operator.example');
  assert.equal(url.searchParams.get('subject'), 'HakaiShield pilot request: Shop & Co');
  const body = url.searchParams.get('body');
  for (const value of ['Asha', 'asha@shop.example', 'https://shop.example', '500k-2m', 'pricing-scraping', 'Scrapers hit /prices\nevery minute']) {
    assert.ok(body.includes(value), value);
  }
});

test('visitor input cannot add mail headers or recipients', () => {
  const { pilotRequestMailto } = load('../src/lib/contact.ts', {}, envEmail('ops@operator.example'));
  const hostile = { ...form, company: 'x&cc=attacker@evil.example&bcc=a@evil.example', message: '?to=evil@evil.example' };
  const url = new URL(pilotRequestMailto('ops@operator.example', hostile));
  assert.deepEqual([...url.searchParams.keys()], ['subject', 'body']);
  assert.equal(decodeURIComponent(url.pathname), 'ops@operator.example');
});

function renderContact(email) {
  const contact = load('../src/lib/contact.ts', {}, envEmail(email));
  const { default: Contact } = load('../src/pages/Contact.tsx', {
    '../context/ThemeContext': { useTheme: () => ({ theme: 'dark', toggleTheme: () => {} }) },
    '../lib/contact': contact,
  });
  const React = nodeRequire('react');
  return nodeRequire('react-dom/server').renderToStaticMarkup(React.createElement(Contact));
}

test('contact form sends by email when an operator inbox is configured', () => {
  const html = renderContact('ops@operator.example');
  assert.match(html, /Send request by email/);
  assert.doesNotMatch(html, /<button[^>]*\sdisabled=""/);
  assert.match(html, /mailto:ops@operator\.example/);
});

test('contact form is disabled and says so when no inbox is configured', () => {
  const html = renderContact('');
  assert.match(html, /<button[^>]*\sdisabled=""/);
  assert.match(html, /Online requests are not enabled yet/);
  assert.doesNotMatch(html, /mailto:/);
});

// These claims were on the pages before and are not true of the product:
// there is no trial or refund, no CNAME target, no hakaishield.io inbox, and
// the repository is private.
test('public pages make no claims the product cannot keep', () => {
  const dir = new URL('../src/pages/', import.meta.url);
  const banned = [/free trial/i, /14-day/i, /money-back/i, /spots remaining/i, /proxy\.hakaishield\.io/, /@hakaishield\.io/, /github\.com\/ToufiqQureshi/, /console\.log\(/, /forever/i];
  for (const name of readdirSync(dir)) {
    const source = readFileSync(new URL(name, dir), 'utf8');
    for (const pattern of banned) {
      assert.doesNotMatch(source, pattern, `${name} contains ${pattern}`);
    }
  }
});
