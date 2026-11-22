# 0004 — Instances document the bodies they send

- Status: accepted (2026-11-22); amended for story 4.1 (see the end)
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
