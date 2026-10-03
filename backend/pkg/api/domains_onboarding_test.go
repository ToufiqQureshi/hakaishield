package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/db"
)

// withDomainOps swaps the database seams for stubs and restores them
// afterwards. The package-level variables mirror the existing listDomains
// seam; tests here are not parallel for the same reason.
func withDomainOps(t *testing.T, create func(context.Context, string, string, string) (db.Domain, error), get func(context.Context, string, string) (db.Domain, error), mark func(context.Context, string, string) error, lookup func(context.Context, string) ([]string, error)) {
	t.Helper()
	origCreate, origGet, origMark, origLookup := createDomain, getOwnedDomain, markDomainVerified, lookupDomainTXT
	if create != nil {
		createDomain = create
	}
	if get != nil {
		getOwnedDomain = get
	}
	if mark != nil {
		markDomainVerified = mark
	}
	if lookup != nil {
		lookupDomainTXT = lookup
	}
	t.Cleanup(func() {
		createDomain, getOwnedDomain, markDomainVerified, lookupDomainTXT = origCreate, origGet, origMark, origLookup
	})
}

func postJSON(t *testing.T, handler http.Handler, target, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestCreateDomainReservesPendingHostWithVerificationRecord(t *testing.T) {
	ia := newIsolationAuth(t)
	var gotOwner, gotHost, gotTarget string
	withDomainOps(t,
		func(_ context.Context, owner, host, target string) (db.Domain, error) {
			gotOwner, gotHost, gotTarget = owner, host, target
			return db.Domain{ID: "d1", Host: host, Target: target, Name: host, Status: db.StatusPendingVerification, VerificationToken: "tok123"}, nil
		}, nil, nil, nil)

	rec := postJSON(t, DomainsHandler(ia.verifier(t)), "/api/v1/domains", `{"domain":"Shop.Example.COM","origin":"https://origin.example"}`, ia.sign(t, "user-a"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if gotOwner != "user-a" {
		t.Fatalf("owner = %q, want the authenticated subject", gotOwner)
	}
	if gotHost != "shop.example.com" {
		t.Fatalf("host = %q, want lowercased shop.example.com", gotHost)
	}
	if gotTarget != "https://origin.example" {
		t.Fatalf("target = %q, want the given origin", gotTarget)
	}

	var resp struct {
		Data struct {
			Status       string `json:"status"`
			Verification struct {
				RecordType  string `json:"recordType"`
				RecordName  string `json:"recordName"`
				RecordValue string `json:"recordValue"`
			} `json:"verification"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Status != db.StatusPendingVerification {
		t.Fatalf("status = %q, want pending", resp.Data.Status)
	}
	if resp.Data.Verification.RecordName != "_hakaishield.shop.example.com" ||
		resp.Data.Verification.RecordValue != "hakaishield-verify=tok123" ||
		resp.Data.Verification.RecordType != "TXT" {
		t.Fatalf("verification record = %+v", resp.Data.Verification)
	}
}

func TestCreateDomainReportsConflictsAndCaps(t *testing.T) {
	ia := newIsolationAuth(t)
	verifier := ia.verifier(t)
	token := ia.sign(t, "user-a")
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"host taken", db.ErrHostTaken, http.StatusConflict},
		{"pending cap", db.ErrTooManyPendingDomains, http.StatusTooManyRequests},
	} {
		withDomainOps(t, func(context.Context, string, string, string) (db.Domain, error) { return db.Domain{}, tc.err }, nil, nil, nil)
		rec := postJSON(t, DomainsHandler(verifier), "/api/v1/domains", `{"domain":"shop.example.com","origin":"https://origin.example"}`, token)
		if rec.Code != tc.want {
			t.Fatalf("%s: status = %d, want %d: %s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestVerifyDomainProvesOwnershipWithTXT(t *testing.T) {
	ia := newIsolationAuth(t)
	var markedOwner, markedID string
	markCalls := 0
	withDomainOps(t,
		nil,
		func(_ context.Context, owner, id string) (db.Domain, error) {
			if owner != "user-a" {
				t.Fatalf("lookup owner = %q, want the caller", owner)
			}
			return db.Domain{ID: id, Host: "shop.example.com", Status: db.StatusPendingVerification, VerificationToken: "tok123"}, nil
		},
		func(_ context.Context, owner, id string) error {
			markCalls++
			markedOwner, markedID = owner, id
			return nil
		},
		func(_ context.Context, name string) ([]string, error) {
			if name != "_hakaishield.shop.example.com" {
				t.Fatalf("TXT lookup name = %q", name)
			}
			// TXT records are returned as separate strings; the value in
			// the right one proves ownership even when others are present.
			return []string{"unrelated", "hakaishield-verify=tok123"}, nil
		})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/d1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+ia.sign(t, "user-a"))
	req.SetPathValue("id", "d1")
	rec := httptest.NewRecorder()
	DomainVerifyHandler(ia.verifier(t)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if markCalls != 1 || markedOwner != "user-a" || markedID != "d1" {
		t.Fatalf("mark calls=%d owner=%q id=%q", markCalls, markedOwner, markedID)
	}
}

func TestVerifyDomainRejectsWrongOrMissingRecord(t *testing.T) {
	ia := newIsolationAuth(t)
	verifier := ia.verifier(t)
	token := ia.sign(t, "user-a")

	for _, tc := range []struct {
		name    string
		lookup  func(context.Context, string) ([]string, error)
		wantSub string
	}{
		{"no record", func(context.Context, string) ([]string, error) { return nil, nil }, "could not find"},
		{"wrong value", func(context.Context, string) ([]string, error) { return []string{"hakaishield-verify=other"}, nil }, "could not find"},
		{"lookup error", func(context.Context, string) ([]string, error) { return nil, errors.New("NXDOMAIN") }, "could not find"},
	} {
		markCalled := false
		withDomainOps(t, nil,
			func(_ context.Context, _, id string) (db.Domain, error) {
				return db.Domain{ID: id, Host: "shop.example.com", Status: db.StatusPendingVerification, VerificationToken: "tok123"}, nil
			},
			func(context.Context, string, string) error { markCalled = true; return nil },
			tc.lookup)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/d1/verify", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "d1")
		rec := httptest.NewRecorder()
		DomainVerifyHandler(verifier).ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.wantSub) {
			t.Fatalf("%s: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if markCalled {
			t.Fatalf("%s: domain was marked verified without a matching record", tc.name)
		}
	}
}

func TestVerifyDomainCannotTouchAnotherAccountsDomain(t *testing.T) {
	ia := newIsolationAuth(t)
	verifier := ia.verifier(t)
	markCalled := false
	withDomainOps(t, nil,
		func(_ context.Context, owner, id string) (db.Domain, error) {
			// The database query is owner-scoped, so another account's id
			// is indistinguishable from a nonexistent one.
			if owner != "user-a" {
				return db.Domain{}, db.ErrDomainNotFound
			}
			return db.Domain{}, db.ErrDomainNotFound // d1 belongs to user-b
		},
		func(context.Context, string, string) error { markCalled = true; return nil },
		func(context.Context, string) ([]string, error) { return []string{"hakaishield-verify=tok123"}, nil })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/d1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+ia.sign(t, "user-a"))
	req.SetPathValue("id", "d1")
	rec := httptest.NewRecorder()
	DomainVerifyHandler(verifier).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-account verify status = %d, want 404", rec.Code)
	}
	if markCalled {
		t.Fatal("another account's domain was marked verified")
	}
}

func TestVerifyDomainIsIdempotentOnceVerified(t *testing.T) {
	ia := newIsolationAuth(t)
	lookupCalled := false
	withDomainOps(t, nil,
		func(_ context.Context, _, id string) (db.Domain, error) {
			return db.Domain{ID: id, Host: "shop.example.com", Status: db.StatusVerified}, nil
		},
		nil,
		func(context.Context, string) ([]string, error) { lookupCalled = true; return nil, nil })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/d1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+ia.sign(t, "user-a"))
	req.SetPathValue("id", "d1")
	rec := httptest.NewRecorder()
	DomainVerifyHandler(ia.verifier(t)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("repeat verify status = %d, want 200", rec.Code)
	}
	if lookupCalled {
		t.Fatal("a verified domain must not trigger another DNS lookup")
	}
}

func TestVerifyDomainRequiresAuthAndPost(t *testing.T) {
	ia := newIsolationAuth(t)
	verifier := ia.verifier(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains/d1/verify", nil)
	req.SetPathValue("id", "d1")
	rec := httptest.NewRecorder()
	DomainVerifyHandler(verifier).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated verify status = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/domains/d1/verify", nil)
	req.Header.Set("Authorization", "Bearer "+ia.sign(t, "user-a"))
	req.SetPathValue("id", "d1")
	rec = httptest.NewRecorder()
	DomainVerifyHandler(verifier).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET verify status = %d, want 405", rec.Code)
	}
}

func TestDomainJSONHidesTokenUnlessPending(t *testing.T) {
	pending := domainJSON(db.Domain{Host: "a.example", Status: db.StatusPendingVerification, VerificationToken: "t"})
	if _, ok := pending["verification"]; !ok {
		t.Fatal("pending domain must expose its verification record")
	}
	for _, status := range []string{db.StatusVerified, "active", "failed"} {
		d := domainJSON(db.Domain{Host: "a.example", Status: status, VerificationToken: "t"})
		if _, ok := d["verification"]; ok {
			t.Fatalf("status %q leaked a verification token", status)
		}
	}
}

func TestValidDomainHostAndOrigin(t *testing.T) {
	for _, host := range []string{"shop.example.com", "a.b.co", "x1.y2.example"} {
		if _, ok := validDomainHost(host); !ok {
			t.Fatalf("validDomainHost(%q) = false, want true", host)
		}
	}
	for _, host := range []string{"", "shop", "*.example.com", "https://shop.example.com", "shop.example.com:443", "127.0.0.1", "bad host.com", "-a.example.com", "a..example.com", strings.Repeat("a", 64) + ".example.com"} {
		if _, ok := validDomainHost(host); ok {
			t.Fatalf("validDomainHost(%q) = true, want false", host)
		}
	}

	for _, origin := range []string{"https://origin.example", "http://127.0.0.1:9000", "https://origin.example/base"} {
		if _, ok := validOrigin(origin); !ok {
			t.Fatalf("validOrigin(%q) = false, want true", origin)
		}
	}
	for _, origin := range []string{"", "origin.example", "ftp://origin.example", "https://user:pass@origin.example", "https://origin.example#frag", "https://origin.example?x=1"} {
		if _, ok := validOrigin(origin); ok {
			t.Fatalf("validOrigin(%q) = true, want false", origin)
		}
	}
}
