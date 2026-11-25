package doc

import (
	"net/http"
	"reflect"
	"sync"
)

// API contains the public API metadata rendered into OpenAPI's info object.
// Contact and License are pointers, so == compares their identity, and each
// call to Docs.API returns new copies of them.
type API struct {
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`
}

// Server is an OpenAPI server: a base URL that clients and try-it-out tools
// send requests to.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// Contact is the OpenAPI contact information for the API.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License is the OpenAPI license information for the API. OpenAPI requires
// Name.
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
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
	// BodyExamples are the request body's named examples, written as the
	// OpenAPI examples map; when set, BodyExample is not written (OpenAPI
	// allows one or the other).
	BodyExamples map[string]interface{}
	// BodyContentType is the request body's media type, such as
	// multipart/form-data; empty means application/json. It is not written
	// when neither Body nor BodySchema is set.
	BodyContentType string
	Responses       []RouteResponse
	Parameters      []Parameter
	Security        []string
	// Public marks the operation as not requiring security (security: []),
	// overriding Security and any default security.
	Public bool
	// Hidden keeps the route out of the generated spec; Routes still
	// returns it.
	Hidden bool
}

// RouteResponse describes an HTTP response for an operation.
type RouteResponse struct {
	Status      int
	Description string
	Body        interface{}
	Schema      *Schema
	// ContentType is the response body's media type, such as
	// application/octet-stream; empty means application/json. It is not
	// written for a response without a body or schema.
	ContentType string
	Example     interface{}
	// Examples are named examples, written as the OpenAPI examples map; when
	// set, Example is not written (OpenAPI allows one or the other).
	Examples map[string]interface{}
	Headers  []ResponseHeaderInfo
	// Ref is the name of a component response added with Docs.AddResponse.
	// When set, every other field but Status is ignored and the response is
	// written as a $ref to components.responses.
	Ref string
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
	defaultSecurity []string
	servers         []Server
	responses       map[string]RouteResponse
}

// New creates an OpenAPI documentation collector.
func New(info API) *Docs {
	return &Docs{
		info:            cloneAPI(info),
		securitySchemes: make(map[string]SecurityScheme),
		responses:       make(map[string]RouteResponse),
	}
}

// API returns the docs API metadata.
func (docs *Docs) API() API {
	if docs == nil {
		return API{}
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	return cloneAPI(docs.info)
}

// cloneAPI copies the pointers so a caller cannot change the stored
// metadata through the returned value.
func cloneAPI(info API) API {
	clone := info
	if info.Contact != nil {
		contact := *info.Contact
		clone.Contact = &contact
	}
	if info.License != nil {
		license := *info.License
		clone.License = &license
	}
	return clone
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

// AddResponse registers a reusable response, written once under the spec's
// components.responses, that routes reference with ResponseRef. response is
// any option that adds exactly one response, such as ResponseSchema,
// ResponseExamples or an API's DocError. The option's status is not the
// status the response is documented under, which each referencing route
// chooses; it only shapes the component as the option builds it, such as its
// default description (the status text). An empty name, a nil option, and an
// option that adds no response or more than one are ignored. Adding a name
// again replaces its response.
func (docs *Docs) AddResponse(name string, response RouteOption) {
	if docs == nil || name == "" || response == nil {
		return
	}
	var scratch Route
	response(&scratch)
	if len(scratch.Responses) != 1 {
		return
	}
	docs.mu.Lock()
	defer docs.mu.Unlock()
	if docs.responses == nil {
		docs.responses = make(map[string]RouteResponse)
	}
	docs.responses[name] = cloneResponse(scratch.Responses[0])
}

func (docs *Docs) componentResponses() map[string]RouteResponse {
	if docs == nil {
		return nil
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	responses := make(map[string]RouteResponse, len(docs.responses))
	for name, response := range docs.responses {
		responses[name] = cloneResponse(response)
	}
	return responses
}

// SetDefaultSecurity sets the security schemes that apply to every operation
// without its own requirements, written as the spec's top-level security. Each
// name is a separate requirement, which OpenAPI reads as alternatives (any one
// is enough). A name with no scheme added through AddSecurityScheme is
// documented as bearer JWT. Empty names and repeats are ignored; routes marked
// with Public opt out. Each call replaces the previous names, so a call with
// no names clears the default.
func (docs *Docs) SetDefaultSecurity(names ...string) {
	if docs == nil {
		return
	}
	unique := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" && !hasName(unique, name) {
			unique = append(unique, name)
		}
	}
	docs.mu.Lock()
	defer docs.mu.Unlock()
	docs.defaultSecurity = unique
}

func hasName(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

// SetServers sets the servers written to the spec's top-level servers, in
// order, so try-it-out tools and generated clients get a base URL. Servers
// with an empty URL are ignored, since OpenAPI requires one. Each call
// replaces the previous servers; a call with none clears them.
func (docs *Docs) SetServers(servers ...Server) {
	if docs == nil {
		return
	}
	var kept []Server
	for _, server := range servers {
		if server.URL != "" {
			kept = append(kept, server)
		}
	}
	docs.mu.Lock()
	defer docs.mu.Unlock()
	docs.servers = kept
}

func (docs *Docs) serverList() []Server {
	if docs == nil {
		return nil
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	return append([]Server(nil), docs.servers...)
}

// DefaultSecurity returns the names set with SetDefaultSecurity.
func (docs *Docs) DefaultSecurity() []string {
	if docs == nil {
		return nil
	}
	docs.mu.RLock()
	defer docs.mu.RUnlock()
	return append([]string(nil), docs.defaultSecurity...)
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
		clone.Responses[i] = cloneResponse(route.Responses[i])
	}
	clone.BodySchema = cloneSchema(route.BodySchema)
	clone.BodyExample = cloneValue(route.BodyExample)
	clone.BodyExamples = cloneExamples(route.BodyExamples)
	return clone
}

func cloneResponse(response RouteResponse) RouteResponse {
	clone := response
	clone.Schema = cloneSchema(response.Schema)
	clone.Example = cloneValue(response.Example)
	clone.Examples = cloneExamples(response.Examples)
	clone.Headers = cloneResponseHeaders(response.Headers)
	return clone
}

func cloneExamples(examples map[string]interface{}) map[string]interface{} {
	if examples == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(examples))
	for name, value := range examples {
		clone[name] = cloneExample(value)
	}
	return clone
}

// cloneExample deep-clones a named example, including the value inside an
// ExampleObject, which cloneValue would copy only shallowly.
func cloneExample(example interface{}) interface{} {
	switch typed := example.(type) {
	case ExampleObject:
		return ExampleObject{Summary: typed.Summary, Value: cloneValue(typed.Value)}
	case *ExampleObject:
		if typed == nil {
			return typed
		}
		return &ExampleObject{Summary: typed.Summary, Value: cloneValue(typed.Value)}
	default:
		return cloneValue(example)
	}
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
	if schema.AllOf != nil {
		clone.AllOf = make([]*Schema, len(schema.AllOf))
		for i, part := range schema.AllOf {
			clone.AllOf[i] = cloneSchema(part)
		}
	}
	clone.AdditionalProperties = cloneSchema(schema.AdditionalProperties)
	clone.Required = append([]string(nil), schema.Required...)
	clone.Enum = append([]interface{}(nil), schema.Enum...)
	clone.Minimum = clonePointer(schema.Minimum)
	clone.Maximum = clonePointer(schema.Maximum)
	clone.MinLength = clonePointer(schema.MinLength)
	clone.MaxLength = clonePointer(schema.MaxLength)
	clone.Default = cloneValue(schema.Default)
	return &clone
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	clone := *value
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
