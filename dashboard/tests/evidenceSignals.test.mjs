import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import { transformSync } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const source = readFileSync(new URL('../src/components/EvidenceSignals.tsx', import.meta.url), 'utf8');
const code = transformSync(source, { loader: 'tsx', format: 'cjs', jsx: 'automatic' }).code;
const compiled = { exports: {} };
new Function('require', 'module', 'exports', code)(createRequire(import.meta.url), compiled, compiled.exports);
const EvidenceSignals = compiled.exports.default;

test('scored and shadow signals are visibly distinct', () => {
  const html = renderToStaticMarkup(createElement(EvidenceSignals, {
    signals: ['velocity_spike'],
    shadowSignals: ['challenge_canvas_duplicate'],
  }));
  assert.match(html, /velocity_spike/);
  assert.match(html, /challenge_canvas_duplicate/);
  assert.match(html, /Observed/);
  assert.match(html, /badge-red/);
  assert.match(html, /badge-yellow/);
});

test('missing shadow signals remain an empty observation', () => {
  const html = renderToStaticMarkup(createElement(EvidenceSignals, {
    signals: [],
  }));
  assert.doesNotMatch(html, /Observed/);
});

test('signals beyond the first three are counted and named, not dropped', () => {
  const html = renderToStaticMarkup(createElement(EvidenceSignals, {
    signals: ['a1', 'a2', 'a3', 'a4'],
    shadowSignals: ['datacenter_ip', 'no_subresources', 'beacon_webdriver', 'beacon_no_interaction', 'h2_ua_family_mismatch'],
  }));
  assert.match(html, /\+1 more</);
  assert.match(html, /title="a4"/);
  assert.match(html, /\+2 more observed/);
  assert.match(html, /beacon_no_interaction, h2_ua_family_mismatch/);
});

test('exactly three signals show no overflow badge', () => {
  const html = renderToStaticMarkup(createElement(EvidenceSignals, {
    signals: ['a1', 'a2', 'a3'],
    shadowSignals: ['b1', 'b2', 'b3'],
  }));
  assert.doesNotMatch(html, /more/);
});
