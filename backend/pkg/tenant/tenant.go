package tenant

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/stats"
	"golang.org/x/sync/singleflight"
)

// ErrTenantNotFound is returned when a requested tenant ID or host does not exist.
var ErrTenantNotFound = errors.New("tenant not found")

// TenantConfig holds the configuration specific to a single customer.
type TenantConfig struct {
	Target        string            // The origin server to protect (e.g., https://example.com)
	Mode          config.Mode       // Enforce or Shadow
	Policy        config.PolicyMode // Balanced or Strict
	EvidenceToken string            // Bearer token for the per-request evidence endpoint
	Deception     bool              // If true, high-confidence bot traffic is deceived instead of 403 blocked (docs/STATUS.md item 11a)
	Status        string            // Domain lifecycle status; only active domains route visitor traffic.
	// OwnerUserID identifies the dashboard account this tenant belongs
	// to. mitigation_rules and protection_settings are keyed by this,
	// not by tenant/domain ID (an account's rules apply across every
	// domain it owns) — a core.Guard.PolicyProvider uses this to find
	// which account's rules to evaluate for a request, never a
	// visitor-supplied value (CLAUDE.md Section 16/17).
	OwnerUserID string
}

// Tenant represents a single customer's isolated environment.
// It holds its own proxy, stats, and evidence trail so data cannot leak across customers.
type Tenant struct {
	ID           string
	Config       TenantConfig
	Stats        *stats.Stats
	Trail        *evidence.Trail
	PolicyShadow *evidence.ShadowStats
	Origin       *httputil.ReverseProxy
}

// ProxyFactory is a callback to create origin proxies without creating import cycles.
type ProxyFactory func(target string) (*httputil.ReverseProxy, error)

// TenantLoader fetches a tenant row for lazy host-based loading. The default
// implementation reads Postgres; the hook also keeps the store testable
// without requiring a live database.
type TenantLoader func(ctx context.Context, host string) (id, target, mode, evidenceToken, status, ownerUserID string, err error)

const StatusActive = "active"

const (
	negativeHostTTL        = 30 * time.Second
	maxNegativeHostEntries = 4096

	// maxConcurrentHostLookups caps simultaneous Postgres lookups for
	// hosts not yet in memory. The Host header is visitor-controlled, so
	// without a cap a flood of random hostnames becomes one database
	// query per request. Overflow is answered "not found" without being
	// negative-cached, so a real new domain resolves on the next request.
	maxConcurrentHostLookups = 8
)

// Store is a thread-safe implementation that maps hostnames and IDs to tenant environments.
type Store struct {
	mu           sync.RWMutex
	byHost       map[string]*Tenant
	byID         map[string]*Tenant
	negativeHost map[string]time.Time
	lookupSlots  chan struct{}
	lookups      singleflight.Group
	ProxyFactory ProxyFactory
	TenantLoader TenantLoader

	// Durable evidence is opt-in. When EvidenceWriter is set, every new
	// tenant's trail writes through to it, and EvidenceSink is read once at
	// load time to seed the in-memory ring so a restart does not erase the
	// history a customer can see. EvidenceWriter/EvidenceSink are set before
	// any tenant is added and never change afterwards.
	EvidenceWriter *evidence.Writer
	EvidenceSink   evidence.Sink
	EvidenceWindow time.Duration
}

// NewStore creates a store for testing or single-node deployments.
func NewStore() *Store {
	return &Store{
		byHost:       make(map[string]*Tenant),
		byID:         make(map[string]*Tenant),
		negativeHost: make(map[string]time.Time),
		lookupSlots:  make(chan struct{}, maxConcurrentHostLookups),
	}
}

// Add provisions a new tenant environment and maps it to the given hosts.
func (s *Store) Add(id string, config TenantConfig, hosts []string, origin *httputil.ReverseProxy) error {
	if config.Status == "" {
		config.Status = StatusActive
	}
	canonicalHosts := make([]string, 0, len(hosts))
	for _, host := range hosts {
		host = canonicalHost(host)
		if host == "" {
			return errors.New("tenant: invalid host")
		}
		canonicalHosts = append(canonicalHosts, host)
	}
	t := &Tenant{
		ID:           id,
		Config:       config,
		Stats:        &stats.Stats{Mode: config.Mode},
		Trail:        evidence.NewTrail(),
		PolicyShadow: evidence.NewShadowStats(),
		Origin:       origin,
	}
	if s.EvidenceWriter != nil {
		t.Trail.EnablePersistence(id, s.EvidenceWriter)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, host := range canonicalHosts {
		if existing, ok := s.byHost[host]; ok && existing.ID != id {
			return errors.New("tenant: host already mapped to another tenant")
		}
	}
	s.byID[id] = t
	for _, host := range canonicalHosts {
		if config.routesTraffic() {
			s.byHost[host] = t
		}
		delete(s.negativeHost, host)
	}
	return nil
}

// GetByHost looks up a tenant by their incoming HTTP host header.
func (s *Store) GetByHost(host string) (*Tenant, error) {
	host = canonicalHost(host)
	if host == "" {
		return nil, ErrTenantNotFound
	}
	s.mu.RLock()
	t, ok := s.byHost[host]
	s.mu.RUnlock()

	if ok {
		return t, nil
	}

	// Load an exact database-backed tenant before consulting the single-tenant
	// wildcard. Checking the wildcard first makes lazy loading unreachable.
	if s.ProxyFactory != nil && !s.negativeHostFresh(host, time.Now()) {
		if t, err := s.lookupHost(host); err == nil {
			if !t.Config.routesTraffic() {
				return nil, ErrTenantNotFound
			}
			return t, nil
		}
	}

	s.mu.RLock()
	t, ok = s.byHost["*"]
	s.mu.RUnlock()
	if ok {
		return t, nil
	}
	return nil, ErrTenantNotFound
}

// GetCachedByHost returns an exact, already loaded tenant without database
// access. Challenge shadow evidence uses it on the verify path so an attacker
// cannot turn optional telemetry into a database lookup for arbitrary hosts.
func (s *Store) GetCachedByHost(host string) *Tenant {
	host = canonicalHost(host)
	if host == "" {
		return nil
	}
	s.mu.RLock()
	t := s.byHost[host]
	s.mu.RUnlock()
	return t
}

// errLookupBusy means every lookup slot was taken, so the host was not
// checked at all and must not be remembered as missing.
var errLookupBusy = errors.New("tenant: host lookup capacity exhausted")

// lookupHost loads an unknown host from the database. Concurrent requests
// for the same host share one query, and at most maxConcurrentHostLookups
// queries run at once across all hosts.
func (s *Store) lookupHost(host string) (*Tenant, error) {
	v, err, _ := s.lookups.Do(host, func() (any, error) {
		select {
		case s.lookupSlots <- struct{}{}:
			defer func() { <-s.lookupSlots }()
		default:
			return nil, errLookupBusy
		}
		t, err := s.fetchFromDB(host)
		if err != nil {
			s.rememberNegativeHost(host, time.Now())
			return nil, err
		}
		return t, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Tenant), nil
}

func (s *Store) negativeHostFresh(host string, now time.Time) bool {
	s.mu.RLock()
	expires, ok := s.negativeHost[host]
	s.mu.RUnlock()
	return ok && now.Before(expires)
}

func (s *Store) rememberNegativeHost(host string, now time.Time) {
	if host == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.negativeHost) >= maxNegativeHostEntries {
		for h, expires := range s.negativeHost {
			if !now.Before(expires) {
				delete(s.negativeHost, h)
			}
		}
	}
	if len(s.negativeHost) >= maxNegativeHostEntries {
		var oldestHost string
		var oldestExpiry time.Time
		for h, expires := range s.negativeHost {
			if oldestHost == "" || expires.Before(oldestExpiry) {
				oldestHost, oldestExpiry = h, expires
			}
		}
		if oldestHost != "" {
			delete(s.negativeHost, oldestHost)
		}
	}
	s.negativeHost[host] = now.Add(negativeHostTTL)
}

func (s *Store) fetchFromDB(host string) (*Tenant, error) {
	if s.ProxyFactory == nil {
		return nil, ErrTenantNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	loader := s.TenantLoader
	if loader == nil {
		loader = db.GetTenant
	}
	id, target, modeStr, evidenceToken, status, ownerUserID, err := loader(ctx, host)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	return s.addFromDBRow(ctx, id, host, target, modeStr, evidenceToken, status, ownerUserID)
}

// addFromDBRow turns one tenants-table row into a live Tenant and
// registers it under both lookup maps, so a dashboard request that
// found the row by ID and a proxy request that finds it later by host
// share the same in-memory Stats/Trail rather than each starting a
// fresh one.
func (s *Store) addFromDBRow(ctx context.Context, id, host, target, modeStr, evidenceToken, status, ownerUserID string) (*Tenant, error) {
	// A stored mode we can't parse must never silently decide behaviour.
	// Treat an unknown value as enforce (fail closed) and say so, rather
	// than letting Go's zero value quietly pick a mode for a live tenant.
	mode, err := config.ParseMode(modeStr)
	if err != nil {
		log.Printf("hakaishield: tenant %q has unknown mode %q, defaulting to enforce: %v", id, modeStr, err)
		mode = config.ModeEnforce
	}
	proxy, err := s.ProxyFactory(target)
	if err != nil {
		return nil, err
	}

	tenantConfig := TenantConfig{
		Target:        target,
		Mode:          mode,
		EvidenceToken: evidenceToken,
		Status:        status,
		OwnerUserID:   ownerUserID,
	}

	if err := s.Add(id, tenantConfig, []string{host}, proxy); err != nil {
		return nil, err
	}

	s.mu.RLock()
	t, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrTenantNotFound
	}
	s.hydrateEvidence(ctx, t)
	return t, nil
}

// GetByID looks up a tenant by their internal ID (for dashboard API
// use, where the caller knows a domain's ID from the tenants table but
// has no incoming request/Host header to look it up by).
func (s *Store) GetByID(id string) (*Tenant, error) {
	s.mu.RLock()
	t, ok := s.byID[id]
	s.mu.RUnlock()
	if ok {
		return t, nil
	}

	if s.ProxyFactory == nil {
		return nil, ErrTenantNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	host, target, modeStr, evidenceToken, status, ownerUserID, err := db.GetTenantByID(ctx, id)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	return s.addFromDBRow(ctx, id, host, target, modeStr, evidenceToken, status, ownerUserID)
}

// maxHydratedEvidence matches the in-memory ring size: loading more would
// be overwritten before it could be read.
const maxHydratedEvidence = 1000

// HydrateEvidence seeds every already-known tenant's trail from durable
// storage. Call it once at startup, after the default tenant is added.
func (s *Store) HydrateEvidence(ctx context.Context) {
	s.mu.RLock()
	ts := make([]*Tenant, 0, len(s.byID))
	for _, t := range s.byID {
		ts = append(ts, t)
	}
	s.mu.RUnlock()
	for _, t := range ts {
		s.hydrateEvidence(ctx, t)
	}
}

// hydrateEvidence reads a tenant's recent durable history into memory. A
// read failure is logged, not fatal: an empty trail is the same state a
// non-durable deployment starts in.
func (s *Store) hydrateEvidence(ctx context.Context, t *Tenant) {
	if s.EvidenceSink == nil || t == nil {
		return
	}
	window := s.EvidenceWindow
	if window <= 0 {
		window = 24 * time.Hour
	}
	records, err := s.EvidenceSink.Recent(ctx, t.ID, maxHydratedEvidence, time.Now().Add(-window))
	if err != nil {
		log.Printf("hakaishield: loading durable evidence for tenant %q: %v", t.ID, err)
		return
	}
	t.Trail.Load(records)
}

// OwnerUserID resolves a tenant ID to the dashboard account that owns
// it, or reports ok=false when the tenant can't be found or has no
// owner on record (e.g. a tenant seeded without one). It exists so
// pkg/policyprovider can bind a request's already-validated tenant
// identity to that account's rules without importing tenant.Store's
// full surface, and without ever accepting an owner ID from the caller.
func (s *Store) OwnerUserID(tenantID string) (string, bool) {
	t, err := s.GetByID(tenantID)
	if err != nil || t.Config.OwnerUserID == "" {
		return "", false
	}
	return t.Config.OwnerUserID, true
}

func (c TenantConfig) routesTraffic() bool {
	return c.Status == "" || c.Status == StatusActive
}

func canonicalHost(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "*" {
		return raw
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	} else {
		raw = strings.Trim(raw, "[]")
	}
	raw = strings.TrimSuffix(raw, ".")
	if raw == "" || strings.ContainsAny(raw, " \t\r\n/\\") {
		return ""
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String()
	}
	for _, label := range strings.Split(raw, ".") {
		if label == "" || len(label) > 63 {
			return ""
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return ""
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return ""
		}
	}
	return raw
}
