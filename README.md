# jgolang/api

`github.com/jgolang/api` is a small HTTP request/response micro-framework on top of `net/http`. It gives every endpoint the same response shape, typed request helpers, error responses driven by [`jgolang/errors`](https://github.com/jgolang/errors) codes, event IDs for tracing, and an OpenAPI 3 document built from the routes you register.

```sh
go get github.com/jgolang/api
```

The JSON envelope with device metadata in the request `header` (`device_type`, `device_brand`, `lat`, `lon`, `token`, …) is still supported, but it is no longer the default. It is documented in [docs/legacy.md](docs/legacy.md).

## Quick start

```go
package main

import (
	"encoding/json"
	"net/http"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/api/stdadapter"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

type CreateTaskRequest struct {
	Title string `json:"title"`
}

type TaskResponse struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

var CodeTitleRequired = codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)

func createTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error400().Write(w, r)
		return
	}
	if req.Title == "" {
		api.WriteError(w, r, errors.NewC(CodeTitleRequired, "empty title")) // 400
		return
	}
	task := TaskResponse{ID: 1, Title: req.Title}
	success := api.SuccessWithContent(task)
	success.HTTPStatusCode = http.StatusCreated
	success.Write(w, r)
}

func main() {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)

	api.Post(router, "/tasks", api.ProcessRequest(createTask),
		doc.Summary("Create task"),
		doc.Tags("tasks"),
		doc.Body(CreateTaskRequest{}),
		doc.Status(201, envelope.Success[TaskResponse]()),
	)

	router.Handle("GET", "/openapi.json", doc.OpenAPIHandler(docs))
	router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json"))

	http.ListenAndServe(":8080", router)
}
```

With the defaults, `POST /tasks` takes a plain JSON body (`{"title":"Buy milk"}`) and answers with the response envelope:

```json
{
    "header": {
        "type": "success",
        "code": "0000",
        "event_id": "44767dd580599c70c7516af431772e8c"
    },
    "content": {"id": 1, "title": "Buy milk"}
}
```

## The request flow

1. **Route registration.** `api.Get`, `api.Post`, `api.Put`, `api.Patch` and `api.Delete` register a handler on any `api.Router` and record its `doc.RouteOption`s for the OpenAPI document. `stdadapter` is the `net/http` router; see [Routing and OpenAPI](#routing-and-openapi).
2. **`api.ProcessRequest`.** Wrap a handler with this middleware to get the full flow. It:
   - reads the body and parses it according to the [request mode](#request-and-response-modes); an invalid JSON body answers 400 with code `e002`;
   - collects request metadata from HTTP headers (see [Reading the request](#reading-the-request));
   - assigns an [event ID](#trace-ids) and logs the request and the response;
   - stores the request context, replaces `r.Body` with the endpoint content, and answers 500 if the handler panics.

   Use `api.MiddlewaresChain(...)` to compose it with your own middlewares. `api.AddSkipPath(api.GenSkipPath(method, path), true)` makes it pass a route through untouched.
3. **The handler** reads the content, then writes one [response](#responses) or [error](#writing-errors).
4. **The formatter** shapes the response according to the [response mode](#request-and-response-modes) and adds the event ID, code and token.

A handler that is not wrapped by `api.ProcessRequest` still works: it reads the raw body itself, and its responses are formatted the same way, without a generated event ID.

## Request and response modes

Two modes control the wire format. Set them on an instance with `api.New` and wrap the router with its `Handler`:

```go
a := api.New(
	api.WithRequestMode(api.RequestModeAuto),
	api.WithResponseMode(api.ResponseModePlain),
)
http.ListenAndServe(":8080", a.Handler(router))
```

`a.Handler` stores the instance in the request context, so `api.ProcessRequest`, `api.RequestBody`, the token middlewares, `Write(w, r)` on every response type (`api.Success`, `api.Error`, `api.Warning`, `api.Informative`) and `api.WriteError` use its modes; handler code does not change. Wrap the outermost chain: middlewares outside `a.Handler` (and the package-level `api.Write(data, w)`, which takes no request) use the default instance; with nested instances the innermost one wins. An instance is immutable after `api.New`, and two instances in one process (two services, or two tests) do not see each other's modes. `api.New()` with no options parses plain requests and writes envelope responses. An instance sends response codes as declared (`PLN-001`); `api.WithLowercaseCodes()` restores lowercasing, and the default instance lowercases as in v0. An instance uses the built-in request receiver, response formatter and response writer unless you pass your own:

```go
a := api.New(
	api.WithRequestReceiver(myReceiver),    // core.APIRequestReceiver
	api.WithResponseFormatter(myFormatter), // core.APIResponseFormatter
	api.WithResponseWriter(myWriter),       // core.APIResponseWriter
)
```

The mode options, and for the formatter `api.WithLowercaseCodes()`, apply to the built-in receivers (`api.RequestReceiver`, `api.RequestReceiverV2`) and formatter (`api.ResponseFormatter`), also when you pass them with these options; a custom receiver or formatter (including one that embeds a built-in) alone decides how it parses, what body it writes and its code text. A type that embeds a built-in receiver or formatter is custom, and its promoted methods read the deprecated package mode variables. A nil value keeps the built-in. The deprecated `api.RegisterNewAPIRequestReceiver`, `api.RegisterNewAPIResponseFormatter` and `api.RegisterNewAPIResponseWriter` affect only the default instance. `api.RegisterNewAPISecurityGuarantor` and `api.RegisterParamValidator` are not deprecated and apply to every instance.

| Request mode | Body parsing |
| --- | --- |
| `api.RequestModePlain` (default) | The body is the endpoint content, as plain JSON. |
| `api.RequestModeEnvelope` | The body is `{"header": {...}, "content": {...}}`; the content is `content`, and the `header` fields fill request metadata that the HTTP headers did not set ([legacy envelope](docs/legacy.md)). |
| `api.RequestModeAuto` | Envelope when the body is a JSON object with one or two keys, one of them `header` or `content`; plain otherwise. |
| `api.RequestModeNone` | The body is not unwrapped; it is read as plain JSON. |

| Response mode | Body written |
| --- | --- |
| `api.ResponseModeEnvelope` (default) | `{"header": {"type", "title", "message", "code", "token", "action", "event_id", "info"}, "content": ...}`; empty header fields are omitted, except `type`. |
| `api.ResponseModePlain` | The same fields at the top level, with `content` next to them: `{"type": "success", "code": "0000", ..., "content": ...}`. Every field is present. |
| `api.ResponseModeNone` | Only the content, as is. |

An unknown value falls back to the default (`api.RequestModePlain`, `api.ResponseModeEnvelope`).

Requests that are not wrapped by an instance's `Handler` use the default instance, which reads the package variables `api.CurrentRequestMode` and `api.CurrentResponseMode` (set with `api.SetRequestMode` and `api.SetResponseMode`). These four names are deprecated: they affect only the default instance, for the whole process. The `envelope` package types are for OpenAPI documentation and match the envelope response mode.

## Reading the request

After `api.ProcessRequest`, `r.Body` holds only the endpoint content in every request mode, so a handler decodes it as usual:

```go
var req CreateTaskRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	api.Error400().Write(w, r)
	return
}
```

The request context carries the content and its metadata:

```go
rctx := api.Context(r.Context())
err := rctx.DecodeContent(&req) // same content as r.Body
eventID := rctx.EventID
token := rctx.SecurityToken
```

Request metadata comes from HTTP headers: `X-Request-ID` (else `UUID`) for the request UUID, `SecurityToken` (else `Authorization: Bearer <token>`) for the token, `AppLanguage` (else `Accept-Language`) for the language, and `DeviceType`, `DeviceBrand`, `DeviceModel`, `DeviceOS`, `OSVersion`, `OSTimezone`, `AppVersion`, `AppBuildVersion`, `AppName`, `DeviceSerial`, `DeviceID`, `Latitude`, `Longitude` for the client. In envelope request mode the JSON `header` fills the same fields when the HTTP header is absent.

Route variables are read with `api.GetRouteVar("id", r)`. `stdadapter` matches `{id}` segments itself and stores them with `api.SetRouteVars`, so `r.PathValue` is empty there; an adapter for another router either calls `api.SetRouteVars` or replaces `api.GetRouteVar`.

## Query parameters

`api.QueryInt`, `api.QueryBool`, `api.QueryEnum` and `api.QueryString` read the first value of a query parameter and tell absent from invalid. `ok` is false when the parameter is absent. An invalid value, an empty one for the typed helpers, or a malformed query string that hides the parameter returns `ok` true and a validation error (`codes.ValFormat`), which `api.WriteError` answers with 400. `api.WithQueryCode` sets your own code; give it `codes.KindValidation` to keep the 400.

```go
var CodeLimitInvalid = codes.New("PLN-050", "limit must be a number").WithKind(codes.KindValidation)

limit, ok, err := api.QueryInt(r, "limit", api.WithQueryCode(CodeLimitInvalid))
if err != nil {
	api.WriteError(w, r, err) // 400
	return
}
if !ok {
	limit = 20
}
```

The older `api.GetQueryParamValue*` helpers return an `api.Response`; prefer the `Query*` helpers in new code. `ExampleQueryInt` and `ExampleQueryEnum` in `example_query_test.go` are the compiled versions.

## Responses

Every response kind (`api.Success`, `api.Error`, `api.Warning`, `api.Informative`) is a `core.ResponseData` with a `Write(w, r)` method. Its fields (`Title`, `Message`, `HTTPStatusCode`, `ResponseCode`, `Content`, `Headers`, `Info`, `Actions`) can be set before writing. `Write` fills the event ID and the request token, and defaults the title, message, status and code of its kind.

| Helper | Status | Code |
| --- | --- | --- |
| `api.Success200()`, `api.SuccessWithMsg(msg)` | 200 | `0000` |
| `api.SuccessWithContent(content)` | 200 | `0000` |
| `api.Success201()` | 201 | `0000` |
| `api.Error400(errs...)`, `api.ErrorWithMsg(msg, errs...)` | 400 | the first coded error in `errs`, else `e001` |
| `api.Error401()`, `api.Error403()` | 401, 403 | `b001`, `b002` |
| `api.Error500(errs...)`, `api.Error500WithMsg(msg, errs...)` | 500 | the first coded error in `errs`, else `e300` |
| `api.Error501(errs...)` | 501 (503 before v0.10.0) | the first coded error in `errs`, else `e001` |

The default instance writes response codes in lower case, as in the table (`e001`); an `api.New` instance sends them as declared (`E001`) unless created with `api.WithLowercaseCodes()`. The default titles, messages and codes are package variables (`api.DefaultSuccessTitle`, `api.InternalServerMessage`, `api.ResponseCodes`, …).

```go
api.SuccessWithContent(task).Write(w, r)

created := api.SuccessWithContent(task)
created.HTTPStatusCode = http.StatusCreated
created.Write(w, r)

api.Error500(err).Write(w, r)
```

Two settings shape the `title` and `message` of every response:

- `api.BlankSuccess` (default `true`): success responses leave `title` and `message` empty. Error, warning, informative and custom response types always carry them.
- `api.TraceVisibility` (default `1`): a trace suffix appended to a non-empty message. `1` appends ` (<event id>-<code>)`, `2` only ` (<code>)`, `3` only ` (<event id>)`; any other value appends nothing. The event ID is cut to `api.TraceIDLength` characters (default 6). For example, `Error500()` writes `"An error has occurred please try again later. (cdf606-e300)"`.

## Writing errors

`api.WriteError(w, r, err)` answers with the status of the error's code kind (`jgolang/errors` v0.4.0+): validation 400, unauthorized 401, forbidden 403, not_found 404, conflict 409, and 500 for infrastructure or a code without a kind. The response code and message come from the same code, also behind `fmt.Errorf`. `api.StatusForKind` exposes the mapping.

```go
var ErrTaskNotFound = errors.Define(codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound), "task not found")

func getTask(w http.ResponseWriter, r *http.Request) {
	task, err := tasks.Find(api.GetRouteVar("id", r)) // your code; returns errors.Wrap(ErrTaskNotFound)
	if err != nil {
		api.WriteError(w, r, err) // 404
		return
	}
	api.SuccessWithContent(task).Write(w, r)
}
```

401 and 403 use the unauthorized title, like `Error401`/`Error403`; a nil error answers 500. `ExampleWriteError` in `example_write_error_test.go` is the compiled version.

## Routing and OpenAPI

The package includes a router-agnostic registration layer for OpenAPI 3 and Swagger UI. The native adapter uses only Go's standard `net/http` package, while adapters for Gin, Echo, chi, gorilla/mux, or other routers should live in the application that chooses those routers (`api.RegisterAdapter` and `api.NewRouter` register and build them by name).

The philosophy is to keep `github.com/jgolang/api` small and predictable: prefer Go's standard library and `github.com/jgolang/...` dependencies, with any exception made explicitly.

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
	// Optional: require bearerAuth everywhere and mark exceptions with doc.Public().
	// docs.SetDefaultSecurity("bearerAuth")

	router := stdadapter.New(http.NewServeMux(), docs)

	api.Post(router, "/tasks", api.ProcessRequest(createTask),
		doc.Summary("Create task"),
		doc.Tags("tasks"),
		doc.Body(CreateTaskRequest{}),
		doc.Security("bearerAuth"),
		doc.Status(201, envelope.Success[TaskResponse]()),
	)

	api.Get(router, "/tasks", api.ProcessRequest(listTasks),
		doc.Summary("List tasks"),
		doc.Tags("tasks"),
		doc.Query("limit", doc.Int, false),
		doc.Status(200, envelope.Success[[]TaskResponse]()),
	)

	router.Handle("GET", "/openapi.json", doc.OpenAPIHandler(docs))
	router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json"))

	http.ListenAndServe(":8080", router)
}
```

`GET /openapi.json` serves the OpenAPI 3 document and `GET /docs` serves Swagger UI pointed at it. Document the body in the shape the request mode expects: the content type itself in plain mode (`doc.Body(CreateTaskRequest{})`), or `doc.Body(envelope.Request[CreateTaskRequest]())` in envelope mode. `envelope.Success[T]()` and `envelope.Error()` document the envelope response mode.

See [docs/openapi.md](docs/openapi.md) for security schemes, schema tags, response aliases, all route options, and external adapter examples.

## Security notes

- `doc.Security` and `docs.SetDefaultSecurity` only document authentication; they do not enforce it. Enforce it with a middleware: `api.CustomToken` (validated by `api.CustomTokenValidatorFunc`), `api.BasicToken` (validated by `api.ValidateBasicAuthCredentialsFunc`), or your own.
- The request token (`SecurityToken` header, `Authorization: Bearer`, or the envelope `header.token`) is echoed in the response `token` field unless the handler sets `SecurityToken` on the response. Do not rely on the response body being free of it.
- Request and response bodies are logged only when `api.LogRequestBody` / `api.LogResponseBody` are true; values under the keys in `api.SensitiveLogKeys` are masked and bodies are cut at `api.MaxLoggedBodyBytes`.

## Trace IDs

`api.ProcessRequest` gives every request an event ID. The package does not depend on a tracing vendor: register a provider with `api.RegisterTraceIDProvider(...)` and its trace ID becomes the event ID; without one, the event ID is a hash of the request UUID (or client IP), the time and the URI. See [docs/tracing.md](docs/tracing.md).

The event ID is stored in the request context (`api.Context(r.Context()).EventID`) and in the `EventID` request header (`api.EventIDHeaderKey`), written in the response `event_id` field, used as the log correlation ID, and appended to messages according to `api.TraceVisibility`.

## Legacy envelope

The device-metadata request envelope, the v1 and v2 request receivers, the encrypted body flow and the `api.Response`-returning helpers are documented in [docs/legacy.md](docs/legacy.md).

## Contributing

If you have suggestions for how We could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Contributing Guide](CONTRIBUTING.md).

## License

This project is licensed under the [MIT License](LICENSE).

## Support

If you find this repository helpful and would like to support its development, consider making a donation. Your contributions will help ensure the continued improvement and maintenance of this repository.

Thank you for your support!

[![ko-fi](https://www.ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/josuegiron)
