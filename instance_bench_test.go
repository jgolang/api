package api

import (
	"context"
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
