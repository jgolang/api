package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestProcessRequestSkipPathsIgnoreNonCanonicalEscapes checks that a skip
// path matches only a path with no non-canonical escape, so an escaped slash
// routed to another handler is still processed.
func TestProcessRequestSkipPathsIgnoreNonCanonicalEscapes(t *testing.T) {
	t.Cleanup(func() { SkipPaths = make(map[SkipPath]bool) })
	SkipPaths = make(map[SkipPath]bool)
	AddSkipPath(GenSkipPath(http.MethodGet, "/health/live"), true)
	AddSkipPath(GenSkipPath(http.MethodGet, "/status page"), true)
	AddSkipPath(GenSkipPath(http.MethodGet, "/report(1)"), true)

	tests := []struct {
		name        string
		target      string
		wantRoute   string
		wantSkipped bool
	}{
		{name: "skip path", target: "/health/live", wantRoute: "health", wantSkipped: true},
		{name: "skip path with query", target: "/health/live?verbose=1", wantRoute: "health", wantSkipped: true},
		{name: "canonically escaped space", target: "/status%20page", wantRoute: "tenant", wantSkipped: true},
		{name: "escaped slash", target: "/health%2Flive", wantRoute: "tenant", wantSkipped: false},
		{name: "lower-case escaped slash", target: "/health%2flive", wantRoute: "tenant", wantSkipped: false},
		{name: "double-encoded slash", target: "/health%252Flive", wantRoute: "tenant", wantSkipped: false},
		{name: "escaped letter", target: "/health/%6Cive", wantRoute: "health", wantSkipped: false},
		{name: "canonically escaped sub-delims", target: "/report%281%29", wantRoute: "tenant", wantSkipped: true},
		{name: "raw sub-delims are not canonical", target: "/report(1)", wantRoute: "tenant", wantSkipped: false},
		{name: "other route", target: "/acme", wantRoute: "tenant", wantSkipped: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newRecordingLogger()
			var route string
			mux := http.NewServeMux()
			mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { route = "health" })
			mux.HandleFunc("GET /{tenant}", func(w http.ResponseWriter, r *http.Request) { route = "tenant" })
			handler := New(WithLogger(logger)).Handler(ProcessRequest(mux.ServeHTTP))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.target, nil))

			if route != tt.wantRoute {
				t.Fatalf("route = %q, want %q", route, tt.wantRoute)
			}
			skipped := buf.Len() == 0
			if skipped != tt.wantSkipped {
				t.Fatalf("skipped = %v, want %v (log %q)", skipped, tt.wantSkipped, buf.String())
			}
		})
	}
}
