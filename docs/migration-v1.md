# Migrating to v1

This guide takes a v0 program (v0.11.x) to `github.com/jgolang/api` v1. The design is recorded in [ADR 0001](adr/0001-v1-instance-api.md) (the instance) and [ADR 0002](adr/0002-stdadapter-servemux.md) (routing; it lands with PR #38).

## Summary

- The module path does not change. When v1.0.0 is tagged, upgrade with `go get github.com/jgolang/api@v1.0.0`; no import changes.
- v1 removes nothing. Every v0 name still compiles. Names replaced by an instance option are deprecated, and each one points at its option.
- A program that does not change its code sends the same bodies as v0. Two things change for it:
  - `stdadapter` routes through Go 1.22 `ServeMux` patterns ([step 2](#step-2-fix-routing-stdadapter)). This is the change most programs must act on.
  - A non-empty `Content-Type` set in a response's `Headers` is now sent. v0 overwrote it with `application/json`. This applies to the default instance too. It is a fix, not a breaking change: v0 discarded a `Content-Type` the service set. An empty value counts as unset, and a `Content-Type` the handler set on `w.Header()` is still overwritten, as in v0.
- Configuration moves from package variables to an instance: `a := api.New(opts...)` and `a.Handler(router)` ([step 3](#step-3-move-to-apinew)). Handler code does not change.

Three steps, each one shippable on its own:

1. [Upgrade without code changes.](#step-1-upgrade-without-code-changes)
2. [Fix routing (`stdadapter`).](#step-2-fix-routing-stdadapter)
3. [Move to `api.New`.](#step-3-move-to-apinew)

## Step 1: upgrade without code changes

Requests that no `a.Handler` wraps use the default instance. It keeps v0's behavior:

- Response codes are lowercased (`e001`, `tsk-001`).
- The package variables are read at call time: `CurrentRequestMode`, `CurrentResponseMode`, `TraceVisibility`, `TraceIDLength`, `BlankSuccess` and the body-logging variables.
- The components set with `RegisterNewAPIRequestReceiver`, `RegisterNewAPIResponseFormatter` and `RegisterNewAPIResponseWriter` are used.
- Logging goes through `Print` and `PrintError` and writes the same lines as v0.11.0. A response that cannot be encoded is answered with 500, as in v0.11.0; `Fatal` is never called. That 500 now drops the failed body's `Content-Type`, `Content-Encoding`, `Content-Length` and similar headers, and answers with no body instead of panicking when a registered formatter returns nil.
- `SensitiveLogKeys` keeps its v0.11.0 list (with `cookie` and `session`) and masking (headers, JSON body fields, query and form values, the user information of the `uri`).

Deprecated names are reported by staticcheck (`SA1019`) and by gopls in your editor. `go vet` does not report them. Fix the routing first (step 2); the deprecations can wait until step 3.

## Step 2: fix routing (stdadapter)

`stdadapter` registers each route as one `ServeMux` pattern (`GET /tasks/{id}`). Matching, precedence, 404 and 405 now come from `net/http`. `api.GetRouteVar` works as before, and `r.PathValue` now works too. Other adapters are not affected.

| # | Change | What to check | Fix |
|---|---|---|---|
| 1 | Precedence is "most specific wins" (`/tasks/new` beats `/tasks/{id}`). Two routes conflict when a path matches both and neither is more specific: `/tasks/{id}` and `/tasks/{name}`, or `/tasks/{id}/notes` and `/tasks/new/{kind}`. | Duplicate routes and overlapping wildcards. They now panic at start-up. | Remove the duplicate, or make one route more specific. |
| 2 | A trailing slash no longer matches a route without one: `/tasks/42/` is 404 for `/tasks/{id}`. A registered `/tasks/` still matches only `/tasks/` (unchanged). | Clients that send a trailing slash. | Register the trailing-slash path too, if clients send it. |
| 3 | An escaped `%2F` matches a wildcard: `/files/a%2Fb` gives `name` = `a/b`, and `/files/..%2F..%2Fetc` gives `../../etc`. v0.11.0 answered 404. | Route variables used in file paths or other trusted lookups. | Validate the value first (see the security note below). |
| 4 | Wildcard names must be Go identifiers and whole segments. `{name}` is the only wildcard syntax. The method must be an upper-case token. | `{user-id}`, `{name}.json`, `{name...}`, `{$}`, and methods such as `get` or `""`. They now panic at registration. | `{user_id}`; split `{name}.json` into whole segments; drop `{name...}` and `{$}`; use `GET`. |
| 5 | 405 is decided by the whole mux and carries an `Allow` header (`GET, HEAD, PUT`). A catch-all `/` on your mux takes these requests instead. | Clients or tests that expect a 405 body or no `Allow` header; a catch-all `/` route. | Expect `Allow` on 405; with a catch-all `/`, expect its answer. |
| 6 | A GET route also answers HEAD (v0.11.0 answered 405). Method-keyed logic sees `HEAD`. | Skip paths: `GET:/health` (`api.GenSkipPath`) does not cover `HEAD /health`. | Add a `HEAD:` skip path where needed. |
| 7 | Unclean paths (`//tasks`, `/a/../tasks`) redirect to the cleaned path with 307. Unchanged from v0.11.0 (earlier notes said 301). | Nothing. | None. |
| 8 | `GODEBUG=httpmuxgo121=1` turns off pattern matching: every route breaks, and the conflict panic is off too. | The environment, `//go:debug httpmuxgo121=1` directives, and `godebug` lines in your `go.mod`. | Remove the setting. |

**Panics at registration.** `Router.Handle`, and so `api.Get`/`Post`/…, panics on:

- a conflicting route (the message names both patterns);
- an invalid method: empty, lower-case (`get`) or not an RFC 9110 token (`GE T`);
- `{name...}` or `{$}`;
- a wildcard name that is not a Go identifier (`{user-id}`), or that does not fill its segment (`{name}.json`).

A rejected route is neither served nor documented. This is a programming error in the route table, found at start-up. Run your service (or a test that builds its router) once after the upgrade to find them all.

**Security note.** A wildcard value is decoded. With v1 it can contain `/` and `..` (`%2F`), so `/files/..%2F..%2Fetc` gives `name` = `../../etc`. It can also contain CR, LF or NUL (`%0D%0A`, `%00`), as in v0. Validate a route variable before you use it in a file path, a header, a log line or another lookup that trusts it.

**OpenAPI note.** A route with a custom method (`PROPFIND`) still registers, and `doc` still emits an operation for it. OpenAPI 3.0 does not allow custom methods, so some tools reject that document. This is not new in v1.

## Step 3: move to api.New

### Wiring

Before (v0), configuration lives in package state and the router is served as is:

```go
func main() {
	api.SetRequestMode(api.RequestModePlain)
	api.SetResponseMode(api.ResponseModePlain)

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Post(router, "/tasks", api.ProcessRequest(createTask), doc.Body(CreateTaskRequest{}))

	http.ListenAndServe(":8080", router)
}
```

After (v1), one instance holds the configuration and wraps the router:

```go
func main() {
	a := api.New(
		api.WithRequestMode(api.RequestModePlain),
		api.WithResponseMode(api.ResponseModePlain),
		api.WithLowercaseCodes(), // keep v0's codes; drop it to send them as declared
	)

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Post(router, "/tasks", api.ProcessRequest(createTask), doc.Body(CreateTaskRequest{}))

	http.ListenAndServe(":8080", a.Handler(router))
}
```

`createTask` does not change. `a.Handler` stores the instance in the request context. `ProcessRequest`, `RequestBody`, the token middlewares, the `Get*Value*` helpers, `WriteError` and `Write(w, r)` on every response type find it there. Route registration (`api.Get`/`Post`/…), `*doc.Docs` and the adapters do not change.

With `WithLowercaseCodes()` the handler answers byte for byte as on the default instance. Without it, codes are sent as declared:

```json
{"action":"","code":"tsk-001","content":null,"event_id":"evt1","info":null,"message":"title is required (evt1-tsk-001)","title":"Error response!","token":"","type":"error"}
{"action":"","code":"TSK-001","content":null,"event_id":"evt1","info":null,"message":"title is required (evt1-TSK-001)","title":"Error response!","token":"","type":"error"}
```

`Example_migrationBeforeAndAfter` in `example_migration_test.go` proves it for the handler, served without `ProcessRequest`. Under `ProcessRequest` the bodies also match apart from the generated event IDs; `TestQuickStartAnswersLikeTheDefaultInstanceExceptCodeCase` in `instance_test.go` covers that.

**Wrap the outermost chain.** Middlewares outside `a.Handler` use the default instance. So does `api.Write(data, w)`, which takes no request. Put `a.Handler` around everything, or a rejection from an outer `ProcessRequest` (invalid body, `e002`) keeps v0's code case and settings. With nested instances, the innermost one wins.

### Deprecated names and their options

The names replaced by an instance option keep working, but only for the default instance. The `GetQueryParamValue*` helpers keep working everywhere, instances included.

| Deprecated (v0) | Use instead |
|---|---|
| `SetRequestMode` | `api.New(api.WithRequestMode(...))` |
| `CurrentRequestMode` | `api.New(api.WithRequestMode(...))` |
| `SetResponseMode` | `api.New(api.WithResponseMode(...))` |
| `CurrentResponseMode` | `api.New(api.WithResponseMode(...))` |
| `RegisterNewAPIRequestReceiver` | `api.New(api.WithRequestReceiver(...))` |
| `RegisterNewAPIResponseFormatter` | `api.New(api.WithResponseFormatter(...))` |
| `RegisterNewAPIResponseWriter` | `api.New(api.WithResponseWriter(...))` |
| `Print` | `api.New(api.WithLogger(...))` |
| `PrintError` | `api.New(api.WithLogger(...))` |
| `Fatal` | `api.New(api.WithLogger(...))` |
| `LogRequest` | `api.New(api.WithLogger(...))` |
| `LogResponse` | `api.New(api.WithLogger(...))` |
| `LogResponseWithDuration` | `api.New(api.WithLogger(...))` |
| `SensitiveLogKeys` | `api.New(api.WithSensitiveLogKeys(...))` |
| `GetQueryParamValueString` | `api.QueryString` (deprecated since v0.9.0) |
| `GetQueryParamValueInt` | `api.QueryInt` (deprecated since v0.9.0) |
| `GetQueryParamValueBool` | `api.QueryBool` (deprecated since v0.9.0) |

Every option in one place:

```go
a := api.New(
	api.WithRequestMode(api.RequestModePlain),          // SetRequestMode, CurrentRequestMode
	api.WithResponseMode(api.ResponseModeEnvelope),     // SetResponseMode, CurrentResponseMode
	api.WithRequestReceiver(api.RequestReceiver{}),     // RegisterNewAPIRequestReceiver
	api.WithResponseFormatter(api.ResponseFormatter{}), // RegisterNewAPIResponseFormatter
	api.WithResponseWriter(api.ResponseWriter{}),       // RegisterNewAPIResponseWriter
	api.WithLogger(logger),                             // Print, PrintError, Fatal, Log*
	api.WithSensitiveLogKeys("pin"),                    // SensitiveLogKeys
)
```

`Example_migrationOptions` is the compiled version. Pass only the options you need: `api.New()` already uses the built-in receiver, formatter and writer.

**Custom components.** The mode options, and `WithLowercaseCodes` for the formatter, apply to the built-in `RequestReceiver`, `RequestReceiverV2` and `ResponseFormatter`, also when you pass them with an option. A custom receiver or formatter decides on its own how it parses, what body it writes and its code text. A type that embeds a built-in one is custom: its promoted methods read the deprecated package mode variables and lowercase codes. A type that embeds `ResponseWriter` keeps v0 behavior and can still call `Fatal`. A nil value keeps the built-in.

### What stays package-level

These are not deprecated and are shared by every instance:

- name registries: `RegisterAdapter`/`NewRouter`, `RegisterAuth`, `RegisterCodec`, `RegisterCrypto`, `RegisterValidator`, `RegisterTraceProvider`, `doc.RegisterExporter`;
- `RegisterTraceIDProvider` and `GetRouteVar`;
- header key names: `SecurityTokenHeaderKey`, `EventIDHeaderKey`, `UserIDHeaderKey`;
- default titles and messages, `ResponseCodes` and `SkipPaths`;
- security credentials: `Username`, `Password`, `CustomTokenValidatorFunc`, `ValidateBasicAuthCredentialsFunc`;
- map methods: `AddNewMapMethod`, `ValidateMethods`;
- `RegisterNewAPISecurityGuarantor` and `RegisterParamValidator`: every instance forwards to them;
- `Write(data, w)`.

`TraceVisibility`, `TraceIDLength`, `BlankSuccess`, `PrintFullEvent`, `LogRequestBody`, `LogResponseBody` and `MaxLoggedBodyBytes` are not deprecated either, but they affect only the default instance. Instances use their v0 defaults and never log bodies.

## What changes on an instance

**Codes as declared.** An instance sends `core.ResponseCode` as declared, in the `code` field and in the message trace suffix. The library's own codes in `ResponseCodes` are declared upper-case, so `api.New()` sends `E001`, `E002`, `E300`, `B001`, … where v0 sent `e001`. Clients, alerts and dashboards that match `e001` or `tsk-001` case-sensitively must change, or keep `api.WithLowercaseCodes()`.

**Fixed formatter settings.** An instance uses `TraceVisibility = 1`, `TraceIDLength = 6` and `BlankSuccess = true`, and does not read the package variables.

**Errors.** `WithKindStatus(kind, status)` overrides the status `WriteError` sends for one kind; repeat it per kind. Only 400–599 is accepted; other statuses are ignored, and so is the empty kind (errors without a kind always answer 500). `WithErrorHook(fn)` is called by `WriteError` only, with the original error and the decided status, before anything is written. It is not called by `Error400`/`Error500` and the other helpers, nor by the library's own rejections. A panic in it is not recovered. It may run concurrently, so it must be safe for concurrent use.

```go
a := api.New(
	api.WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity),
	api.WithErrorHook(func(r *http.Request, err error, status int) {
		slog.ErrorContext(r.Context(), "request failed", "status", status, "error", err)
	}),
)
```

`Example_migrationKindStatusAndHook` is the compiled version.

**Logging.** An instance logs only through its `WithLogger` logger, as `slog` records (`request`, `response`, and error records). Without a logger it logs nothing. Compared with the default instance:

- the sensitive keys are the default `SensitiveLogKeys` list, copied when the instance is built, plus `WithSensitiveLogKeys`, which only adds; changing `SensitiveLogKeys` does not reach an instance;
- header values, query values in `uri` (`/login?access_token=[REDACTED]&page=2`) and the user information of the `uri` (`http://[REDACTED]@host/...`) are masked with the same rules as the default instance; path segments are not;
- bodies and the parsed form are never logged;
- a failed `Get*Value*` helper logs the name as `key`; for a sensitive name the `error` gives only the kind of failure, never the value;
- the built-in writer never calls `Fatal` and never exits: a body that cannot be encoded is logged and answered with 500 and the `Error500()` body, with the formatter's `Content-Type`. The failed response's headers that describe its body (`Content-Type`, `Content-Encoding`, `Content-Length`, `ETag`, …) are dropped from that 500; the others are kept.

See the README "Logging" section for every record.

**Problem details.** `api.WithResponseMode(api.ResponseModeProblem)` writes errors as RFC 9457 `application/problem+json`. That includes the library's own rejections under the instance: `ProcessRequest`'s invalid body (400, `E002`) and panic (500, `E300`), the token and header middlewares (401, 415) and `RequestBody`. Other responses are written as their content alone. Only instances accept this mode; `SetResponseMode(ResponseModeProblem)` falls back to envelope. A non-empty `Content-Type` in the response's `Headers` still wins over `application/problem+json`.

```json
{"type":"about:blank","title":"Bad Request","status":400,"detail":"title is required","code":"TSK-001","event_id":"evt1"}
```

`title` is the HTTP status phrase when the response title is a library default (`Error response!`, `Sorry!`, `Unauthorized!`, …); a title the service set is kept. `detail` has no trace suffix. Content, token, actions and info are not sent. `Example_migrationProblemDetails` is the compiled version.

## Tests

Wrap the handler under test with its own instance. Two instances in one test binary do not see each other's options, and nothing the instance owns (modes, components, code case, logger, sensitive keys, kind overrides, hook) has to be reset. The shared package-level settings (`ResponseCodes`, `SkipPaths`, credentials, the guarantor, the param validator, default titles) still need resetting if a test changes them:

```go
a := api.New(api.WithResponseMode(api.ResponseModePlain))
rec := httptest.NewRecorder()
a.Handler(http.HandlerFunc(createTask)).ServeHTTP(rec, req)
```

## Open notes

These are not decided and may change before v1.0.0:

- **Q35:** a request body limit, possibly as a `WithMaxBodyBytes` option with a 10 MiB default. Not built.
- **Q36:** whether instances stop echoing the request token in the response `token` field. Today they echo it, as v0 does (problem bodies never carry it).

## Checklist

- [ ] Upgrade and build. Nothing is removed, so the program compiles.
- [ ] Start the service once (or build its router in a test): fix every registration panic (conflicts, methods, wildcard names).
- [ ] Check trailing-slash clients, HEAD skip paths, 405 handling and `GODEBUG=httpmuxgo121`.
- [ ] Validate route variables that reach file paths, headers or logs.
- [ ] Check responses that set a `Content-Type` in `Headers`: it is now sent (one set on `w.Header()` is still overwritten).
- [ ] Create one instance with `api.New` and wrap the outermost handler with `a.Handler`.
- [ ] Replace each deprecated name with its option (staticcheck `SA1019` lists them).
- [ ] Decide on code case: keep `api.WithLowercaseCodes()`, or update clients and alerts to the declared codes.
- [ ] Give the instance a `*slog.Logger`, or accept that it logs nothing.
- [ ] Move tests to `a.Handler(h).ServeHTTP` instead of resetting package variables.
