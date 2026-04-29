# Router-agnostic OpenAPI documentation

The package includes a small router-agnostic registration layer. Adapters implement the `api.Router` interface and store route metadata in an `api.Registry`, which can generate an OpenAPI 3 document and Swagger UI.

This module intentionally avoids importing third-party routers. The native adapter uses only Go's standard `net/http` package. If an application uses Gin, Echo, chi, gorilla/mux, or another router, create the adapter in that application or in a separate module owned by the implementer. The philosophy is to keep `github.com/jgolang/api` small, predictable, and free of router-specific dependencies.

In general, dependencies should come from Go's standard library or from `github.com/jgolang/...`. Exceptions should be explicit and intentional.

## Example

```go
package main

import (
	"net/http"

	"github.com/jgolang/api"
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
	registry := api.NewRegistry(api.Info{
		Title:   "Tasks API",
		Version: "1.0.0",
	})
	registry.AddSecurityScheme("bearerAuth", api.BearerSecurity("JWT"))

	router := stdadapter.New(http.NewServeMux(), registry)

	api.Post(router, "/tasks", createTask,
		api.OperationID("createTask"),
		api.Summary("Create task"),
		api.Tags("tasks"),
		api.HeaderWithDescription("X-Request-ID", api.String, false, "Trace request ID"),
		api.Body(api.RequestDoc[CreateTaskRequest]()),
		api.Security("bearerAuth"),
		api.StatusWithHeaders(201, api.SuccessDoc[TaskResponse](),
			api.ResponseHeader("Location", api.String, "Created task URL"),
		),
		api.Status(400, api.ErrorDoc()),
	)

	api.Get(router, "/tasks", listTasks,
		api.Summary("List tasks"),
		api.Tags("tasks"),
		api.Status(200, api.SuccessDoc[[]TaskResponse]()),
	)

	router.Handle("GET", "/openapi.json", api.OpenAPIHandler(registry))
	router.Handle("GET", "/docs", api.SwaggerUIHandler("/openapi.json"))

	http.ListenAndServe(":8080", router)
}
```

`api.Status`, `api.Responds`, and `api.ResponseStatus` document responses. The longer `ResponseStatus` name is kept because this package already has an exported `api.Response` interface.

## Request and response wrappers

Runtime requests use `api.JSONRequest`, whose `content` field is `json.RawMessage` for backward compatibility. For OpenAPI documentation, use typed wrappers so the generator can infer the payload schema:

```go
api.Body(api.JSONRequestOf[CreateTaskRequest]{})
api.Body(api.RequestDoc[CreateTaskRequest]())
```

The typed request wrapper documents `content` as optional, so endpoints that only
receive `header` can still use the same JSON request envelope.

Runtime responses use `api.JSONResponse`, whose `content` field is `interface{}` for backward compatibility. For OpenAPI documentation, use typed wrappers so the generator can infer the payload schema:

```go
api.Status(http.StatusOK, api.JSONResponseOf[TaskResponse]{})
api.Status(http.StatusOK, api.JSONResponseOf[[]TaskResponse]{})
api.Status(http.StatusBadRequest, api.JSONErrorResponse{})
```

The typed response wrapper also documents `content` as optional, so successful
responses may use only `header` when there is no payload.

The helpers below are equivalent and often read better in route declarations:

```go
api.Status(http.StatusOK, api.SuccessDoc[TaskResponse]())
api.Status(http.StatusBadRequest, api.ErrorDoc())
```

These types are for documentation only. They do not change the runtime request or response format.
In generated OpenAPI documents, inferred Go types are emitted under
`components.schemas` and referenced with `$ref` to avoid repeating schemas per
operation.

## Route metadata options

- `api.Summary("Create task")`
- `api.OperationID("createTask")`
- `api.Description("Creates a new task")`
- `api.Tags("tasks")`
- `api.Body(CreateTaskRequest{})`
- `api.BodySchema(&api.Schema{Type: "object"})`
- `api.Status(200, TaskResponse{})`
- `api.StatusWithHeaders(201, TaskResponse{}, api.ResponseHeader("Location", api.String, "Created resource URL"))`
- `api.ResponseStatus(200, TaskResponse{})`
- `api.ResponseSchema(400, "Bad Request", &api.Schema{Type: "object"})`
- `api.Query("page", api.Int, false)`
- `api.QueryWithDescription("page", api.Int, false, "Page number")`
- `api.Header("X-Request-ID", api.String, false)`
- `api.HeaderWithDescription("X-Request-ID", api.String, false, "Trace request ID")`
- `api.Path("id", api.Int, true)`
- `api.PathWithDescription("id", api.Int, true, "Resource ID")`
- `api.Security("bearerAuth")`

## Security schemes

Security requirements are declared per route with `api.Security("name")`. Reusable schemes are registered in the registry:

```go
registry.AddSecurityScheme("bearerAuth", api.BearerSecurity("JWT"))
registry.AddSecurityScheme("basicAuth", api.BasicSecurity())
registry.AddSecurityScheme("apiKeyAuth", api.APIKeySecurity("X-API-Key", "header"))
```

If a route references a security name that was not registered, the OpenAPI generator assumes an HTTP bearer JWT scheme by default.

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
	registry *api.Registry
	// router *yourRouter
}

func (adapter *RouterAdapter) Handle(method string, path string, handler http.HandlerFunc, opts ...api.RouteOption) {
	if adapter.registry != nil {
		adapter.registry.Register(method, path, opts...)
	}

	// Translate method, path, and handler to your router here.
}
```
