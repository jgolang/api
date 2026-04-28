package api

import (
	"context"
	"sync"
)

// TraceIDProvider retrieves a trace ID from a context.
type TraceIDProvider interface {
	TraceID(context.Context) string
}

type noopTraceIDProvider struct{}

func (provider noopTraceIDProvider) TraceID(context.Context) string {
	return ""
}

var traceIDProvider = struct {
	sync.RWMutex
	provider TraceIDProvider
}{
	provider: noopTraceIDProvider{},
}

// RegisterTraceIDProvider configures the provider used to derive event IDs from context.
func RegisterTraceIDProvider(provider TraceIDProvider) {
	traceIDProvider.Lock()
	defer traceIDProvider.Unlock()
	if provider == nil {
		traceIDProvider.provider = noopTraceIDProvider{}
		return
	}
	traceIDProvider.provider = provider
}

func getTraceID(ctx context.Context) string {
	traceIDProvider.RLock()
	provider := traceIDProvider.provider
	traceIDProvider.RUnlock()
	if provider == nil {
		return ""
	}
	return provider.TraceID(ctx)
}
