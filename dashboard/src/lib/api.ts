// Thin fetch client for the hakaishield backend's domains/rules/settings
// API. Every endpoint returns {"success": boolean, "data"|"message"|"error"}
// (see backend/pkg/api/response.go) — this file is the one place that
// unwraps that envelope, so a page component only ever deals with plain
// data or a thrown ApiError.
//
// Auth itself (signup, signin, sign-out, password reset, email
// verification) is not here — it goes straight to Supabase via
// supabaseClient.ts. This file only attaches whatever session token
// Supabase already issued when calling the Go backend.

import { supabase } from './supabaseClient';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || (
  import.meta.env.DEV ? 'http://localhost:8080/api/v1' : ''
);
if (!BASE_URL) {
  throw new Error('VITE_API_BASE_URL must be set for production dashboard builds.');
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function isSignedIn(): Promise<boolean> {
  const { data } = await supabase.auth.getSession();
  return !!data.session;
}

interface Envelope<T> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { data: sessionData } = await supabase.auth.getSession();
  const token = sessionData.session?.access_token;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> | undefined),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  let res: Response;
  try {
    res = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  } catch {
    throw new ApiError(0, 'Could not reach the server. Check your connection and try again.');
  }

  let body: Envelope<T> | null = null;
  try {
    body = await res.json();
  } catch {
    // A non-JSON response (proxy error page, empty 204, etc.) is
    // still handled by the status-code branch below rather than
    // treated as a parse failure the caller has to know about.
  }

  if (!res.ok || body?.success === false) {
    throw new ApiError(res.status, body?.error || body?.message || `Request failed (${res.status})`);
  }

  return (body?.data ?? body?.message ?? null) as T;
}

// ---- Domains ----

// A pending domain carries the DNS record the owner must publish to
// prove control of it. It is absent once the domain is verified.
export interface DomainVerification {
  recordType: string;
  recordName: string;
  recordValue: string;
}

export interface Domain {
  id: string;
  domain: string;
  origin: string;
  name: string;
  status: string;
  verification?: DomainVerification;
}

export async function listDomains() {
  return request<Domain[]>('/domains');
}

export async function createDomain(domain: string, origin: string) {
  return request<Domain>('/domains', {
    method: 'POST',
    body: JSON.stringify({ domain, origin }),
  });
}

// verifyDomain publishes nothing itself; it asks the backend to check the
// TXT record the owner added and to record the verdict.
export async function verifyDomain(id: string) {
  return request<string | null>(`/domains/${encodeURIComponent(id)}/verify`, { method: 'POST' });
}

export interface PolicyDocument {
  mode: 'shadow' | 'enforce';
  rules: unknown[];
  routeClasses?: Record<string, 'login' | 'checkout'>;
  allowlist?: string[];
  challengeTheme?: string;
  blockMessage?: string;
}

export interface PolicyRevision {
  version: number;
  document: PolicyDocument;
}

// A missing revision is normal for a new pilot domain. PUT checks ownership
// against the authenticated account before it can create version one.
export async function getTenantPolicy(tenantId: string, signal?: AbortSignal): Promise<PolicyRevision | null> {
  try {
    return await request<PolicyRevision>(`/domains/${encodeURIComponent(tenantId)}/policy`, { signal });
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
}

export async function saveTenantPolicy(tenantId: string, expectedVersion: number, document: PolicyDocument): Promise<PolicyRevision> {
  return request<PolicyRevision>(`/domains/${encodeURIComponent(tenantId)}/policy`, {
    method: 'PUT',
    body: JSON.stringify({ expectedVersion, document }),
  });
}

// ---- Mitigation Rules ----

export interface ManagedRule {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
}

export interface RuleCondition {
  field: string;
  operator: string;
  value: string;
}

export interface CustomRule {
  id: string;
  name: string;
  conditions: RuleCondition[];
  action: string;
  enabled: boolean;
  createdAt: string;
}

export interface RulesResponse {
  managedRules: ManagedRule[];
  customRules: CustomRule[];
  exceptions: unknown[];
}

export async function listRules() {
  return request<RulesResponse>('/rules');
}

// ---- Dashboard ----

export interface DashboardStats {
  total_requests: number;
  passed: number;
  challenged: number;
  blocked: number;
  deceived: number;
  rateLimited: number;
  // P1 measurement: egress bytes are the inline proxy's dominant hosting
  // cost, and challenge solves/failures are the human-burden numbers.
  egress_bytes: number;
  challenge_solves: number;
  challenge_failures: number;
  mode: string;
  enforcing: boolean;
}

// getStats talks to the authenticated stats endpoint. Its response is raw
// JSON (rather than the CRUD API envelope), so it keeps a small dedicated
// fetch while still attaching the current Supabase session token.
export async function getStats(tenantId: string, signal?: AbortSignal): Promise<DashboardStats> {
  const { data: sessionData } = await supabase.auth.getSession();
  const token = sessionData.session?.access_token;
  const res = await fetch(`${BASE_URL}/dashboard/stats?tenant=${encodeURIComponent(tenantId)}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    signal,
  });
  if (!res.ok) {
    throw new ApiError(res.status, `Could not load stats (${res.status})`);
  }
  return res.json();
}

export interface TopOffender {
  ja4: string;
  hits: number;
  blocked: number;
}

export async function getTopOffenders(tenantId: string, signal?: AbortSignal) {
  return request<TopOffender[]>(`/dashboard/top-offenders?tenant=${encodeURIComponent(tenantId)}`, { signal });
}

export interface EvidenceEntry {
  time: string;
  ja4: string;
  signals: string[];
  shadowSignals?: string[];
  score: number;
  decision: string;
  enforced: boolean;
}

export async function getEvidenceLogs(tenantId: string, signal?: AbortSignal) {
  return request<EvidenceEntry[]>(`/dashboard/evidence-logs?tenant=${encodeURIComponent(tenantId)}`, { signal });
}
