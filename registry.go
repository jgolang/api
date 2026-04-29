package api

import (
	"net/http"
	"sync"
)

// Info contains the public API metadata rendered into OpenAPI.
type Info struct {
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
}

// Registry stores route metadata for OpenAPI generation.
type Registry struct {
	info            Info
	mu              sync.RWMutex
	routes          []Route
	securitySchemes map[string]SecurityScheme
}

// NewRegistry creates a metadata registry.
func NewRegistry(info Info) *Registry {
	return &Registry{
		info:            info,
		securitySchemes: make(map[string]SecurityScheme),
	}
}

// Info returns the registry API metadata.
func (registry *Registry) Info() Info {
	if registry == nil {
		return Info{}
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.info
}

// Register stores a route and applies its declarative options.
func (registry *Registry) Register(method string, path string, opts ...RouteOption) Route {
	route := Route{
		Method: method,
		Path:   path,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&route)
		}
	}
	if registry == nil {
		return route
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.routes = append(registry.routes, cloneRoute(route))
	return route
}

// Routes returns a snapshot of registered routes.
func (registry *Registry) Routes() []Route {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	routes := make([]Route, len(registry.routes))
	for i := range registry.routes {
		routes[i] = cloneRoute(registry.routes[i])
	}
	return routes
}

// AddSecurityScheme registers a reusable OpenAPI security scheme.
func (registry *Registry) AddSecurityScheme(name string, scheme SecurityScheme) {
	if registry == nil || name == "" {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.securitySchemes[name] = scheme
}

// SecuritySchemes returns a snapshot of configured security schemes.
func (registry *Registry) SecuritySchemes() map[string]SecurityScheme {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	schemes := make(map[string]SecurityScheme, len(registry.securitySchemes))
	for name, scheme := range registry.securitySchemes {
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

// Summary sets the OpenAPI operation summary.
func Summary(summary string) RouteOption {
	return func(route *Route) {
		route.Summary = summary
	}
}

// OperationID sets the OpenAPI operationId used by client generators.
func OperationID(id string) RouteOption {
	return func(route *Route) {
		route.OperationID = id
	}
}

// Description sets the OpenAPI operation description.
func Description(description string) RouteOption {
	return func(route *Route) {
		route.Description = description
	}
}

// Tags sets the OpenAPI operation tags.
func Tags(tags ...string) RouteOption {
	return func(route *Route) {
		route.Tags = append(route.Tags, tags...)
	}
}

// Body sets the request body model inferred with reflection.
func Body(body interface{}) RouteOption {
	return func(route *Route) {
		route.Body = body
	}
}

// BodySchema sets a manual request body schema.
func BodySchema(schema *Schema) RouteOption {
	return func(route *Route) {
		route.BodySchema = schema
	}
}

// ResponseStatus adds a response model for a status code.
//
// The shorter name Response is already an exported interface in this package,
// so this option keeps backward compatibility with the existing API.
func ResponseStatus(status int, body interface{}) RouteOption {
	return ResponseWithDescription(status, "", body)
}

// Status is a short alias for ResponseStatus.
func Status(status int, body interface{}) RouteOption {
	return ResponseStatus(status, body)
}

// Responds is a readable alias for ResponseStatus.
func Responds(status int, body interface{}) RouteOption {
	return ResponseStatus(status, body)
}

// ResponseWithDescription adds a response model and description.
func ResponseWithDescription(status int, description string, body interface{}) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:      status,
			Description: description,
			Body:        body,
		})
	}
}

// ResponseSchema adds a response with a manual schema.
func ResponseSchema(status int, description string, schema *Schema) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:      status,
			Description: description,
			Schema:      schema,
		})
	}
}

// Query adds a query parameter.
func Query(name string, typ DataType, required bool) RouteOption {
	return parameter("query", name, typ, required)
}

// Path adds a path parameter.
func Path(name string, typ DataType, required bool) RouteOption {
	return parameter("path", name, typ, required)
}

// Security adds a named security requirement to the operation.
func Security(name string) RouteOption {
	return func(route *Route) {
		route.Security = append(route.Security, name)
	}
}

func parameter(in string, name string, typ DataType, required bool) RouteOption {
	return func(route *Route) {
		route.Parameters = append(route.Parameters, Parameter{
			Name:     name,
			In:       in,
			Type:     typ,
			Required: required,
		})
	}
}

// ErrorResponse is a small reusable schema model for documented errors.
type ErrorResponse struct {
	Title   string `json:"title,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

type registryRouter struct {
	registry *Registry
}

// NewMetadataRouter creates a router useful for tests and documentation-only registration.
func NewMetadataRouter(registry *Registry) Router {
	return registryRouter{registry: registry}
}

func (router registryRouter) Handle(method string, path string, handler http.HandlerFunc, opts ...RouteOption) {
	if router.registry != nil {
		router.registry.Register(method, path, opts...)
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
	}
	clone.BodySchema = cloneSchema(route.BodySchema)
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
