package api

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// BenchmarkSuccessWrite measures the cost of the instance lookup on the
// response path: "default" has no instance in the context, "instance" has the
// context that (*API).Handler builds.
func BenchmarkSuccessWrite(b *testing.B) {
	content := map[string]string{"status": "ok"}
	defaultRequest := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	instanceRequest := defaultRequest.WithContext(context.WithValue(defaultRequest.Context(), instanceKey{}, New()))

	tests := []struct {
		name string
		r    *http.Request
	}{
		{name: "default", r: defaultRequest},
		{name: "instance", r: instanceRequest},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				SuccessWithContent(content).Write(httptest.NewRecorder(), tt.r)
			}
		})
	}
}

// BenchmarkWriteError measures WriteError on the default instance, on an
// instance without options, and on one with a kind override and an error hook.
func BenchmarkWriteError(b *testing.B) {
	err := errors.NewC(planValidation, "title")
	defaultRequest := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	withInstance := func(a *API) *http.Request {
		return defaultRequest.WithContext(context.WithValue(defaultRequest.Context(), instanceKey{}, a))
	}
	overrideAndHook := New(
		WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity),
		WithErrorHook(func(*http.Request, error, int) {}),
	)

	tests := []struct {
		name string
		r    *http.Request
	}{
		{name: "default", r: defaultRequest},
		{name: "instance", r: withInstance(New())},
		{name: "instance with override and hook", r: withInstance(overrideAndHook)},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				WriteError(httptest.NewRecorder(), tt.r, err)
			}
		})
	}
}

// BenchmarkProcessRequest measures ProcessRequest's logging on the default
// instance (with slog.Default discarding every record), on an instance
// without a logger and on one whose logger discards every record.
func BenchmarkProcessRequest(b *testing.B) {
	previous, previousOutput, previousFlags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	b.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})
	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		Success200().Write(w, r)
	})

	tests := []struct {
		name    string
		handler http.Handler
	}{
		{name: "default", handler: handler},
		{name: "instance without logger", handler: New().Handler(handler)},
		{name: "instance with discard logger", handler: New(WithLogger(slog.New(slog.DiscardHandler))).Handler(handler)},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				req := httptest.NewRequest(http.MethodGet, "/tasks?page=2", nil)
				req.Header.Set("Authorization", "Bearer placeholder")
				tt.handler.ServeHTTP(httptest.NewRecorder(), req)
			}
		})
	}
}

// BenchmarkRedactHeaders measures masking a typical request header set: a
// sensitive name, two values without URL shape and a Referer with a query.
func BenchmarkRedactHeaders(b *testing.B) {
	headers := http.Header{
		"Authorization": []string{"Bearer placeholder"},
		"Accept":        []string{"application/json"},
		"User-Agent":    []string{"Mozilla/5.0 (X11; Linux) Firefox/120.0"},
		"Referer":       []string{"https://app.example/cb?token=placeholder&page=2"},
	}
	sensitiveKeys := defaultSensitiveLogKeys()

	b.ReportAllocs()
	for b.Loop() {
		redactHeaders(headers, sensitiveKeys)
	}
}
