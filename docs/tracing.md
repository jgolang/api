# Trace ID provider

`github.com/jgolang/api` does not depend on a tracing library. Event IDs can be derived from request context through a small provider interface:

```go
type TraceIDProvider interface {
	TraceID(context.Context) string
}
```

Register a provider from the application when a tracing system is available:

```go
type MyTraceProvider struct{}

func (provider MyTraceProvider) TraceID(ctx context.Context) string {
	// Read a trace ID from OpenTelemetry, headers, or another tracing system.
	return ""
}

func main() {
	api.RegisterTraceIDProvider(MyTraceProvider{})
}
```

If no provider is registered, the library generates event IDs with its existing fallback.
