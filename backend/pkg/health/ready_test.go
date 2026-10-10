package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadyHandler(t *testing.T) {
	shutdown := make(chan struct{})
	called := 0
	ok := func(context.Context) error { called++; return nil }
	fail := func(context.Context) error { return errors.New("dependency unavailable") }
	for _, tc := range []struct {
		name   string
		method string
		probes []Probe
		status int
	}{
		{"healthy", http.MethodGet, []Probe{ok, ok}, http.StatusOK},
		{"database down", http.MethodGet, []Probe{fail, ok}, http.StatusServiceUnavailable},
		{"redis down", http.MethodGet, []Probe{ok, fail}, http.StatusServiceUnavailable},
		{"missing probe", http.MethodGet, []Probe{nil}, http.StatusServiceUnavailable},
		{"no probes", http.MethodGet, nil, http.StatusServiceUnavailable},
		{"method", http.MethodPost, []Probe{ok}, http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ReadyHandler(shutdown, tc.probes...).ServeHTTP(rec, httptest.NewRequest(tc.method, "/ready", nil))
			if rec.Code != tc.status {
				t.Fatalf("got %d, want %d", rec.Code, tc.status)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("readiness response must not be cached")
			}
		})
	}
	if called == 0 {
		t.Fatal("healthy probes were never called")
	}
	close(shutdown)
	rec := httptest.NewRecorder()
	ReadyHandler(shutdown, ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("shutdown status = %d", rec.Code)
	}
}
