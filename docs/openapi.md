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
	docs.SetServers(doc.Server{URL: "http://localhost:8080"})
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

	router.Handle("GET", "/openapi.json", doc.OpenAPIHandler(docs), doc.Hidden())
	router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json"), doc.Hidden())

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
), doc.Hidden())
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

The envelope wrappers document the envelope response mode. Other response modes
write other bodies, and have their own wrappers:

```go
// ResponseModePlain: the header keys and content at the top level.
doc.Status(http.StatusOK, envelope.PlainSuccess[TaskResponse]())
doc.Status(http.StatusBadRequest, envelope.PlainError())

// ResponseModeProblem: an error as RFC 9457 problem details.
doc.Status(http.StatusNotFound, envelope.Problem())
```

`doc.Status` documents the problem body under `application/json`, while problem
mode sends `application/problem+json`; `(*api.API).DocError` documents it under
`application/problem+json` (see [Instance response options](#instance-response-options)).

Plain mode writes all nine keys (`title`, `message`, `type`, `code`, `token`,
`action`, `event_id`, `info`, `content`), so the plain wrappers list them all as
required; `info` and `content` are nullable, because the wire writes `null` when
they are unset. `envelope.PlainError` documents `content` as any nullable value.
`envelope.Problem` lists `type` as required and `title`, `status`, `detail`,
`code` and `event_id` as optional, as the problem body omits them when empty. Any
response other than an error writes its content alone in problem mode, so
document it with the content type itself (`doc.Status(200, TaskResponse{})`).

Use `doc.BodyWithExample` and `doc.StatusWithExample` to add complete request or
response examples to the generated OpenAPI media type. For several labelled
cases use `doc.BodyExamples` or `doc.ResponseExamples`, written as the media
type's `examples` map instead of `example` (OpenAPI allows one or the other). A
`doc.ExampleObject{Summary: "...", Value: v}` value in that map is written as is,
with its summary; any other value is written as `{"value": v}`.

These types are for documentation only. They do not change the runtime request or response format.
In generated OpenAPI documents, inferred Go types are emitted under
`components.schemas` and referenced with `$ref` to avoid repeating schemas per
operation.
A reference that also needs `nullable` (a pointer field, slice item or map
value, such as `*Owner` or `[]*Owner`) or a tag such as `description` is
written as `allOf: [{$ref}]` with those keys beside `allOf`, because OpenAPI
3.0.3 ignores every key next to `$ref`. A body passed as a pointer
(`doc.Body(&CreateTaskRequest{})`) is a sample value and stays a plain `$ref`.

## Instance response options

`envelope.Success[T]()` and `envelope.Error()` describe the envelope mode only,
so a route documented with them is wrong once its instance sends another
mode. `(*api.API).DocSuccess` and `(*api.API).DocError` return a
`doc.RouteOption` that wraps the payload the way the instance's built-in
formatter sends it:

```go
instance := api.New(api.WithResponseMode(api.ResponseModeEnvelope))
// or api.New(api.WithResponseMode(api.ResponseModeNone))
// or api.New(api.WithResponseMode(api.ResponseModePlain))
// or api.New(api.WithResponseMode(api.ResponseModeProblem))

docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
router := stdadapter.New(http.NewServeMux(), docs)
api.Get(router, "/tasks/{taskID}", func(w http.ResponseWriter, r *http.Request) {},
	instance.DocSuccess(http.StatusOK, TaskResponse{}),
	instance.DocError(http.StatusNotFound, "Task not found"),
)
```

The route declaration is the same in every mode; only the instance changes.
The spec then documents, per status, the media type and schema below (the
output of the `ExampleAPI_DocSuccess_envelope`, `_none`, `_plain` and
`_problem` examples in `example_doc_options_test.go`; the plain success
schema is abbreviated):

| Mode | 200 | 404 |
|------|-----|-----|
| envelope | `application/json`: object with required `header` (`$ref` `ResponseInfo`) and nullable `content` (`$ref` `TaskResponse`) | `application/json`: `$ref` `ErrorResponse` |
| none | `application/json`: `$ref` `TaskResponse` | `application/json`: `{"type":"object","enum":[null],"nullable":true}` |
| plain | `application/json`: object with the nine plain keys required, `content` nullable `$ref` `TaskResponse` | `application/json`: `$ref` `PlainErrorResponse` |
| problem | `application/json`: `$ref` `TaskResponse` | `application/problem+json`: `$ref` `ProblemDetails` |

For any content type `T`, and for a nil sample:

| Mode | `DocSuccess(status, T{})` | `DocSuccess(status, nil)` | `DocError(status, description)` |
|------|---------------------------|---------------------------|---------------------------------|
| envelope | inline object: required `header` (`ResponseInfo`), optional `content` (`T`) | object with required `header` only | `ErrorResponse` |
| none | `T` | JSON `null` | JSON `null` |
| plain | inline object with the nine plain keys required; `content` (`T`) and `info` nullable | as `T{}`, with `content` any nullable value | `PlainErrorResponse` |
| problem | `T` | JSON `null` | `ProblemDetails` under `application/problem+json` |

The envelope and plain inline objects copy the fields and tags of
`envelope.ResponseOf` and `envelope.PlainResponseOf`, with `T` as the content
type, so they document what those wrappers document. Every response other than
a problem-mode error is documented under `application/json`.

`sample` is a Go value of the content type, and its type is what is
documented; a schema written by hand goes to `doc.ResponseSchema`. A status that never carries
a body (1xx, 204, 304) is documented without content in every mode.
A pointer sample documents the type it points to. A JSON `null` body is
written as `{"type": "object", "nullable": true, "enum": [null]}`, since
OpenAPI 3.0.3 needs a type beside `nullable`. An empty description documents
the HTTP status text. Only the schema is written, never an example: a
zero-value sample is not a body the route sends. Add a real example with
`DocSuccessExample` (below).

The mode is read when the option is built: an API from `api.New` uses its
own, and the default instance (a nil `*api.API`) reads `CurrentResponseMode`.
As with every response option, a later option for the same status replaces an
earlier one. See [ADR 0004](adr/0004-instance-doc-options.md).

### Examples from a sample

`(*api.API).DocSuccessExample(status, sample, headers...)` documents the same
response as `DocSuccess`, with the given response headers, and adds an
example built by the instance's own response formatter:

```go
api.Post(router, "/tasks", createTask,
	instance.DocSuccessExample(http.StatusCreated, TaskResponse{ID: 42, Title: "Write docs"},
		doc.ResponseHeader("Location", doc.String, "URL of the created task")),
	instance.DocError(http.StatusBadRequest, "Invalid body"),
)
```

On an envelope instance the 201 example is
`{"content":{"id":42,"title":"Write docs"},"header":{"code":"0000","type":"success"}}`
(`ExampleAPI_DocSuccessExample`). The formatter receives the success that
`Success.Write` would send for `sample`: the default title and message,
`ResponseCodes.Success`, and no event ID or token, since there is no request.
The example therefore has no event ID or token, and no trace suffix in a
message, while a route under `api.ProcessRequest` sends an event ID; everything
else matches the wire. So the example shows what the wire shows in each mode: the blank title and
message omitted from the envelope, the nine plain keys with empty strings and
`info: null`, the content alone in none and problem modes, and the code as
the formatter writes it (lowercased with `api.WithLowercaseCodes`).

- The sample is the example: pass real values. A zero value is never turned
  into an example on its own; that is what `DocSuccess` is for.
- The example is built when `DocSuccessExample` is called, with the formatter
  of an API from `api.New` (a custom `api.WithResponseFormatter` included) or,
  on the default instance, the registered one. Values the formatter computes,
  such as timestamps, are frozen at that moment.
- A Content-Type the formatter sets in its headers becomes the documented
  media type.
- No example is written for a status without a body (1xx, 204, 304), a
  formatter that returns nil, a body that cannot be encoded (a NaN), or a JSON
  `null` body (a nil sample in none and problem modes).
- Numbers are kept exact: the body is encoded as the writer encodes it and
  decoded with `UseNumber`.
- Moving a route from `doc.StatusWithExample(status, envelope.Success[T](), ...)`
  (or `envelope.PlainSuccess[T]()`) to `DocSuccessExample` replaces the named
  `ResponseOf…`/`PlainResponseOf…` component with an inline schema, so
  generated clients lose that type name.

### Errors from codes

`(*api.API).DocErrors(errorCodes ...codes.Coder)` documents the error
responses `api.WriteError` sends for `jgolang/errors` codes, so the statuses
and bodies a client sees come from the same codes the handler writes:

```go
var (
	errTaskNotFound  = codes.New("TSK-404", "task not found").WithKind(codes.KindNotFound)
	errTaskLocked    = codes.New("TSK-409", "task is locked").WithKind(codes.KindConflict)
	errTaskStoreDown = codes.New("TSK-500", "task store is unavailable") // no kind: 500
)

api.Delete(router, "/projects/{projectID}/tasks/{taskID}", deleteTask,
	instance.DocErrors(errTaskNotFound, errTaskLocked, errTaskStoreDown),
)
```

The spec gets a 404, a 409 and a 500 response, in ascending order, each with
the schema `DocError` documents in the instance's mode, the HTTP status text
as description, and one named example per code. On an envelope instance the
404 is:

```json
"examples": {
  "TSK-404": {
    "summary": "task not found",
    "value": {"header": {"code": "TSK-404", "message": "task not found (-TSK-404)", "title": "Error response!", "type": "error"}}
  }
}
```

- **Status.** A code goes to the status the instance gives its kind: its
  `api.WithKindStatus` override, else `api.StatusForKind`; 500 for a code
  without a kind. It is the function `WriteError` uses, so an override moves
  the documentation and the wire together. Codes of one status share one
  response with one example each.
- **Example.** The name is the code the response carries: the code's
  `Str()`, or the default code `WriteError` uses when `Str()` is empty
  (`ResponseCodes.DefaultError`, or the internal server error code for a
  500). The summary is its `Msg()` (none
  when empty), and the value the body the instance's formatter builds for the
  `Error` that `WriteError` writes for an error with that code (same title,
  message, code and status rules), with no event ID or token since there is
  no request. In problem mode it is problem details under
  `application/problem+json`. In none mode every error body is JSON `null`,
  so, as with `DocSuccessExample`, no example is written: the response keeps
  its null-body schema only.
- **Edge cases.** A `nil` code, a nil pointer such as `(*codes.Code)(nil)`
  included, documents an uncoded error: a 500 named by the internal server
  error code. A code whose name was already documented is skipped (first
  wins). With no codes the option documents nothing. A
  formatter that returns nil, or a body that cannot be encoded, leaves that
  code without an example and its response with the schema.
- **When.** Like `DocSuccessExample`, the examples are built when `DocErrors`
  is called: with the mode, overrides and formatter of an API from `api.New`,
  or, on the default instance (a nil `*api.API`), with `CurrentResponseMode`,
  `StatusForKind` and the registered formatter. The writer and the
  `api.WithErrorHook` are never called.
- A later response option for the same status replaces the `DocErrors`
  response, as with every response option.

### Library rejections

`(*api.API).DocRejections()` documents the error responses the library's
middlewares send on their own, which no handler writes, so a client sees them
in the spec instead of first meeting them in production:

```go
api.Post(router, "/tasks", api.ProcessRequest(api.BasicToken(createTask)),
	instance.DocRejections(),
)
```

| Status | Example | Sent by |
|--------|---------|---------|
| 400 | `invalidRequestContent` | `ProcessRequest` or `RequestBody`, for a body it cannot parse |
| 401 | `invalidAuthorizationHeader` | `BasicToken` or `CustomToken`, for a missing header or another scheme |
| 401 | `invalidBasicToken` | `BasicToken`, for rejected credentials |
| 401 | `invalidBearerToken` | `CustomToken`, for a rejected token |
| 401 | `invalidSessionID` | `RequestHeaderSession`, for a missing session ID |
| 413 | `requestContentTooLarge` | `ProcessRequest` or `RequestBody`, for a body over the limit |
| 500 | `internalServerError` | `ProcessRequest`, for a handler that panics before writing |

The four responses are added in ascending order, each with the schema
`DocError` documents in the instance's mode and the HTTP status text as
description. Each example's summary says what triggers it, and its value is
the body the instance's formatter builds for the same `Error` value the
middleware writes (one constructor serves both), with no event ID or token
since there is no request. The names are descriptions, not codes, because
the four 401 bodies share one code. As with `DocErrors`, the examples are
built when `DocRejections` is called, with the mode and formatter of an API
from `api.New` or, on the default instance (a nil `*api.API`), with
`CurrentResponseMode` and the registered formatter; the message variables
(`DefaultInvalidAuthHeaderMsg`, `DefaultBasicUnauthorizedMsg`,
`DefaultBearerUnauthorizedMsg`) are read at that moment. Problem mode
documents them under `application/problem+json`; none mode keeps the
null-body schema without examples.

### Limits

- **One mode per route, from the instance it calls.** A route documents the mode of the
  instance whose `DocSuccess`/`DocError` it called, not the mode of the
  instance that serves it. When several instances with different modes serve
  routes of one `doc.Docs`, the spec holds several formats, one per route, so
  each route must use the methods of the instance that serves it.
- **Built-in formatter schemas.** The schemas describe what the built-in
  formatter of each mode sends. A custom formatter set with
  `api.WithResponseFormatter` is not described; document a body of another
  shape with the `doc` options (`doc.Status`, `doc.ResponseSchema`,
  `doc.StatusWithExample`). Only the examples of `DocSuccessExample`,
  `DocErrors` and `DocRejections` come from the custom formatter, so a formatter that changes the
  shape documents an example its schema rejects, which an OpenAPI validator
  with example validation reports.
- **Library rejections are all or nothing.** `DocRejections` adds all four
  statuses whether or not the route uses the middleware that sends each one,
  and there is no per-route selection; a later response option for the same
  status replaces its response (for example `DocErrors` with the route's own
  400 codes). The examples have no event ID, which the wire bodies carry.
  The 415 of `RequestHeaderJSON` and headers such as `WWW-Authenticate` are
  not documented.
- **No per-code problem type.** `DocErrors` documents the `about:blank`
  problem type `WriteError` sends; there is no example URI or problem `type`
  per code.

## API metadata

`doc.API` fills the document's `info` object, and `docs.SetServers` its top-level `servers`:

```go
docs := doc.New(doc.API{
	Title:       "Tasks API",
	Version:     "1.0.0",
	Description: "Manages tasks.",
	Contact:     &doc.Contact{Name: "Tasks team", Email: "tasks@example.com"},
	License:     &doc.License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
})
docs.SetServers(
	doc.Server{URL: "https://api.example.com", Description: "Production"},
	doc.Server{URL: "http://localhost:8080", Description: "Local"},
)
```

`docs.SetServers` gives Swagger UI's "Try it out" and generated clients a base URL; without it they call the host that served the document. Each call replaces the previous list, and a server with an empty URL is ignored. `Contact` and `License` are optional and written to `info.contact` and `info.license`. A `doc.API` without them generates the same document as before.

## Route metadata options

- `doc.Summary("Create task")`
- `doc.OperationID("createTask")`
- `doc.Description("Creates a new task")`
- `doc.Tags("tasks")`
- `doc.Body(envelope.Request[CreateTaskRequest]())`
- `doc.BodyWithExample(envelope.Request[CreateTaskRequest](), map[string]any{"content": map[string]any{"title": "Buy milk"}})`
- `doc.BodyExamples(envelope.Request[CreateTaskRequest](), map[string]any{"minimal": doc.ExampleObject{Summary: "Only the title", Value: minimalExample}, "urgent": urgentExample})` (OpenAPI `examples` map; replaces the `doc.BodyWithExample` example)
- `doc.BodySchema(&doc.Schema{Type: "object"})`
- `doc.Status(200, envelope.Success[TaskResponse]())`
- `doc.StatusWithExample(200, envelope.Success[TaskResponse](), map[string]any{"content": map[string]any{"id": 1}})`
- `doc.StatusWithHeaders(201, envelope.Success[TaskResponse](), doc.ResponseHeader("Location", doc.String, "Created resource URL"))`
- `doc.ResponseSchema(400, "Bad Request", &doc.Schema{Type: "object"})`
- `doc.ResponseExamples(409, "Transition not allowed or system record", ErrorBody{}, map[string]any{"transition": transitionExample, "system": systemExample})` (OpenAPI `examples` map; a `doc.ExampleObject` value keeps its summary; one option per status, a later one for the same status replaces it)
- `doc.ResponseRef(404, "TaskNotFound")` (writes `{"$ref": "#/components/responses/TaskNotFound"}`; see [Component responses](#component-responses))
- `doc.Query("page", doc.Int, false)`
- `doc.QueryWithDescription("page", doc.Int, false, "Page number")`
- `doc.QueryEnum("in_inbox", doc.Bool, false, "Only true is accepted.", true)` (writes `schema.enum`; values match the type)
- `doc.Header("X-Request-ID", doc.String, false)`
- `doc.HeaderWithDescription("X-Request-ID", doc.String, false, "Trace request ID")`
- `doc.Path("id", doc.Int, true)`
- `doc.PathWithDescription("id", doc.Int, true, "Resource ID")`
- `doc.PathEnum("status", doc.String, true, "Task status.", "open", "done")`
- `doc.Public()` (writes `security: []`; overrides `doc.Security` and the default)
- `doc.Hidden()` (keeps the route out of the spec, e.g. `/openapi.json` and `/docs`; `docs.Routes()` still returns it with `Hidden` set)
- `doc.Security("bearerAuth")`

### Component responses

A response that many routes share (a 404, a 401, an error built by an instance) can be written once under `components.responses` and referenced from each route, so an edit is made in one place:

```go
docs.AddResponse("TaskNotFound", a.DocError(http.StatusNotFound, "Task not found"))

api.Get(router, "/tasks/{id}", getTask, doc.ResponseRef(http.StatusNotFound, "TaskNotFound"))
api.Delete(router, "/tasks/{id}", deleteTask, doc.ResponseRef(http.StatusNotFound, "TaskNotFound"))
```

Each route's `404` is written as `{"$ref": "#/components/responses/TaskNotFound"}`, and the component is written like an operation's response: description, headers, schema (a body's Go type goes to `components.schemas`), and examples.

- `docs.AddResponse(name, option)` takes any `doc.RouteOption` that adds exactly one response: `doc.ResponseSchema`, `doc.ResponseExamples`, `doc.Status`, `a.DocError`, or `a.DocErrors` with codes of one status. An empty name, a nil option, and an option that adds no response or several are ignored. Adding a name again replaces it. The stored response is a copy.
- The option's status is not the status the response is documented under: `doc.ResponseRef(status, name)` chooses that on each route, so one component can be referenced as a `404` on one route and a `410` on another. The option's status only shapes the component as the option builds it: its default description is that status's text, and `a.DocError` writes no content for a status without a body. An empty name adds nothing, and as with every response option, a later option for the same status replaces it.
- References are not checked or inlined: a name that was never added is written as is, and the spec then has a dangling `$ref` that validators reject.
- `components.responses` is written only when a response is added, so a spec that does not use these names is unchanged. `doc.RouteResponse.Ref` carries the name in Go, and `doc.ResponseObject.Ref` the written reference; a `ResponseObject` with `Ref` marshals as `$ref` alone.

### Media types

Request bodies and responses are written under `application/json` unless the route says otherwise. `doc.Route.BodyContentType` sets the request body's media type and `doc.RouteResponse.ContentType` sets a response's; an empty value keeps `application/json`, and the string is written as given. There is no option function for them: set them with a `doc.RouteOption` closure, `func(*doc.Route)`.

```go
binary := &doc.Schema{Type: "string", Format: "binary"}

// Multipart upload: a form with a required binary "file" field.
api.Post(router, "/files", upload,
	doc.BodySchema(&doc.Schema{
		Type:       "object",
		Properties: map[string]*doc.Schema{"file": binary},
		Required:   []string{"file"},
	}),
	func(route *doc.Route) { route.BodyContentType = "multipart/form-data" },
	doc.Status(201, nil),
)

// Binary download.
api.Get(router, "/files/{id}", download,
	doc.Path("id", doc.String, true),
	func(route *doc.Route) {
		route.Responses = append(route.Responses, doc.RouteResponse{
			Status:      200,
			ContentType: "application/octet-stream", // or "*/*"
			Schema:      binary,
		})
	},
)
```

The same closure documents an `application/problem+json` error: append a `doc.RouteResponse` with that `ContentType` and the problem body or schema. A response with a content type but no body or schema has no `content`, as before. `Example` and `Examples` are written under the chosen media type.

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
- `validate:"required"` marks a field as required even when it has `omitempty`. No other `validate` rule is read.

Constraint tags write the matching OpenAPI keyword. They document the contract only: the library does not validate requests against them.

```go
type Capture struct {
	TargetType string  `json:"target_type" enum:"goal,project,task,note"`
	Priority   int     `json:"priority" enum:"1,2,3" default:"2"`
	Title      string  `json:"title" minLength:"1" maxLength:"80" pattern:"^[^<>]+$"`
	Estimate   float64 `json:"estimate" minimum:"0" maximum:"99.5"`
	LegacyID   string  `json:"legacy_id,omitempty" deprecated:"true"`
}
// target_type: {"type":"string","enum":["goal","project","task","note"]}
// priority:    {"type":"integer","format":"int32","enum":[1,2,3],"default":2}
```

- `enum:"a,b,c"` lists the allowed values, split on `,` with spaces trimmed. Only scalar fields (bool, integer, float, string, `time.Time` as a string, or a pointer to one) take it; each value is typed like the field, so `enum:"1,2,3"` on an `int` writes JSON numbers and a value out of the field's range (`enum:"300"` on an `int8`) is rejected. A `time.Time` value must be RFC 3339. On a pointer field, which is nullable, `null` is added to the list, because OpenAPI 3.0.3 validators check `enum` even for `null`.
- `default:"..."` sets the default value. A scalar field's default is typed like the field (`default:"false"` and `default:"0"` are written); a struct, map, slice or interface field's default is read as JSON (`default:"{\"a\":1}"`).
- `minimum:"0"` and `maximum:"99.5"` set number limits.
- `minLength:"1"` and `maxLength:"80"` set string lengths; they must be non-negative integers.
- `pattern:"^[a-z]+$"` sets a regular expression; it must compile with Go's `regexp`.
- `deprecated:"true"` marks the field deprecated. On a field whose schema is a `$ref`, the keyword sits in an `allOf` wrapper, because OpenAPI 3.0.3 ignores keys beside `$ref`.

A tag whose value does not parse for its field, such as `enum:"1,two"` on an `int`, a `NaN` limit or a pattern that does not compile, is left out of the schema rather than guessed; the library never panics or logs for it. `doc.CheckSchemaTags(docs)` walks the same types as `doc.GenerateOpenAPI` and returns every such tag as a `codes.CfgInvalid` error naming the Go type, field, tag and value (several are joined with `errors.Join`), so a service can fail its startup or a test on a typo:

```go
if err := doc.CheckSchemaTags(docs); err != nil {
	return err
}
```

`CheckSchemaTags` covers only types given as sample values (`doc.Body`, `doc.Status` and similar options); a ready-made `*doc.Schema` passed to `doc.BodySchema` or `doc.ResponseSchema`, such as one built with `doc.SchemaFromType`, is not walked. `doc.SchemaFromType` reads the same tags and leaves a bad value out silently.

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
