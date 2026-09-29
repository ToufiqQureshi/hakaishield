import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { transformSync } from 'esbuild';

const source = readFileSync(new URL('../src/lib/onboarding.ts', import.meta.url), 'utf8');
const mod = { exports: {} };
new Function('module', 'exports', transformSync(source, { loader: 'ts', format: 'cjs' }).code)(mod, mod.exports);
const { stepComplete } = mod.exports;

const empty = { useCase: '', monthlyVisitors: '', website: '', botProblem: '', teamSize: '' };
const full = { useCase: 'ecommerce', monthlyVisitors: '100k-500k', website: 'https://shop.example', botProblem: 'pricing-scraping', teamSize: '2-10' };

test('no step can be skipped with empty answers', () => {
  for (const step of [1, 2, 3]) assert.equal(stepComplete(step, empty), false, `step ${step}`);
});

test('each step completes once its own answers are given', () => {
  assert.equal(stepComplete(1, { ...empty, useCase: 'api' }), true);
  assert.equal(stepComplete(2, { ...empty, monthlyVisitors: '<100k', teamSize: 'solo', website: 'https://a.example' }), true);
  assert.equal(stepComplete(3, { ...empty, botProblem: 'not-sure' }), true);
  assert.equal(stepComplete(4, full), false);
});

test('step 2 rejects a website that is not a real http(s) URL', () => {
  for (const website of ['', 'shop', 'javascript:alert(1)', 'ftp://shop.example', 'https://localhost', '   ']) {
    assert.equal(stepComplete(2, { ...full, website }), false, JSON.stringify(website));
  }
  assert.equal(stepComplete(2, { ...full, teamSize: '' }), false);
  assert.equal(stepComplete(2, { ...full, monthlyVisitors: '' }), false);
});
