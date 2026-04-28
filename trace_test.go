package api

import (
	"context"
	"testing"
)

type staticTraceIDProvider struct {
	traceID string
}

func (provider staticTraceIDProvider) TraceID(context.Context) string {
	return provider.traceID
}

func TestGenerateEventIDUsesRegisteredTraceIDProvider(t *testing.T) {
	RegisterTraceIDProvider(staticTraceIDProvider{traceID: "trace-123"})
	defer RegisterTraceIDProvider(nil)

	eventID := generateEventID(context.Background(), "prefix", "/tasks")
	if eventID != "trace-123" {
		t.Fatalf("expected trace ID from provider, got %s", eventID)
	}
}

func TestRegisterTraceIDProviderNilRestoresFallback(t *testing.T) {
	RegisterTraceIDProvider(staticTraceIDProvider{traceID: "trace-123"})
	RegisterTraceIDProvider(nil)

	eventID := generateEventID(context.Background(), "prefix", "/tasks")
	if eventID == "" || eventID == "trace-123" {
		t.Fatalf("expected generated fallback event ID, got %s", eventID)
	}
}
