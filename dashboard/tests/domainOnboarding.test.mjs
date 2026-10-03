import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
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

const { domainInputError, normalizeDomain } = load('../src/lib/domainOnboarding.ts');

test('normalizeDomain lowercases and strips a trailing dot', () => {
  assert.equal(normalizeDomain('  Shop.Example.COM. '), 'shop.example.com');
});

test('a valid domain and origin are accepted', () => {
  assert.equal(domainInputError('shop.example.com', 'https://origin.example'), null);
  assert.equal(domainInputError('Shop.Example.com.', 'http://127.0.0.1:9000'), null);
});

test('hosts we cannot route are rejected before any request', () => {
  for (const host of ['', 'shop', '*.example.com', 'https://shop.example.com', 'shop.example.com:443', '127.0.0.1', 'bad host.com', '-a.example.com', 'a..example.com']) {
    assert.notEqual(domainInputError(host, 'https://origin.example'), null, host);
  }
});

test('non-http origins and credentialed origins are rejected', () => {
  for (const origin of ['', 'origin.example', 'ftp://origin.example', 'https://user:pass@origin.example', 'https://origin.example?x=1', 'https://origin.example#f']) {
    assert.notEqual(domainInputError('shop.example.com', origin), null, origin);
  }
});

function apiWith(fetcher) {
  globalThis.fetch = fetcher;
  return load('../src/lib/api.ts', { './supabaseClient': { supabase: { auth: { getSession: async () => ({ data: { session: { access_token: 'owned-token' } } }) } } } }, {
    'import.meta.env.VITE_API_BASE_URL': '"https://api.example/api/v1"',
    'import.meta.env.DEV': 'false',
  });
}

test('createDomain posts the domain and origin with the session token', async () => {
  const api = apiWith(async (url, options) => {
    assert.equal(url, 'https://api.example/api/v1/domains');
    assert.equal(options.method, 'POST');
    assert.equal(options.headers.Authorization, 'Bearer owned-token');
    assert.deepEqual(JSON.parse(options.body), { domain: 'shop.example.com', origin: 'https://origin.example' });
    return { ok: true, status: 201, json: async () => ({ success: true, data: { id: 'd1', domain: 'shop.example.com', origin: 'https://origin.example', name: 'shop.example.com', status: 'pending_verification', verification: { recordType: 'TXT', recordName: '_hakaishield.shop.example.com', recordValue: 'hakaishield-verify=tok' } } }) };
  });
  const created = await api.createDomain('shop.example.com', 'https://origin.example');
  assert.equal(created.status, 'pending_verification');
  assert.equal(created.verification.recordName, '_hakaishield.shop.example.com');
});

test('verifyDomain scopes the request to the encoded domain id', async () => {
  let seen = '';
  const api = apiWith(async (url, options) => {
    seen = url;
    assert.equal(options.method, 'POST');
    return { ok: true, status: 200, json: async () => ({ success: true, message: 'ownership verified' }) };
  });
  await api.verifyDomain('id/with space');
  assert.equal(seen, 'https://api.example/api/v1/domains/id%2Fwith%20space/verify');
});

test('a failed create surfaces the backend message, not a fake success', async () => {
  const api = apiWith(async () => ({ ok: false, status: 409, json: async () => ({ success: false, error: 'that domain is already registered' }) }));
  await assert.rejects(api.createDomain('shop.example.com', 'https://origin.example'), (error) => error.status === 409 && /already registered/.test(error.message));
});
