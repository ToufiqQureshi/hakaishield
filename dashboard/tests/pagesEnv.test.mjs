import test from 'node:test';
import assert from 'node:assert/strict';
import { validatePagesEnv } from '../scripts/pages-env.mjs';

const valid = {
  VITE_SUPABASE_URL: 'https://pilot-ref.supabase.co',
  VITE_SUPABASE_ANON_KEY: 'sb_publishable_validpilotkey123',
  VITE_API_BASE_URL: 'https://api.example.com/api/v1',
};

test('Cloudflare Pages build accepts the public production configuration', () => {
  assert.deepEqual(validatePagesEnv(valid), []);
});

test('Cloudflare Pages build rejects missing and placeholder configuration', () => {
  const errors = validatePagesEnv({
    VITE_SUPABASE_URL: 'https://your-project-ref.supabase.co',
    VITE_SUPABASE_ANON_KEY: 'your-anon-key',
    VITE_API_BASE_URL: '',
  });
  assert.equal(errors.length, 3);
});

test('Cloudflare Pages build refuses a Supabase server secret in public JavaScript', () => {
  const errors = validatePagesEnv({ ...valid, VITE_SUPABASE_ANON_KEY: 'sb_secret_never_publish_this_key' });
  assert.ok(errors.some((error) => error.includes('public Supabase')));
  const serviceRole = `header.${Buffer.from(JSON.stringify({ role: 'service_role' })).toString('base64url')}.signature`;
  assert.ok(validatePagesEnv({ ...valid, VITE_SUPABASE_ANON_KEY: serviceRole }).length > 0);
});

test('Cloudflare Pages build rejects insecure or malformed API origins', () => {
  for (const apiURL of [
    'http://localhost:8080/api/v1',
    'https://api.example.com/',
    'https://api.example.com/api/v1/',
    'https://user:pass@api.example.com/api/v1',
    'https://api.example.com/api/v1?token=x',
  ]) {
    assert.ok(validatePagesEnv({ ...valid, VITE_API_BASE_URL: apiURL }).length > 0, apiURL);
  }
});
