# 0004 — Instances document the bodies they send

- Status: accepted (2026-11-22)
- Initiative: api docs match the wire (epic "mode-aware responses", story 1)
- Release: next MINOR (new exported methods, no wire change)

## Context

A route picks its documented response bodies by hand with `envelope.Success[T]()` and `envelope.Error()`, which describe the envelope mode only. A service that runs an instance in `ResponseModeNone` (or switches modes) gets a spec that does not match what it sends. None mode also writes JSON `null` for an error, because an error has no content, and the `doc` package had no way to say so: the contract test kept two known gaps for it.

The `doc` package must not import the `api` root (ADR 0001 §3), so it cannot read a mode itself.

## Decision

1. **Methods on `*api.API`.** `(*API).DocSuccess(status int, sample any) doc.RouteOption` and `(*API).DocError(status int, description string) doc.RouteOption` build ordinary response options from the instance's response mode. `doc` gets no new field, option or dependency, and there is no new package-level state.
2. **Envelope wrapper built at run time.** A method cannot be generic, so `DocSuccess` builds the shape of `envelope.Success[T]()` with `reflect.StructOf` (`Header envelope.ResponseInfo json:"header"`, `Content *T json:"content,omitempty"`). The struct is anonymous, so the spec inlines it while `header` and `content` stay `$ref`s. A nil sample gives an envelope with only its header; a pointer sample documents the type it points to.
3. **Schema only.** Neither method writes an example. A sample is usually a zero value, and a zero-value example would document a body the route never sends (the contract stories pin examples to real wire bodies). A real example stays with `doc.StatusWithExample`.
4. **Mode snapshot.** The mode is read when the method is called, not when the spec is generated. An API from `New` stores its normalized `WithResponseMode` value in an unexported field; the default instance, a zero API and a nil API have none and read `CurrentResponseMode`, normalized. A custom formatter set with `WithResponseFormatter` is not inspected: the methods document what the built-in formatter sends, and a service with its own formatter documents its bodies with the `doc` options.
5. **Null body for none-mode errors.** None mode writes JSON `null` for an error, and for a success without content. It is documented as `{"type": "object", "nullable": true, "enum": [null]}`: OpenAPI 3.0.3 requires a type beside `nullable`, and the enum leaves `null` as the only valid value (kin-openapi accepts `null` and rejects objects and strings against it).
6. **Modes without a wrapper yet.** Until plain and problem bodies get their own wrappers, plain successes and plain and problem errors are documented as envelope (wrong for those modes), and problem successes as none (correct: problem mode sends a success's content alone).

The wire format does not change. Whether none mode should keep sending `null` for an error is an open owner question recorded in the initiative.

## Consequences

- A route declared with `a.DocSuccess`/`a.DocError` follows its instance's mode; the complete-route contract test passes in envelope and none modes with no known gaps.
- Existing `doc` options and their output are unchanged.
- The inline success schema is not a named component, unlike `envelope.Success[T]()`; generated clients see an anonymous object for it.
- Routes registered before a default-instance mode change keep the mode they were built with.

## Alternatives

- **Generic functions `api.DocSuccess[T](a, status)`.** Would avoid reflection, but read as a second family of doc helpers beside `envelope.Success[T]()` and still need the instance; a method on the instance keeps the mode next to its owner.
- **`doc` reads the mode** (a mode field on `doc.Docs` or the route). `doc` would have to know `api`'s modes and formatter, against ADR 0001 §3, and every route would depend on generation-time state.
- **Examples from the sample.** Rejected in decision 3: a zero value is not a real body.
