import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { transformSync } from 'esbuild';

const source = readFileSync(new URL('../src/lib/authRedirect.ts', import.meta.url), 'utf8');
const mod = { exports: {} };
new Function('module', 'exports', transformSync(source, { loader: 'ts', format: 'cjs' }).code)(mod, mod.exports);
const { signedOutPath } = mod.exports;

test('signed-out visitor at the site root sees the landing page', () => {
  assert.equal(signedOutPath('/'), '/landing');
});

test('signed-out visitor on a dashboard page is sent to sign-in', () => {
  for (const path of ['/evidence-logs', '/mitigation-rules', '/protection-settings', '/domains-siem']) {
    assert.equal(signedOutPath(path), '/sign-in', path);
  }
});
