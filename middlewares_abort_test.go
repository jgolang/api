package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveRecovering runs handler once and returns the recorded response and
// the value that panicked out of ServeHTTP, nil when none did.
func serveRecovering(handler http.Handler) (rec *httptest.ResponseRecorder, recovered any) {
	rec = httptest.NewRecorder()
	defer func() { recovered = recover() }()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tasks", nil))
	return rec, nil
}

// newDebugLogger returns a logger that writes JSON records of every level
// into buf.
func newDebugLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// assertAborted checks that http.ErrAbortHandler left ServeHTTP, that
// nothing was written, and that "request" and "handler aborted" are the only
// records.
func assertAborted(t *testing.T, rec *httptest.ResponseRecorder, recovered any, buf *bytes.Buffer) {
	t.Helper()
	// Only the plain sentinel keeps net/http quiet; a wrapped value would be
	// logged by the server with its text and a stack.
	if recovered != http.ErrAbortHandler {
		t.Fatalf("recovered = %v, want http.ErrAbortHandler re-panicked", recovered)
	}
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 || len(rec.Header()) != 0 {
		t.Fatalf("response = %d %q %v, want nothing written", rec.Code, rec.Body, rec.Header())
	}
	records := decodeRecords(t, buf)
	record := recordNamed(t, records, "handler aborted")
	if record["level"] != "DEBUG" || record["event_id"] == nil || record["panic"] == nil {
		t.Fatalf("record = %v, want a debug record with event_id and panic", record)
	}
	recordNamed(t, records, "request")
	if len(records) != 2 {
		t.Fatalf("records = %v, want only request and handler aborted", records)
	}
}

func TestInstanceRepanicsAbortHandler(t *testing.T) {
	tests := []struct {
		name  string
		abort error
	}{
		{name: "plain", abort: http.ErrAbortHandler},
		{name: "wrapped", abort: fmt.Errorf("upstream failed: %w", http.ErrAbortHandler)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			var buf bytes.Buffer
			handler := ProcessRequest(func(http.ResponseWriter, *http.Request) { panic(tt.abort) })

			rec, recovered := serveRecovering(New(WithLogger(newDebugLogger(&buf))).Handler(handler))

			assertAborted(t, rec, recovered, &buf)
		})
	}
}

func TestDefaultInstanceRepanicsAbortHandler(t *testing.T) {
	buf := captureDefaultLogger(t)
	slog.SetDefault(newDebugLogger(buf))
	failOnPackageLoggers(t)
	handler := ProcessRequest(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })

	rec, recovered := serveRecovering(handler)

	assertAborted(t, rec, recovered, buf)
}
