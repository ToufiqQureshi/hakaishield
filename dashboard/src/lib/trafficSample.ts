import type { EvidenceEntry } from './api';

export type TrafficBucket = {
  hour: string;
  passed: number;
  blocked: number;
  challenged: number;
  deceived: number;
  rateLimited: number;
};

// Evidence logs contain only the newest records held by the edge. This
// visualizes that bounded set; it must never be used for billing or totals.
export function trafficSample(entries: EvidenceEntry[], now = Date.now()): TrafficBucket[] {
  const hourMs = 60 * 60 * 1000;
  const firstHour = Math.floor(now / hourMs) * hourMs - 23 * hourMs;
  const buckets = Array.from({ length: 24 }, (_, index) => ({
    hour: new Date(firstHour + index * hourMs).toISOString(),
    passed: 0,
    blocked: 0,
    challenged: 0,
    deceived: 0,
    rateLimited: 0,
  }));
  for (const entry of entries) {
    const at = Date.parse(entry.time);
    if (!Number.isFinite(at) || at < firstHour || at > now) continue;
    const bucket = buckets[Math.floor((at - firstHour) / hourMs)];
    switch (entry.decision) {
      case 'allow': bucket.passed++; break;
      case 'block': bucket.blocked++; break;
      case 'challenge': bucket.challenged++; break;
      case 'deceive': bucket.deceived++; break;
      case 'rate_limit': bucket.rateLimited++; break;
    }
  }
  return buckets;
}
