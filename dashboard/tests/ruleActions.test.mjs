import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';

const requireModule = createRequire(import.meta.url);
function apiWith(fetcher) {
  globalThis.fetch = fetcher;
  const source = readFileSync(new URL('../src/lib/api.ts', import.meta.url), 'utf8');
  const code = transformSync(source, {
    loader: 'ts', format: 'cjs', define: {
      'import.meta.env.VITE_API_BASE_URL': '"https://api.example/api/v1"',
      'import.meta.env.DEV': 'false',
    },
  }).code;
  const mod = { exports: {} };
  new Function('require', 'module', 'exports', code)((name) => name === './supabaseClient'
    ? { supabase: { auth: { getSession: async () => ({ data: { session: { access_token: 'owner-token' } } }) } } }
    : requireModule(name), mod, mod.exports);
  return mod.exports;
}

test('rule builder saves an authenticated shadow draft and toggles only its own rule ID', async () => {
  const calls = [];
  const api = apiWith(async (url, options) => {
    calls.push({ url, options });
    return { ok: true, status: options.method === 'POST' ? 201 : 200, json: async () => ({ success: true, data: { id: 'rule-1' }, message: 'rule updated' }) };
  });
  await api.createRule('Block bad JA4', [{ field: 'JA4 Fingerprint', operator: 'EQUALS', value: 't13d...' }], 'BLOCK');
  await api.setRuleEnabled('rule/with space', false);
  assert.equal(calls[0].url, 'https://api.example/api/v1/rules/custom');
  assert.equal(calls[0].options.headers.Authorization, 'Bearer owner-token');
  assert.deepEqual(JSON.parse(calls[0].options.body), {
    name: 'Block bad JA4', conditions: [{ field: 'JA4 Fingerprint', operator: 'EQUALS', value: 't13d...' }], action: 'BLOCK',
  });
  assert.equal(calls[1].url, 'https://api.example/api/v1/rules/rule%2Fwith%20space/toggle');
  assert.deepEqual(JSON.parse(calls[1].options.body), { enabled: false });
});
