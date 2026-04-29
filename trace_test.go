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

func TestRegisteredTraceProviderCanBeResolved(t *testing.T) {
	RegisterTraceProvider("test-trace", staticTraceIDProvider{traceID: "trace-123"})

	provider, err := TraceProviderByName("test-trace")
	if err != nil {
		t.Fatalf("expected registered provider, got error: %v", err)
	}
	if provider.TraceID(context.Background()) != "trace-123" {
		t.Fatalf("unexpected trace ID")
	}
}

func TestTraceProviderByNameReturnsErrorForMissingProvider(t *testing.T) {
	provider, err := TraceProviderByName("missing-trace")
	if err == nil {
		t.Fatalf("expected error")
	}
	if provider != nil {
		t.Fatalf("expected nil provider, got %#v", provider)
	}
}

func TestRegisterTraceProviderIgnoresInvalidInput(t *testing.T) {
	RegisterTraceProvider("", staticTraceIDProvider{traceID: "trace-123"})
	RegisterTraceProvider("nil-trace", nil)

	if _, err := TraceProviderByName(""); err == nil {
		t.Fatalf("empty trace provider name should not be registered")
	}
	if _, err := TraceProviderByName("nil-trace"); err == nil {
		t.Fatalf("nil trace provider should not be registered")
	}
}
