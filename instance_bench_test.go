package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
