package doc

import (
	"net/http"
	"reflect"
	"sync"
)

// API contains the public API metadata rendered into OpenAPI.
type API struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// DataType describes primitive route parameter types.
type DataType string

const (
	String DataType = "string"
	Int    DataType = "integer"
	Float  DataType = "number"
	Bool   DataType = "boolean"
)

// Parameter describes a query, path, header, or cookie parameter.
type Parameter struct {
	Name        string
	In          string
	Type        DataType
	Required    bool
	Description string
	Schema      *Schema
}

// Route describes a registered HTTP operation.
type Route struct {
	Method      string
	Path        string
	OperationID string
	Summary     string
	Description string
	Tags        []string
	Body        interface{}
	BodySchema  *Schema
	BodyExample interface{}
	Responses   []RouteResponse
	Parameters  []Parameter
	Security    []string
}

// RouteResponse describes an HTTP response for an operation.
type RouteResponse struct {
	Status      int
	Description string
	Body        interface{}
	Schema      *Schema
	Example     interface{}
	Headers     []ResponseHeaderInfo
}

// ResponseHeaderInfo describes a documented response header.
type ResponseHeaderInfo struct {
	Name        string
	Type        DataType
	Description string
	Schema      *Schema
}

// Docs stores route metadata for OpenAPI generation.
type Docs struct {
	info            API
	mu              sync.RWMutex
	routes          []Route
	securitySchemes map[string]SecurityScheme
}

// New creates an OpenAPI documentation collector.
func New(info API) *Docs {
	return &Docs{
		info:            info,
		securitySchemes: make(map[string]SecurityScheme),
	}
}

// API returns the docs API metadata.
func (docs *Docs) API() API {
	if docs == nil {
		return API{}
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	return docs.info
}

// Register stores a route and applies its declarative options.
func (docs *Docs) Register(method string, path string, opts ...RouteOption) Route {
	route := Route{
		Method: method,
		Path:   path,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&route)
		}
	}
	if docs == nil {
		return route
	}
	docs.mu.Lock()
	defer docs.mu.Unlock()
	docs.routes = append(docs.routes, cloneRoute(route))
	return route
}

// Routes returns a snapshot of registered routes.
func (docs *Docs) Routes() []Route {
	if docs == nil {
		return nil
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	routes := make([]Route, len(docs.routes))
	for i := range docs.routes {
		routes[i] = cloneRoute(docs.routes[i])
	}
	return routes
}

// AddSecurityScheme registers a reusable OpenAPI security scheme.
func (docs *Docs) AddSecurityScheme(name string, scheme SecurityScheme) {
	if docs == nil || name == "" {
		return
	}
	docs.mu.Lock()
	defer docs.mu.Unlock()
	docs.securitySchemes[name] = scheme
}

// SecuritySchemes returns a snapshot of configured security schemes.
func (docs *Docs) SecuritySchemes() map[string]SecurityScheme {
	if docs == nil {
		return nil
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	schemes := make(map[string]SecurityScheme, len(docs.securitySchemes))
	for name, scheme := range docs.securitySchemes {
		schemes[name] = scheme
	}
	return schemes
}

// BearerSecurity returns an HTTP bearer security scheme.
func BearerSecurity(format string) SecurityScheme {
	return SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: format}
}

// BasicSecurity returns an HTTP basic security scheme.
func BasicSecurity() SecurityScheme {
	return SecurityScheme{Type: "http", Scheme: "basic"}
}

// APIKeySecurity returns an API key security scheme.
func APIKeySecurity(name string, in string) SecurityScheme {
	return SecurityScheme{Type: "apiKey", Name: name, In: in}
}

// RouteOption mutates route metadata.
type RouteOption func(*Route)

// Router is the minimal contract implemented by documentation-aware router adapters.
type Router interface {
	Handle(method string, path string, handler http.HandlerFunc, opts ...RouteOption)
}

type docsRouter struct {
	docs *Docs
}

// NewRouter creates a router useful for tests and documentation-only registration.
func NewRouter(docs *Docs) Router {
	return docsRouter{docs: docs}
}

func (router docsRouter) Handle(method string, path string, handler http.HandlerFunc, opts ...RouteOption) {
	if router.docs != nil {
		router.docs.Register(method, path, opts...)
	}
}

func cloneRoute(route Route) Route {
	clone := route
	clone.Tags = append([]string(nil), route.Tags...)
	clone.Parameters = append([]Parameter(nil), route.Parameters...)
	for i := range clone.Parameters {
		clone.Parameters[i].Schema = cloneSchema(route.Parameters[i].Schema)
	}
	clone.Security = append([]string(nil), route.Security...)
	clone.Responses = append([]RouteResponse(nil), route.Responses...)
	for i := range clone.Responses {
		clone.Responses[i].Schema = cloneSchema(route.Responses[i].Schema)
		clone.Responses[i].Example = cloneValue(route.Responses[i].Example)
		clone.Responses[i].Headers = cloneResponseHeaders(route.Responses[i].Headers)
	}
	clone.BodySchema = cloneSchema(route.BodySchema)
	clone.BodyExample = cloneValue(route.BodyExample)
	return clone
}

func cloneResponseHeaders(headers []ResponseHeaderInfo) []ResponseHeaderInfo {
	clone := append([]ResponseHeaderInfo(nil), headers...)
	for i := range clone {
		clone[i].Schema = cloneSchema(headers[i].Schema)
	}
	return clone
}

func cloneSchema(schema *Schema) *Schema {
	if schema == nil {
		return nil
	}
	clone := *schema
	if schema.Properties != nil {
		clone.Properties = make(map[string]*Schema, len(schema.Properties))
		for name, property := range schema.Properties {
			clone.Properties[name] = cloneSchema(property)
		}
	}
	clone.Items = cloneSchema(schema.Items)
	clone.AdditionalProperties = cloneSchema(schema.AdditionalProperties)
	clone.Required = append([]string(nil), schema.Required...)
	clone.Enum = append([]interface{}(nil), schema.Enum...)
	return &clone
}

func cloneValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			clone[key] = cloneValue(item)
		}
		return clone
	case []interface{}:
		clone := make([]interface{}, len(typed))
		for i, item := range typed {
			clone[i] = cloneValue(item)
		}
		return clone
	default:
		return cloneReflectValue(value)
	}
}

func cloneReflectValue(value interface{}) interface{} {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return value
	}
	switch reflected.Kind() {
	case reflect.Map:
		if reflected.Type().Key().Kind() != reflect.String {
			return value
		}
		clone := reflect.MakeMapWithSize(reflected.Type(), reflected.Len())
		iter := reflected.MapRange()
		for iter.Next() {
			clone.SetMapIndex(iter.Key(), reflect.ValueOf(cloneValue(iter.Value().Interface())))
		}
		return clone.Interface()
	case reflect.Slice:
		clone := reflect.MakeSlice(reflected.Type(), reflected.Len(), reflected.Len())
		for i := 0; i < reflected.Len(); i++ {
			clone.Index(i).Set(reflect.ValueOf(cloneValue(reflected.Index(i).Interface())))
		}
		return clone.Interface()
	default:
		return value
	}
}
