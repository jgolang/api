package api

import (
	"context"
	"fmt"
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

var traceProviderRegistry = struct {
	sync.RWMutex
	providers map[string]TraceIDProvider
}{
	providers: make(map[string]TraceIDProvider),
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

// RegisterTraceProvider registers a named trace ID provider.
func RegisterTraceProvider(name string, provider TraceIDProvider) {
	if name == "" || provider == nil {
		return
	}
	traceProviderRegistry.Lock()
	defer traceProviderRegistry.Unlock()
	traceProviderRegistry.providers[name] = provider
}

// TraceProviderByName returns a registered trace ID provider.
func TraceProviderByName(name string) (TraceIDProvider, error) {
	traceProviderRegistry.RLock()
	provider, ok := traceProviderRegistry.providers[name]
	traceProviderRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("api trace provider %q is not registered", name)
	}
	return provider, nil
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
