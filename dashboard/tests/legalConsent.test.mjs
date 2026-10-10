import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';

const require = createRequire(import.meta.url);
const source = new URL('../src/lib/legalConsent.ts', import.meta.url);
const legalSource = new URL('../src/lib/legal.ts', import.meta.url);

function load(path, stubs = {}) {
  const code = transformSync((awaitRead(path)), { loader: 'ts', format: 'cjs' }).code;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', code)((name) => stubs[name] ?? require(name), mod, mod.exports);
  return mod.exports;
}

function awaitRead(url) {
  return require('node:fs').readFileSync(url, 'utf8');
}

test('signup consent records exact document versions and one acceptance timestamp', () => {
  const legal = load(legalSource);
  const { buildSignupConsent } = load(source, { './legal': legal });
  const timestamp = '2026-10-05T12:34:56.000Z';
  const consent = buildSignupConsent(false, timestamp);
  assert.equal(consent.terms_version, legal.LEGAL_VERSIONS.terms);
  assert.equal(consent.privacy_version, legal.LEGAL_VERSIONS.privacy);
  assert.equal(consent.accepted_at, timestamp);
  assert.equal(consent.product_updates, false);
  assert.equal(consent.product_updates_updated_at, timestamp);
});

test('marketing opt-in is independent from mandatory terms and privacy acceptance', () => {
  const legal = load(legalSource);
  const { buildSignupConsent } = load(source, { './legal': legal });
  const consent = buildSignupConsent(true, '2026-10-05T12:34:56.000Z');
  assert.equal(consent.product_updates, true);
  assert.equal(consent.terms_version, legal.LEGAL_VERSIONS.terms);
  assert.equal(consent.privacy_version, legal.LEGAL_VERSIONS.privacy);
});
