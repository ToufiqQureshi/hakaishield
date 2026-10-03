package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/api"
)

func TestDomainCreationStillRequiresAuth(t *testing.T) {
	auth := newTestAuth(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{"domain":"victim.example","origin":"https://origin.example"}`))
	rec := httptest.NewRecorder()
	api.DomainsHandler(auth.verifier(t)).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated domain creation: status=%d, want 401", rec.Code)
	}
}

// Invalid input must be rejected before anything is persisted, so a typo
// cannot reserve a hostname the account does not control.
func TestDomainCreationRejectsInvalidInputBeforeSaving(t *testing.T) {
	auth := newTestAuth(t)
	handler := api.DomainsHandler(auth.verifier(t))
	token := auth.sign(t, "pilot-user")

	for _, body := range []string{
		`{"domain":"","origin":"https://origin.example"}`,
		`{"domain":"not a host","origin":"https://origin.example"}`,
		`{"domain":"no-dot","origin":"https://origin.example"}`,
		`{"domain":"*.example.com","origin":"https://origin.example"}`,
		`{"domain":"shop.example.com","origin":"notaurl"}`,
		`{"domain":"shop.example.com","origin":"ftp://origin.example"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status=%d, want 400", body, rec.Code)
		}
	}
}
