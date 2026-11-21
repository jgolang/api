# Migrating to v1

This guide takes a v0 program (v0.11.x) to `github.com/jgolang/api` v1. The design is recorded in [ADR 0001](adr/0001-v1-instance-api.md) (the instance) and [ADR 0002](adr/0002-stdadapter-servemux.md) (routing).

## Summary

- The module path does not change. When v1.0.0 is tagged, upgrade with `go get github.com/jgolang/api@v1.0.0`; no import changes.
- v1 removes nothing. Every v0 name still compiles. Names replaced by an instance option are deprecated, and each one points at its option or says it is ignored.
- **Breaking: the default instance behaves like `api.New()`** (owner decision, 2026-11-21). A program that does not change its code still gets v1's defaults: codes as declared, no request token echo, request bodies bounded at 10 MiB, and `slog` records through `slog.Default` instead of `Print`/`PrintError` ([step 1](#step-1-upgrade-what-changes-without-code-changes)). A program that needs v0.11.0 behavior stays on v0.11.x.
- `stdadapter` routes through Go 1.22 `ServeMux` patterns ([step 2](#step-2-fix-routing-stdadapter)).
- Configuration moves from package variables to an instance: `a := api.New(opts...)` and `a.Handler(router)` ([step 3](#step-3-move-to-apinew)). Handler code does not change, and it is the only way back to v0's lowercase codes (`api.WithLowercaseCodes()`).
- Programs that use `api.BasicToken` no longer get the password in the `Basic-Secret` request header ([details](#breaking-no-basic-secret-header)).

Three steps. Step 1 is the upgrade itself; read it before you ship it.

1. [Upgrade: what changes without code changes.](#step-1-upgrade-what-changes-without-code-changes)
2. [Fix routing (`stdadapter`).](#step-2-fix-routing-stdadapter)
3. [Move to `api.New`.](#step-3-move-to-apinew)

## Step 1: upgrade: what changes without code changes

Requests that no `a.Handler` wraps use the default instance. Since v1.0.0 it answers and logs like `api.New()` with its defaults. `TestDefaultInstanceAnswersLikeNew` in `default_instance_test.go` checks that status, headers and body are the same as under `api.New().Handler`, apart from event IDs.

**Wire changes** (clients, alerts and dashboards may need to change):

- **Codes as declared.** The `code` field and the message trace suffix send `core.ResponseCode` as declared: `E001`, `E002`, `E300`, `TSK-001` where v0 sent `e001`, `e002`, `e300`, `tsk-001`. Built-in `jgolang/errors` codes that are declared lowercase (`d001`, `s001`) do not change. There is no package-level switch back; a service that must keep lowercase codes moves to `api.New(api.WithLowercaseCodes())` ([step 3](#step-3-move-to-apinew)).
- **No request token echo.** `Success`, `Error`, `Warning` and `Informative` (and so `WriteError`, the token middlewares' 401s and every library rejection) no longer copy the request token (`SecurityToken` header, `Authorization: Bearer`, the envelope `header.token`) into the response: plain bodies send `"token":""`, envelope bodies omit `header.token`. A token the handler sets (`api.Success{SecurityToken: renewed}`) is still sent.
- **`RequestHeaderSession`** no longer copies the `SecurityToken` request header into the response header. It still answers 401 without it. A handler that wants to return a token sets the header itself.
- **Request body limit.** `ProcessRequest`, `RequestBody`, `ProcessBody`, `ProcessEncryptedBody` and `UnmarshalBody` read at most 10 MiB. `ProcessRequest`, `RequestBody` and `UnmarshalBody` answer a larger body with 413, code `E002` and the title `Request content too large`; `ProcessBody` and `ProcessEncryptedBody` return the `*http.MaxBytesError`. A limit the service installed itself (`http.MaxBytesHandler`) is answered the same way (v0.11.0 answered 400 `Invalid request content`); such an outer limit can only lower the 10 MiB cap. To raise it, use `api.New(api.WithMaxBodyBytes(n)).Handler` ([step 3](#step-3-move-to-apinew)). A handler that reads `r.Body` itself is not bounded; wrap it with `http.MaxBytesHandler`, or move to `api.New` ([step 3](#step-3-move-to-apinew)), whose `Handler` bounds every reader.
- **Problem details.** `SetResponseMode(ResponseModeProblem)` is now accepted and the default instance writes RFC 9457 errors; v0 fell back to envelope.
- A non-empty `Content-Type` set in a response's `Headers` is now sent. v0 overwrote it with `application/json`. It is a fix, not a breaking change: v0 discarded a `Content-Type` the service set. An empty value counts as unset, and a `Content-Type` the handler set on `w.Header()` is still overwritten, as in v0.
- `RequestBasic.GetSecurityToken`, `GetUserID` and `GetEventID` (and so `GetRequestBasicInfo` and `GetRequestFullInfo`) return their own error: title `Request info error!` and the message `The session id was not obtained` (`user id`, `event id`). v0 meant to send it but returned the generic `Error getting header value!` / `The <key> key header has not been obtained` instead (#63).

**Logging changes:**

- `ProcessRequest` writes `slog` records through `slog.Default()`, read at call time: `request` and `response` info records, `request body rejected`, `panic recovered` (with `panic_type`; the value itself only for a `runtime.Error`, `[REDACTED]` otherwise) and the `Get*Value*` helper failures as error records, the same records an instance writes with `WithLogger` (see the README "Logging" section). `slog.Default()` writes through the standard `log` package unless you call `slog.SetDefault`.
- `Print` and `PrintError` are never called by the package any more; only the deprecated `LogRequest`, `LogResponse` and `LogResponseWithDuration` call `Print`. A service that replaced them to route logs must call `slog.SetDefault` with its handler, or move to `api.WithLogger` ([step 3](#step-3-move-to-apinew)). `Fatal` is never called, as since v0.11.0.
- Bodies and the parsed form are never logged. `LogRequestBody`, `LogResponseBody`, `PrintFullEvent` and `MaxLoggedBodyBytes` now affect only the deprecated `Log*` functions.
- Header values and the query values in `uri` whose key is sensitive are masked, as is the `uri`'s user information. Other header values have the query values after each `?` masked the same way, and the user information too when they carry a URL (`Referer`, `Origin`, `X-Original-URL`, …). The keys are the built-in list plus `SensitiveLogKeys`: the variable can add keys but no longer remove a built-in one.
- A response that cannot be encoded is answered with 500 and the registered formatter's `Error500()` body (bare 500 if it returns nil), and logged as `response cannot be encoded`.

**What still configures the default instance:** `SetRequestMode`, `SetResponseMode` (`CurrentRequestMode`, `CurrentResponseMode`), `TraceVisibility`, `TraceIDLength`, `BlankSuccess`, `SensitiveLogKeys` (adds only) and the components set with `RegisterNewAPIRequestReceiver`, `RegisterNewAPIResponseFormatter` and `RegisterNewAPIResponseWriter`.

Deprecated names are reported by staticcheck (`SA1019`) and by gopls in your editor. `go vet` does not report them. Fix the routing first (step 2); the deprecations can wait until step 3.

### Breaking: no Basic-Secret header

After a successful check, `api.BasicToken` no longer copies the plain password into the `Basic-Secret` request header, on the default instance and on instances. It also removes a `Basic-Secret` header the client sent, so a handler never reads a client-chosen value there. `Basic-Client` (the user name) is still set. A handler that read `Basic-Secret` must take the password from its own credential source, for example the `Authorization` header (`r.BasicAuth()`) or its secret store.

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
		// api.WithLowercaseCodes(), // only if clients still need v0's lowercase codes
	)

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Post(router, "/tasks", api.ProcessRequest(createTask), doc.Body(CreateTaskRequest{}))

	http.ListenAndServe(":8080", a.Handler(router))
}
```

`createTask` does not change. `a.Handler` stores the instance in the request context. `ProcessRequest`, `RequestBody`, the token middlewares, the `Get*Value*` helpers, `WriteError` and `Write(w, r)` on every response type find it there. Route registration (`api.Get`/`Post`/…), `*doc.Docs` and the adapters do not change.

The handler answers byte for byte as on the default instance. `WithLowercaseCodes()` restores the v0.11.0 code case:

```json
{"action":"","code":"TSK-001","content":null,"event_id":"evt1","info":null,"message":"title is required (evt1-TSK-001)","title":"Error response!","token":"","type":"error"}
{"action":"","code":"tsk-001","content":null,"event_id":"evt1","info":null,"message":"title is required (evt1-tsk-001)","title":"Error response!","token":"","type":"error"}
```

`Example_migrationBeforeAndAfter` in `example_migration_test.go` proves it for the handler, served without `ProcessRequest`. Under `ProcessRequest` the bodies also match apart from the generated event IDs; `TestQuickStartAnswersLikeTheDefaultInstance` in `instance_test.go` covers that.

**Wrap the outermost chain.** Middlewares outside `a.Handler` use the default instance. So does `api.Write(data, w)`, which takes no request. Put `a.Handler` around everything, or a rejection from an outer `ProcessRequest` uses the package modes, logs through `slog.Default` instead of the instance logger and is bounded at 10 MiB instead of the instance's limit. With nested instances, the innermost one wins.

### Deprecated names and their options

The names replaced by an instance option keep working, but only for the default instance; the ones marked "ignored" have no effect on the package any more. The `GetQueryParamValue*` helpers and `ResponseCodes.InternalServerEerror` keep working everywhere, instances included.

| Deprecated (v0) | Use instead |
|---|---|
| `SetRequestMode` | `api.New(api.WithRequestMode(...))` |
| `CurrentRequestMode` | `api.New(api.WithRequestMode(...))` |
| `SetResponseMode` | `api.New(api.WithResponseMode(...))` |
| `CurrentResponseMode` | `api.New(api.WithResponseMode(...))` |
| `RegisterNewAPIRequestReceiver` | `api.New(api.WithRequestReceiver(...))` |
| `RegisterNewAPIResponseFormatter` | `api.New(api.WithResponseFormatter(...))` |
| `RegisterNewAPIResponseWriter` | `api.New(api.WithResponseWriter(...))` |
| `Print` | `api.New(api.WithLogger(...))` or `slog.SetDefault` (called only by the deprecated `Log*`) |
| `PrintError` | `api.New(api.WithLogger(...))` or `slog.SetDefault` (ignored) |
| `Fatal` | `api.New(api.WithLogger(...))` or `slog.SetDefault` (ignored) |
| `LogRequest` | `api.New(api.WithLogger(...))` |
| `LogResponse` | `api.New(api.WithLogger(...))` |
| `LogResponseWithDuration` | `api.New(api.WithLogger(...))` |
| `LogRequestBody`, `LogResponseBody`, `PrintFullEvent`, `MaxLoggedBodyBytes` | none: bodies are never logged (they affect only the deprecated `Log*`) |
| `SensitiveLogKeys` | `api.New(api.WithSensitiveLogKeys(...))` |
| `GetQueryParamValueString` | `api.QueryString` (deprecated since v0.9.0) |
| `GetQueryParamValueInt` | `api.QueryInt` (deprecated since v0.9.0) |
| `GetQueryParamValueBool` | `api.QueryBool` (deprecated since v0.9.0) |
| `ResponseCodes.InternalServerEerror` | `ResponseCodes.InternalServerError` (once it is changed from `E300`, it wins over the old field) |

Every option in one place:

```go
a := api.New(
	api.WithRequestMode(api.RequestModePlain),          // SetRequestMode, CurrentRequestMode
	api.WithResponseMode(api.ResponseModeEnvelope),     // SetResponseMode, CurrentResponseMode
	api.WithRequestReceiver(api.RequestReceiver{}),     // RegisterNewAPIRequestReceiver
	api.WithResponseFormatter(api.ResponseFormatter{}), // RegisterNewAPIResponseFormatter
	api.WithResponseWriter(api.ResponseWriter{}),       // RegisterNewAPIResponseWriter
	api.WithLogger(logger),                             // slog.Default; Print, PrintError, Fatal, Log*
	api.WithSensitiveLogKeys("pin"),                    // SensitiveLogKeys
)
```

`Example_migrationOptions` is the compiled version. Pass only the options you need: `api.New()` already uses the built-in receiver, formatter and writer.

**Custom components.** The mode options, and `WithLowercaseCodes` for the formatter, apply to the built-in `RequestReceiver`, `RequestReceiverV2` and `ResponseFormatter`, also when you pass them with an option. A custom receiver or formatter decides on its own how it parses, what body it writes and its code text. A type that embeds a built-in one is custom: its promoted methods read the deprecated package mode variables and send codes as declared. A type that embeds `ResponseWriter` behaves as the default instance's writer: it logs through `slog.Default` and builds its 500 body with the registered formatter. A nil value keeps the built-in.

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

`TraceVisibility`, `TraceIDLength` and `BlankSuccess` are not deprecated either (no instance option replaces them), but they affect only the default instance.

## What an instance adds

**Codes.** An instance sends `core.ResponseCode` as declared, in the `code` field and in the message trace suffix, like the default instance. `api.WithLowercaseCodes()` restores v0's lowercasing (`e001`, `tsk-001`) for clients that cannot change yet.

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

**Logging.** An instance logs only through its `WithLogger` logger, with the same `slog` records as the default instance (`request`, `response`, and error records). Without a logger it logs nothing; the default instance logs through `slog.Default` instead. Both:

- mask the built-in sensitive keys plus the added ones: `WithSensitiveLogKeys` on an instance, `SensitiveLogKeys` on the default instance; changing `SensitiveLogKeys` does not reach an instance;
- mask header values, query values in `uri` (`/login?access_token=[REDACTED]&page=2`) and after each `?` in other header values (`Referer`, `Origin`, `Link`, …), and the user information of the `uri` (`http://[REDACTED]@host/...`); path segments are not;
- log a recovered panic's `panic_type` and, as `panic`, the value only for a `runtime.Error` (`[REDACTED]` otherwise);
- never log bodies or the parsed form;
- log a failed `Get*Value*` helper with the name as `key`; for a sensitive name the `error` gives only the kind of failure, never the value;
- never call `Fatal` or exit from the built-in writer: a body that cannot be encoded is logged and answered with 500 and the `Error500()` body, with the formatter's `Content-Type`. The failed response's headers that describe its body (`Content-Type`, `Content-Encoding`, `Content-Length`, `ETag`, …) are dropped from that 500; the others are kept.

See the README "Logging" section for every record.

**No token echo.** Neither an instance nor the default instance copies the request token (`SecurityToken` header, `Authorization: Bearer`, or the envelope `header.token`) into the response `token` field: plain bodies send `"token":""` and envelope bodies omit `header.token`, for every response kind, 401 included. A token the handler sets on the response (`api.Success{SecurityToken: renewed}`) is still sent, through any formatter. A client that read the token back from responses must keep its own copy, or the handler sets `SecurityToken` from `rctx.SecurityToken`. `RequestHeaderSession` does not copy the `SecurityToken` request header into the response header either; it still answers 401 without it. There is no option to restore the echo.

**Request body limit.** An instance's `Handler` bounds every reader of the request body at 10 MiB (`http.MaxBytesReader`), a handler reading `r.Body` itself included; the default instance bounds only the library's read points. `ProcessRequest`, `RequestBody` and `UnmarshalBody` answer a larger body with 413, code `E002` and the title `Request content too large` (in problem mode, `Request Entity Too Large`). Set another limit with `api.WithMaxBodyBytes(n)`; `n <= 0` is ignored, and `math.MaxInt64` means no practical limit. Nested instance limits combine, so the smallest applies. A custom receiver, or a handler that reads `r.Body` itself, gets the `*http.MaxBytesError` and decides the answer.

**Problem details.** `api.WithResponseMode(api.ResponseModeProblem)` (or `SetResponseMode` on the default instance) writes errors as RFC 9457 `application/problem+json`. That includes the library's own rejections: `ProcessRequest`'s invalid body (400, `E002`) and panic (500, `E300`), the token and header middlewares (401, 415) and `RequestBody`. Other responses are written as their content alone. A non-empty `Content-Type` in the response's `Headers` still wins over `application/problem+json`.

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

## Checklist

- [ ] Upgrade and build. Nothing is removed, so the program compiles.
- [ ] Check clients, alerts and dashboards that match lowercase codes (`e001`), read the request token back from responses or from the `SecurityToken` response header, or send bodies over 10 MiB ([step 1](#step-1-upgrade-what-changes-without-code-changes)).
- [ ] If clients send bodies over 10 MiB, raise the limit with `api.New(api.WithMaxBodyBytes(n)).Handler`; `http.MaxBytesHandler` can only lower it.
- [ ] If you replaced `Print` or `PrintError` to route logs, call `slog.SetDefault` (or move to `api.WithLogger`).
- [ ] Start the service once (or build its router in a test): fix every registration panic (conflicts, methods, wildcard names).
- [ ] Check trailing-slash clients, HEAD skip paths, 405 handling and `GODEBUG=httpmuxgo121`.
- [ ] Validate route variables that reach file paths, headers or logs.
- [ ] Find handlers that read the `Basic-Secret` request header after `api.BasicToken`: it is no longer set.
- [ ] Check responses that set a `Content-Type` in `Headers`: it is now sent (one set on `w.Header()` is still overwritten).
- [ ] Create one instance with `api.New` and wrap the outermost handler with `a.Handler`.
- [ ] Replace each deprecated name with its option (staticcheck `SA1019` lists them).
- [ ] Decide on code case: update clients and alerts to the declared codes, or use `api.WithLowercaseCodes()`.
- [ ] Give the instance a `*slog.Logger`, or accept that it logs nothing.
- [ ] Move tests to `a.Handler(h).ServeHTTP` instead of resetting package variables.
