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
responses may use only `header` when there is no payload.

Use `doc.BodyWithExample` and `doc.StatusWithExample` to add complete request or
response examples to the generated OpenAPI media type.

These types are for documentation only. They do not change the runtime request or response format.
In generated OpenAPI documents, inferred Go types are emitted under
`components.schemas` and referenced with `$ref` to avoid repeating schemas per
operation.

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

Adapters for third-party routers should live outside this module. A minimal adapter only needs to translate `api.Router.Handle` to the chosen router and register metadata:

```go
type RouterAdapter struct {
	docs *doc.Docs
	// router *yourRouter
}

func (adapter *RouterAdapter) Handle(method string, path string, handler http.HandlerFunc, opts ...doc.RouteOption) {
	if adapter.docs != nil {
		adapter.docs.Register(method, path, opts...)
	}

	// Translate method, path, and handler to your router here.
}
```
