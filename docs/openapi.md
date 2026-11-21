# Router-agnostic OpenAPI documentation

The package includes a small router-agnostic documentation layer. Adapters implement the `api.Router` interface and store route metadata in `doc.Docs`, which can generate an OpenAPI 3 document and Swagger UI.

This module intentionally avoids importing third-party routers. The native adapter uses only Go's standard `net/http` package. If an application uses Gin, Echo, chi, gorilla/mux, or another router, create the adapter in that application or in a separate module owned by the implementer. The philosophy is to keep `github.com/jgolang/api` small, predictable, and free of router-specific dependencies.

In general, dependencies should come from Go's standard library or from `github.com/jgolang/...`. Exceptions should be explicit and intentional.

## Example

```go
package main

import (
	"net/http"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/api/stdadapter"
)

type CreateTaskRequest struct {
	Title string `json:"title"`
}

type TaskResponse struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func createTask(w http.ResponseWriter, r *http.Request) {
	api.Success201().Write(w, r)
}

func listTasks(w http.ResponseWriter, r *http.Request) {
	api.Success200().Write(w, r)
}

func main() {
	docs := doc.New(doc.API{
		Title:   "Tasks API",
		Version: "1.0.0",
	})
	docs.AddSecurityScheme("bearerAuth", doc.BearerSecurity("JWT"))

	router := stdadapter.New(http.NewServeMux(), docs)

	api.Post(router, "/tasks", createTask,
		doc.OperationID("createTask"),
		doc.Summary("Create task"),
		doc.Tags("tasks"),
		doc.HeaderWithDescription("X-Request-ID", doc.String, false, "Trace request ID"),
		doc.Body(envelope.Request[CreateTaskRequest]()),
		doc.Security("bearerAuth"),
		doc.StatusWithHeaders(201, envelope.Success[TaskResponse](),
			doc.ResponseHeader("Location", doc.String, "Created task URL"),
		),
		doc.Status(400, envelope.Error()),
	)

	api.Get(router, "/tasks", listTasks,
		doc.Summary("List tasks"),
		doc.Tags("tasks"),
		doc.Status(200, envelope.Success[[]TaskResponse]()),
	)

	router.Handle("GET", "/openapi.json", doc.OpenAPIHandler(docs))
	router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json"))

	http.ListenAndServe(":8080", router)
}
```

The `doc` subpackage contains the OpenAPI metadata helpers, schema generation, reusable security schemes, and HTTP handlers for documentation.

## Docs page title and icon

`doc.SwaggerUIHandler` serves a page titled `API Docs` with a generic inline icon, so the browser tab never shows its blank-page globe. Pass options to make each service's docs tab recognizable:

```go
router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json",
	doc.WithTitle("Orders API"),
	doc.WithFavicon("/static/orders-icon.png"),
))
```

`WithTitle` sets the `<title>`. `WithFavicon` replaces the default icon with a URL or a `data:` URI (PNG, SVG, ICO). Both values are HTML-escaped; an empty value keeps the default. Percent-encode (`url.PathEscape`) or base64-encode an SVG `data:` URI: a raw `#` (as in `fill="#fff"`) starts a URL fragment and cuts the image.

The built-in `"swagger"` exporter (`doc.ExporterByName("swagger")`) always serves the defaults. To pass options through the exporter registry, register an exporter of your own:

```go
type ordersExporter struct{}

func (ordersExporter) Handler(openAPIURL string) http.HandlerFunc {
	return doc.SwaggerUIHandler(openAPIURL, doc.WithTitle("Orders API"), doc.WithFavicon("/static/orders-icon.png"))
}

func init() {
	doc.RegisterExporter("orders-swagger", ordersExporter{})
}
```

Registering under `"swagger"` instead replaces the built-in exporter for the whole process.

## Request and response wrappers

Runtime requests use `api.JSONRequest`, whose `content` field is `json.RawMessage` for backward compatibility. For OpenAPI documentation, use typed wrappers so the generator can infer the payload schema:

```go
doc.Body(envelope.Request[CreateTaskRequest]())
```

The typed request wrapper documents `content` as optional, so endpoints that only
receive `header` can still use the same JSON request envelope.

Runtime responses use `api.JSONResponse`, whose `content` field is `interface{}` for backward compatibility. For OpenAPI documentation, use typed wrappers so the generator can infer the payload schema:

```go
doc.Status(http.StatusOK, envelope.Success[TaskResponse]())
doc.Status(http.StatusOK, envelope.Success[[]TaskResponse]())
doc.Status(http.StatusBadRequest, envelope.Error())
```

The typed response wrapper also documents `content` as optional, so successful
responses may use only `header` when there is no payload. `header` is required
in `envelope.Success` and `envelope.Error`, as on the wire, and its `code`
example is `0000`, the default success code; an error sends its own code.

Use `doc.BodyWithExample` and `doc.StatusWithExample` to add complete request or
response examples to the generated OpenAPI media type.

These types are for documentation only. They do not change the runtime request or response format.
In generated OpenAPI documents, inferred Go types are emitted under
`components.schemas` and referenced with `$ref` to avoid repeating schemas per
operation.
A reference that also needs `nullable` (a pointer field, slice item or map
value, such as `*Owner` or `[]*Owner`) or a tag such as `description` is
written as `allOf: [{$ref}]` with those keys beside `allOf`, because OpenAPI
3.0.3 ignores every key next to `$ref`. A body passed as a pointer
(`doc.Body(&CreateTaskRequest{})`) is a sample value and stays a plain `$ref`.

## Route metadata options

- `doc.Summary("Create task")`
- `doc.OperationID("createTask")`
- `doc.Description("Creates a new task")`
- `doc.Tags("tasks")`
- `doc.Body(envelope.Request[CreateTaskRequest]())`
- `doc.BodyWithExample(envelope.Request[CreateTaskRequest](), map[string]any{"content": map[string]any{"title": "Buy milk"}})`
- `doc.BodySchema(&doc.Schema{Type: "object"})`
- `doc.Status(200, envelope.Success[TaskResponse]())`
- `doc.StatusWithExample(200, envelope.Success[TaskResponse](), map[string]any{"content": map[string]any{"id": 1}})`
- `doc.StatusWithHeaders(201, envelope.Success[TaskResponse](), doc.ResponseHeader("Location", doc.String, "Created resource URL"))`
- `doc.ResponseSchema(400, "Bad Request", &doc.Schema{Type: "object"})`
- `doc.ResponseExamples(409, "Transition not allowed or system record", ErrorBody{}, map[string]any{"transition": transitionExample, "system": systemExample})` (OpenAPI `examples` map; one option per status, a later one for the same status replaces it)
- `doc.Query("page", doc.Int, false)`
- `doc.QueryWithDescription("page", doc.Int, false, "Page number")`
- `doc.QueryEnum("in_inbox", doc.Bool, false, "Only true is accepted.", true)` (writes `schema.enum`; values match the type)
- `doc.Header("X-Request-ID", doc.String, false)`
- `doc.HeaderWithDescription("X-Request-ID", doc.String, false, "Trace request ID")`
- `doc.Path("id", doc.Int, true)`
- `doc.PathWithDescription("id", doc.Int, true, "Resource ID")`
- `doc.PathEnum("status", doc.String, true, "Task status.", "open", "done")`
- `doc.Public()` (writes `security: []`; overrides `doc.Security` and the default)
- `doc.Security("bearerAuth")`

## Security schemes

Security requirements are declared per route with `doc.Security("name")`. Reusable schemes are registered in docs:

```go
docs.AddSecurityScheme("bearerAuth", doc.BearerSecurity("JWT"))
docs.AddSecurityScheme("basicAuth", doc.BasicSecurity())
docs.AddSecurityScheme("apiKeyAuth", doc.APIKeySecurity("X-API-Key", "header"))
```

If a route references a security name that was not registered, the OpenAPI generator assumes an HTTP bearer JWT scheme by default.

To require a scheme on every route, set it once on the docs and mark the exceptions public:

```go
docs.SetDefaultSecurity("bearerAuth") // top-level security
api.Get(router, "/health", health, doc.Public()) // security: []
```

Each name passed to `SetDefaultSecurity` is a separate requirement, which OpenAPI reads as alternatives (any one is enough). `doc.Public()` writes `security: []`, ignores the route's `doc.Security` options and adds no scheme for them. `Operation.Public` carries the flag in Go; `Operation`'s JSON methods read and write it as `security: []`, and a struct that embeds `doc.Operation` marshals as `Operation`.

## Schema tags

Schemas are inferred from Go structs with reflection. The generator understands `json` tags and a few OpenAPI-oriented tags:

```go
type CreateUserRequest struct {
	Email string `json:"email" format:"email" example:"user@example.com" validate:"required"`
	Name  string `json:"name,omitempty"`
}
```

Supported tags:

- `json:"name,omitempty"` controls property names and optional fields.
- `description:"..."` sets the schema description.
- `format:"email"` sets the OpenAPI format.
- `example:"..."` sets an example value. JSON literals such as `3`, `true`, or `{"x":1}` are parsed.
- `validate:"required"` marks a field as required even when it has `omitempty`.

## External router adapter example

Adapters for third-party routers live outside this module, so `github.com/jgolang/api` never depends on them. An adapter does four things:

1. registers a factory with `api.RegisterAdapter`, so `api.NewRouter("gin", engine, docs)` works;
2. calls `docs.Register` in `Handle` after registering the route on the router, so the route appears in `/openapi.json` and a route the router rejects is not documented;
3. converts `{id}` path variables to the router's syntax (`:id` for Gin);
4. copies the router's path parameters into `api.SetRouteVars`, so `api.GetRouteVar` works in handlers.

A complete Gin adapter, as its own module (`go get github.com/gin-gonic/gin github.com/jgolang/api`); it compiles against `jgolang/api` v0.8.0:

```go
// Package ginadapter shows an api.Router adapter for Gin, kept outside
// github.com/jgolang/api so the library does not depend on Gin.
package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
)

func init() {
	api.RegisterAdapter("gin", func(target any, docs *doc.Docs) (api.Router, error) {
		engine, ok := target.(*gin.Engine)
		if !ok {
			return nil, fmt.Errorf("gin adapter expects *gin.Engine")
		}
		return &Router{engine: engine, docs: docs}, nil
	})
}

// Router adapts *gin.Engine to api.Router.
type Router struct {
	engine *gin.Engine
	docs   *doc.Docs
}

// Handle registers the route on Gin, then documents it.
func (router *Router) Handle(method string, path string, handler http.HandlerFunc, opts ...doc.RouteOption) {
	router.engine.Handle(method, ginPath(path), func(c *gin.Context) {
		r := c.Request
		if len(c.Params) > 0 {
			vars := make(map[string]string, len(c.Params))
			for _, param := range c.Params {
				vars[param.Key] = param.Value
			}
			r = api.SetRouteVars(vars, r)
		}
		handler(c.Writer, r)
	})
	if router.docs != nil {
		router.docs.Register(method, path, opts...)
	}
}

// ServeHTTP lets the adapter be passed to http.ListenAndServe.
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.engine.ServeHTTP(w, r)
}

// ginPath turns "/tasks/{id}" into Gin's "/tasks/:id".
func ginPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = ":" + strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		}
	}
	return strings.Join(parts, "/")
}

func main() {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router, err := api.NewRouter("gin", gin.New(), docs)
	if err != nil {
		panic(err)
	}

	api.Get(router, "/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "task", api.GetRouteVar("id", r))
	}, doc.Summary("Get task"), doc.PathWithDescription("id", doc.String, true, "Task ID"))
	router.Handle(http.MethodGet, "/openapi.json", doc.OpenAPIHandler(docs))

	_ = http.ListenAndServe(":8080", router.(http.Handler))
}
```
