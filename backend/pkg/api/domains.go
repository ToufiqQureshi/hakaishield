package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
)

var (
	listDomains = db.ListDomains
	// These are function variables so the handlers can be tested without a
	// live database, matching the existing listDomains seam.
	createDomain                 = db.CreateDomain
	getOwnedDomain               = db.GetOwnedDomain
	markDomainVerified           = db.MarkDomainVerified
	markDomainActive             = db.MarkDomainActive
	dashboardOwnershipConfigured = func() bool { return db.DB != nil }
	// lookupDomainTXT is the ownership check's only network call. It is
	// injected so a test never has to reach the public DNS.
	lookupDomainTXT = func(ctx context.Context, name string) ([]string, error) {
		return net.DefaultResolver.LookupTXT(ctx, name)
	}
	lookupDomainIPs = func(ctx context.Context, name string) ([]net.IP, error) {
		return net.DefaultResolver.LookupIP(ctx, "ip", name)
	}
	probeDomainOrigin = core.ProbePublicOrigin
)

// dnsLookupTimeout bounds the DNS query made when a customer clicks
// "Verify". It runs on the authenticated dashboard path, not in front of
// visitor traffic, so a slow resolver delays one onboarding request only.
const dnsLookupTimeout = 5 * time.Second

type createDomainRequest struct {
	Domain string `json:"domain"`
	Origin string `json:"origin"`
}

func domainJSON(d db.Domain) map[string]any {
	out := map[string]any{
		"id":     d.ID,
		"domain": d.Host,
		"origin": d.Target,
		"name":   d.Name,
		"status": d.Status,
	}
	// Only a domain still awaiting verification has a token worth showing.
	// An active or verified domain must never leak one.
	if d.Status == db.StatusPendingVerification && d.VerificationToken != "" {
		out["verification"] = map[string]any{
			"recordType":  "TXT",
			"recordName":  verificationRecordName(d.Host),
			"recordValue": verificationRecordValue(d.VerificationToken),
		}
	}
	// After ownership verification, give the customer the exact DNS cutover
	// record. An unconfigured edge must never be presented as activatable.
	if d.Status == db.StatusVerified {
		address := strings.TrimSpace(os.Getenv("HAKAISHIELD_EDGE_IPV4"))
		if core.PublicEdgeIPv4(address) {
			out["edge"] = map[string]string{
				"recordType": "A", "recordName": d.Host, "recordValue": address,
			}
		}
	}
	return out
}

// DomainsHandler lists an account's domains and lets a signed-in user
// reserve a new one. A reserved domain is pending verification: it does
// not route traffic, and proof of ownership is a DNS TXT record the user
// publishes themselves (see DomainVerifyHandler).
func DomainsHandler(verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			domains, err := listDomains(r.Context(), UserIDFromContext(r.Context()))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not list domains")
				return
			}
			out := make([]map[string]any, 0, len(domains))
			for _, d := range domains {
				out = append(out, domainJSON(d))
			}
			writeJSON(w, http.StatusOK, out)

		case http.MethodPost:
			var req createDomainRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "malformed request body")
				return
			}
			host, ok := validDomainHost(req.Domain)
			if !ok {
				writeError(w, http.StatusBadRequest, "enter a domain you control, such as shop.example.com")
				return
			}
			origin, ok := validOrigin(req.Origin)
			if !ok {
				writeError(w, http.StatusBadRequest, "enter a valid origin, such as https://origin.example")
				return
			}
			if originUsesProtectedHost(origin, host) {
				writeError(w, http.StatusBadRequest, "origin must use a separate hostname or public IP so DNS cutover cannot route traffic back to HakaiShield")
				return
			}

			domain, err := createDomain(r.Context(), UserIDFromContext(r.Context()), host, origin)
			switch {
			case errors.Is(err, db.ErrHostTaken):
				writeError(w, http.StatusConflict, "that domain is already registered")
				return
			case errors.Is(err, db.ErrTooManyPendingDomains):
				writeError(w, http.StatusTooManyRequests, "too many domains awaiting verification; verify one before adding another")
				return
			case err != nil:
				writeError(w, http.StatusInternalServerError, "could not add domain")
				return
			}
			writeJSON(w, http.StatusCreated, domainJSON(domain))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

// DomainVerifyHandler proves a domain belongs to the caller by looking up
// the TXT record they published, then marks it verified. Verification only
// unlocks operator setup; it does not by itself route traffic (a valid TLS
// certificate and DNS cutover still have to happen).
func DomainVerifyHandler(verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		owner, id := UserIDFromContext(r.Context()), r.PathValue("id")
		d, err := getOwnedDomain(r.Context(), owner, id)
		if errors.Is(err, db.ErrDomainNotFound) {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load domain")
			return
		}

		switch d.Status {
		case db.StatusVerified, "active":
			writeMessage(w, http.StatusOK, "ownership is already verified")
			return
		case db.StatusPendingVerification:
			// fall through to the lookup below
		default:
			writeError(w, http.StatusConflict, "domain is not awaiting verification")
			return
		}
		if d.VerificationToken == "" {
			writeError(w, http.StatusConflict, "domain has no pending verification")
			return
		}

		recordName := verificationRecordName(d.Host)
		expected := verificationRecordValue(d.VerificationToken)

		ctx, cancel := context.WithTimeout(r.Context(), dnsLookupTimeout)
		records, lookupErr := lookupDomainTXT(ctx, recordName)
		cancel()

		if lookupErr != nil || !containsRecord(records, expected) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf(
				"we could not find %s TXT %q yet; add the record, wait for DNS to update, then retry",
				recordName, expected))
			return
		}

		if err := markDomainVerified(r.Context(), owner, id); err != nil {
			if errors.Is(err, db.ErrDomainNotFound) {
				writeError(w, http.StatusNotFound, "domain not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not save verification")
			return
		}
		writeMessage(w, http.StatusOK, "ownership verified")
	})
}

// DomainActivateHandler provisions a verified tenant end to end: it checks
// the configured origin, obtains a host-restricted ACME certificate, then
// atomically enables visitor routing. Failed steps leave the tenant verified.
func DomainActivateHandler(verifier *auth.Verifier, issueCertificate func(context.Context, string) error) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		owner, id := UserIDFromContext(r.Context()), r.PathValue("id")
		d, err := getOwnedDomain(r.Context(), owner, id)
		if errors.Is(err, db.ErrDomainNotFound) {
			writeError(w, http.StatusNotFound, "domain not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load domain")
			return
		}
		if d.Status == db.StatusActive {
			writeMessage(w, http.StatusOK, "domain is already protected")
			return
		}
		if d.Status != db.StatusVerified {
			writeError(w, http.StatusConflict, "verify domain ownership before enabling protection")
			return
		}
		// Existing rows may predate the create-time check. Never activate a
		// route that will call itself after the customer's DNS cutover.
		if originUsesProtectedHost(d.Target, d.Host) {
			writeError(w, http.StatusConflict, "origin uses the protected hostname; configure a separate origin before enabling protection")
			return
		}
		if issueCertificate == nil {
			writeError(w, http.StatusServiceUnavailable, "automatic TLS is not configured")
			return
		}
		edgeIP := strings.TrimSpace(os.Getenv("HAKAISHIELD_EDGE_IPV4"))
		if !core.PublicEdgeIPv4(edgeIP) {
			writeError(w, http.StatusServiceUnavailable, "public edge address is not configured")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
		defer cancel()
		if err := checkDomainEdgeDNS(ctx, d.Host, edgeIP); err != nil {
			writeError(w, http.StatusConflict, "public DNS does not point only to the HakaiShield edge yet; update this hostname's A record, remove conflicting A/AAAA records, and retry")
			return
		}
		if err := probeDomainOrigin(ctx, d.Target, d.Host); err != nil {
			writeError(w, http.StatusBadGateway, "origin check failed; confirm the origin is publicly reachable and try again")
			return
		}
		if err := issueCertificate(ctx, d.Host); err != nil {
			writeError(w, http.StatusBadGateway, "TLS certificate could not be issued; confirm this hostname points to HakaiShield and try again")
			return
		}
		if err := markDomainActive(ctx, owner, id); err != nil {
			if errors.Is(err, db.ErrDomainNotFound) {
				writeError(w, http.StatusConflict, "domain setup changed; refresh and try again")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not enable protection")
			return
		}
		writeMessage(w, http.StatusOK, "domain protection enabled")
	})
}

func verificationRecordName(host string) string {
	return "_hakaishield." + host
}

func verificationRecordValue(token string) string {
	return "hakaishield-verify=" + token
}

func containsRecord(records []string, want string) bool {
	for _, r := range records {
		if strings.TrimSpace(r) == want {
			return true
		}
	}
	return false
}

// validDomainHost accepts a bare public hostname: lowercase letters,
// digits, hyphens and dots, at least one dot, no scheme, port, wildcard
// or IP literal. It returns the normalized host.
func validDomainHost(raw string) (string, bool) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 || host == "*" || !strings.Contains(host, ".") {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil {
		return "", false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return "", false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return "", false
		}
	}
	return host, true
}

// validOrigin accepts an http(s) origin URL with a host and no embedded
// credentials, query or fragment. Public-origin enforcement (so an
// account cannot point protection at an internal service) happens again
// when the origin proxy dials, in core.NewPublicOriginProxy.
func validOrigin(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	if u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return "", false
	}
	return raw, true
}

func originUsesProtectedHost(origin, protectedHost string) bool {
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), strings.TrimSuffix(protectedHost, "."))
}

// Every public address must reach this edge. A leftover AAAA or A record
// could split visitors between protected and unprotected servers after setup.
func checkDomainEdgeDNS(ctx context.Context, host, edgeIPv4 string) error {
	addresses, err := lookupDomainIPs(ctx, host)
	if err != nil || len(addresses) == 0 {
		return errors.New("domain has no usable public address")
	}
	want := net.ParseIP(edgeIPv4)
	for _, ip := range addresses {
		if !ip.Equal(want) {
			return fmt.Errorf("domain resolves to %s instead of %s", ip, edgeIPv4)
		}
	}
	return nil
}
