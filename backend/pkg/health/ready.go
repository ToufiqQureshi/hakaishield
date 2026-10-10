package health

import (
	"context"
	"net/http"
	"time"
)

// Probe checks a dependency needed for the service to operate normally.
type Probe func(context.Context) error

// ReadyHandler is served only on the operator's health listener. It does not
// disclose dependency names or error details to callers.
func ReadyHandler(shuttingDown <-chan struct{}, probes ...Probe) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		select {
		case <-shuttingDown:
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		default:
		}
		if len(probes) == 0 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for _, probe := range probes {
			if probe == nil || probe(ctx) != nil || ctx.Err() != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
}
