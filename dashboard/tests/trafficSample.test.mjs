import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { transformSync } from 'esbuild';

const source = readFileSync(new URL('../src/lib/trafficSample.ts', import.meta.url), 'utf8');
const mod = { exports: {} };
new Function('module', 'exports', transformSync(source, { loader: 'ts', format: 'cjs' }).code)(mod, mod.exports);
const { trafficSample } = mod.exports;

test('traffic chart counts only actual recent edge decisions and keeps empty hours', () => {
  const now = Date.parse('2026-10-05T12:30:00Z');
  const entries = [
    { time: '2026-10-05T12:01:00Z', decision: 'allow' },
    { time: '2026-10-05T12:02:00Z', decision: 'block' },
    { time: '2026-10-05T11:59:00Z', decision: 'rate_limit' },
    { time: '2026-10-04T11:59:00Z', decision: 'challenge' },
    { time: '2026-10-05T12:31:00Z', decision: 'deceive' },
    { time: 'invalid', decision: 'allow' },
  ];
  const buckets = trafficSample(entries, now);
  assert.equal(buckets.length, 24);
  assert.equal(buckets[0].hour, '2026-10-04T13:00:00.000Z');
  assert.deepEqual(
    [buckets[22].rateLimited, buckets[23].passed, buckets[23].blocked, buckets[23].challenged, buckets[23].deceived],
    [1, 1, 1, 0, 0],
  );
  assert.equal(buckets.slice(0, 22).reduce((sum, bucket) => sum + bucket.passed + bucket.blocked + bucket.challenged + bucket.deceived + bucket.rateLimited, 0), 0);
});
