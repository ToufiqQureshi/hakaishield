import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';

// formatEgress renders the tenant's cost number the way the monthly
// bill reads. This is the number a customer will question, so its
// formatting is pinned here rather than left to whatever the browser
// shows for a raw byte count.
//
// The function lives in Overview.tsx and is compiled out the same way
// the other page tests compile their components, because the module
// cannot be imported directly under plain node (import.meta.env in the
// api client it imports, and tsx in the page itself).
const source = readFileSync(new URL('../src/pages/Overview.tsx', import.meta.url), 'utf8');

function loadModule() {
  const code = transformSync(source, { loader: 'tsx', format: 'cjs', jsx: 'automatic' }).code;
  const mod = { exports: {} };
  const require_ = createRequire(import.meta.url);
  const req = (name) => {
    if (name.includes('react-router-dom')) {
      return { useOutletContext: () => ({ selectedDomain: null, domainsLoading: true }), Link: () => null, Outlet: () => null };
    }
    // The api client cannot load under plain node (import.meta.env), and
    // the component body never runs in these tests — only formatEgress
    // is called — so a stub for the fetch layer is enough.
    if (name.includes('lib/api')) {
      return { getStats: async () => ({}), getTopOffenders: async () => [], ApiError: class ApiError extends Error {} };
    }
    if (name.includes('lib/trafficSample')) {
      return { trafficSample: () => [] };
    }
    return require_(name);
  };
  new Function('require', 'module', 'exports', code)(req, mod, mod.exports);
  return mod.exports;
}

const { formatEgress } = loadModule();

const MB = 1024 * 1024;
const GB = 1024 * MB;

test('egress renders in MB below a gigabyte', () => {
  assert.equal(formatEgress(0), '0.0 MB');
  assert.equal(formatEgress(5 * MB), '5.0 MB');
  assert.equal(formatEgress(900 * MB), '900.0 MB');
});

test('egress renders in GB once past a gigabyte', () => {
  assert.equal(formatEgress(GB), '1.00 GB');
  assert.equal(formatEgress(2.5 * GB), '2.50 GB');
});

test('a gigabyte boundary switches units exactly once', () => {
  assert.equal(formatEgress(GB - 1), '1024.0 MB');
  assert.equal(formatEgress(GB), '1.00 GB');
});
