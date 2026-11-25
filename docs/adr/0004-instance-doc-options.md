# 0004 — Instances document the bodies they send

- Status: accepted (2026-11-22); amended for stories 4.1, 3.1 and 3.3, and smaller decisions recorded on 2026-11-25 (see the end)
- Initiative: api docs match the wire (epic "mode-aware responses", story 1)
- Release: next MINOR (new exported methods, no wire change)

## Context

A route picks its documented response bodies by hand with `envelope.Success[T]()` and `envelope.Error()`, which describe the envelope mode only. A service that runs an instance in `ResponseModeNone` (or switches modes) gets a spec that does not match what it sends. None mode also writes JSON `null` for an error, because an error has no content, and the `doc` package had no way to say so: the contract test kept two known gaps for it.

The `doc` package must not import the `api` root (ADR 0001 §3), so it cannot read a mode itself.

## Decision

1. **Methods on `*api.API`.** `(*API).DocSuccess(status int, sample any) doc.RouteOption` and `(*API).DocError(status int, description string) doc.RouteOption` build ordinary response options from the instance's response mode. `doc` gets no new field, option or dependency, and there is no new package-level state.
2. **Wrappers built at run time from the doc types.** A method cannot be generic, so `DocSuccess` builds the shape of `envelope.ResponseOf[T]` (envelope mode) or `envelope.PlainResponseOf[T]` (plain mode) with `reflect.StructOf`: it copies the fields of the `struct{}` instantiation, tags included, and replaces the type of `Content` with `*T`. The tags have one source, the doc types. The struct is anonymous, so the spec inlines it while named field types such as `header` and `content` stay `$ref`s. A nil sample drops `content` from the envelope, which omits it when empty, and documents it as any nullable value in plain mode, which writes `null`; a pointer sample documents the type it points to.
3. **Schema only.** Neither method writes an example. A sample is usually a zero value, and a zero-value example would document a body the route never sends (the contract stories pin examples to real wire bodies). A real example stays with `doc.StatusWithExample`.
4. **Mode snapshot.** The mode is read when the method is called, not when the spec is generated. An API from `New` stores its normalized `WithResponseMode` value in an unexported field; the default instance, a zero API and a nil API have none and read `CurrentResponseMode`, normalized. A custom formatter set with `WithResponseFormatter` is not inspected: the methods document what the built-in formatter sends, and a service with its own formatter documents its bodies with the `doc` options.
5. **Null body for none-mode errors.** None mode writes JSON `null` for an error, and for a success without content. It is documented as `{"type": "object", "nullable": true, "enum": [null]}`: OpenAPI 3.0.3 requires a type beside `nullable`, and the enum leaves `null` as the only valid value (kin-openapi accepts `null` and rejects objects and strings against it).
6. **Plain and problem modes.** Plain successes use the run-time `envelope.PlainResponseOf[T]` shape (decision 2), and plain errors `envelope.PlainErrorResponse`. Problem successes are documented as none mode, since problem mode sends a success's content alone; problem errors are `envelope.ProblemDetails` under `application/problem+json` only, through `doc.RouteResponse.ContentType`, as the wire sends them.

The wire format does not change. Whether none mode should keep sending `null` for an error is an open owner question recorded in the initiative.

## Consequences

- A route declared with `a.DocSuccess`/`a.DocError` follows its instance's mode; the complete-route contract test passes in the envelope, none, plain and problem modes with no known gaps.
- Existing `doc` options and their output are unchanged.
- The inline envelope and plain success schemas are not named components, unlike `envelope.Success[T]()` and `envelope.PlainSuccess[T]()`; generated clients see an anonymous object for them.
- Routes registered before a default-instance mode change keep the mode they were built with.

## Alternatives

- **Generic functions `api.DocSuccess[T](a, status)`.** Would avoid reflection, but read as a second family of doc helpers beside `envelope.Success[T]()` and still need the instance; a method on the instance keeps the mode next to its owner.
- **`doc` reads the mode** (a mode field on `doc.Docs` or the route). `doc` would have to know `api`'s modes and formatter, against ADR 0001 §3, and every route would depend on generation-time state.
- **Examples from the sample.** Rejected in decision 3: a zero value is not a real body.

## Amendment (story 4.1): examples from an explicit sample

Accepted 2026-11-22. Decision 3 stands for `DocSuccess`, which stays schema-only. A route that wants a real example no longer has to write the nested envelope or plain map by hand with `doc.StatusWithExample`, which drifted from what the formatter sends (code `0000`, blank title, plain's nine keys, custom formatters).

1. **An explicit method.** `(*API).DocSuccessExample(status int, sample any, headers ...doc.ResponseHeaderInfo) doc.RouteOption` documents the response `DocSuccess` documents, with the headers, plus an example. The separate name keeps the author's intent ("this value is the example") in the call, so an example is never written from a type or zero value implicitly. The variadic headers mirror `doc.StatusWithExample`, since a created response carries `Location`.
2. **The formatter, at call time.** The example is the body the instance's response formatter builds for a success carrying `sample`, as `Success.Write` builds it (default title and message, `ResponseCodes.Success`, `SuccessType`), JSON-encoded as the writer encodes it and decoded with `UseNumber`. An API from `New` stores the formatter it bound in an unexported field; the default instance uses the registered one. As in decision 4, it is read when the method is called. The writer is never called, so documentation has no writer side effects such as logging.
3. **No request.** There is no request at documentation time, so the event ID and the token are empty.
4. **Content type.** A Content-Type the formatter sets becomes the response's `doc.RouteResponse.ContentType`.
5. **No example, no panic.** A status without a body, a nil formatted response, a body that cannot be encoded and a JSON `null` body leave the response with its schema only.

Consequences: a custom formatter's computed values (timestamps, random IDs) are frozen when the method is called. The schema still describes the built-in mode, so a custom formatter that changes the body's shape documents with the `doc` options; kin-openapi example validation flags an example its schema rejects. The complete-route contract test now also records a custom-formatter variant and checks that every documented example equals the recorded wire body.

## Amendment (story 3.1): errors from their codes

Accepted 2026-11-25. A route documented its error statuses by hand with `DocError(status, description)`, schema only, repeating per route the statuses `WriteError` already derives from `jgolang/errors` codes, with no example of the code and message a client receives.

1. **One method over codes.** `(*API).DocErrors(errorCodes ...codes.Coder) doc.RouteOption` groups the codes by the status the instance gives their kind and writes one response per status, ordered by status, with the schema `DocError` documents (decisions 5 and 6) and one named example per code. The parameter is not named `codes`, which would shadow the package.
2. **The status `WriteError` uses.** Grouping calls the instance's own kind-to-status function (`WithKindStatus` overrides, else `StatusForKind`, 500 without a kind), so an override moves documentation and wire together. A nil API, the default instance, has no overrides.
3. **The body `WriteError` sends.** `WriteError`'s body rules take the code, not the error (`errorResponseForCode`), and the defaults `Error.Write` applies before writing (`withDefaults`) are shared, so the example is built from the same `Error` value `WriteError` writes, then formatted as in the story 4.1 amendment (decision 2, a shared `formattedExample`). There is no second copy of the status, title or message rules.
4. **Named examples.** The name is the code the response carries: the code's `Str()`, or the default code `WriteError` uses when `Str()` is empty (`ResponseCodes.DefaultError`, or the internal server error code for a 500), so a client sees which code each body carries; the summary is its `Msg()`. A repeated name keeps its first example; a nil code, a nil pointer one included, documents an uncoded error, named by the internal server error code.
5. **No example for JSON null.** As with `DocSuccessExample`, a JSON `null` body (every error in none mode), a nil formatted response or an unencodable body leaves that code without an example, so the response keeps its schema only. A `"value": null` example is valid OpenAPI 3.0, but common validators such as kin-openapi read it as a missing value and reject the spec.
6. **No side effects.** The examples are built when `DocErrors` is called; the writer and the error hook are never called.

Consequences: the complete-route contract test adds a `DELETE` route documented only with `DocErrors` and checks, in every variant whose error body is not `null` (all but none mode), that each recorded wire body equals the named example of its code. Per-code problem types and component responses are out of scope.

## Amendment (story 3.3): the library's own rejections

Accepted 2026-11-25. The library answers some requests itself: 400 for a body `ProcessRequest` or `RequestBody` cannot parse, 401 from the token and session middlewares, 413 for a body over the limit and 500 for a handler panic under `ProcessRequest`. No route documented those bodies, so a client met them first in production.

1. **One opt-in method.** `(*API).DocRejections() doc.RouteOption` adds the 400, 401, 413 and 500 responses, in ascending order, with the schema `DocError` documents and the HTTP status text as description. It is the only new exported name: a route without it documents nothing new, and there is no per-status selection (YAGNI); a later response option for the same status replaces one, as with every response option.
2. **One constructor per body.** The middlewares and `DocRejections` build each rejection with the same unexported constructor (`bodyParseError`, `bodyTooLargeError`, `unauthorizedError`, `Error500`; the session message is a constant), so the documented body cannot drift from the wire. The wire output is unchanged.
3. **Named examples by trigger.** Each body is a named example whose name describes it (`invalidBasicToken`) and whose summary says what triggers it. Codes cannot be the names, because the four 401 bodies share one code.
4. **Built as `DocErrors` builds them.** The defaults `Error.Write` applies are applied, and the body is formatted when the method is called through the shared `formattedExample`: no event ID or token, a formatter Content-Type as the media type, and no example for a nil, unencodable or JSON `null` body (none mode). The message variables are read at that moment.
5. **Out of scope.** The 415 of `RequestHeaderJSON` and response headers such as `WWW-Authenticate` are not documented.

Consequences: the library-rejections goldens record a route with the option and one without it in every variant, validated by the contracttest module, and a test serves each rejection through the real middleware and checks that its body equals the named example once the event ID is removed.

## Smaller decisions

Recorded 2026-11-25 from the initiative's retrospectives. Each one is small, but changes spec output or the exported API in a way a reader would not guess.

1. **A `doc.ExampleObject` in a named-examples map is written as the example object** (v1.4.0, PR #107, commit `f4db89c`). In a `doc.ResponseExamples` or `doc.BodyExamples` map, a `doc.ExampleObject{Summary, Value}` value, or a non-nil pointer to one, is written as the OpenAPI example object with its summary; any other value, a nil pointer included, is wrapped as `{"value": v}`. Reusing the exported type gave summaries without a new type or option. Before v1.4.0 an `ExampleObject` in a `ResponseExamples` map was nested under `value`; the release notes list the change as a fix.
2. **The media type is a field on the route, not an option** (v1.3.0, PR #100, commit `7f23b9a`). `doc.Route.BodyContentType` sets the request body's media type and `doc.RouteResponse.ContentType` a response's; empty means `application/json`, and the string is written as given, so existing routes generate the same output. #75 (API-19) asked for multipart uploads and binary downloads, and problem mode needed `application/problem+json`. The owner's triage of #75 kept the need and declined the proposed `BodyWithContentType` and `ResponseWithContentType` options: a route sets the field with a `func(*doc.Route)` closure, as `docs/openapi.md` shows, and the instance options set `RouteResponse.ContentType` themselves (decision 6 and the story 4.1 amendment, item 4).
3. **The plain doc types mark nullable required fields with `validate:"required"`** (v1.3.0, PR #101, commit `4c07ccd`). The schema walker makes a pointer field nullable and not required, unless the field carries `validate:"required"` (`doc/schema.go`, `isRequiredField`). Plain mode writes every key and sends `info` and `content` as `null` when unset, so `envelope.PlainResponseOf[T]` and `envelope.PlainErrorResponse` declare `Info *map[string]string` and `Content` as pointers with `validate:"required"`, and the spec lists them as required and nullable. It is a doc-only use of a validation tag on documentation types, chosen over a new tag; the type comments say why. `envelope.ProblemDetails` has no pointer field and needs no such tag: its `type` is required because it has no `omitempty`.
