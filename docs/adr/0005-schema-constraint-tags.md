# 0005 — Constraint tags reach the schema

- Status: accepted (2026-11-25)
- Initiative: api docs match the wire (epic "constraints from tags", story 1)
- Issue: jgolang/api#76 (API-20)
- Release: next MINOR (new `doc.Schema` fields and `doc.CheckSchemaTags`; specs without the new tags are unchanged)

## Context

`doc` read only `json`, `description`, `format`, `example` and `validate:"required"` from body struct fields. Limits and closed value lists, such as #76's `target_type` (`goal`, `project`, `task`, `note`), never reached the spec, so clients learned them from descriptions. The need is shared by every consumer with closed lists or limits.

## Decision

1. **Explicit tags only.** The schema walker reads `enum`, `default`, `minimum`, `maximum`, `minLength`, `maxLength`, `pattern` and `deprecated` from a struct field, in the one field loop that serves both `GenerateOpenAPI` and `SchemaFromType`. Each writes the OpenAPI keyword of the same name into a new `doc.Schema` field (`Minimum`, `Maximum`, `MinLength`, `MaxLength`, `Pattern`, `Default`, `Deprecated`; `Enum` already existed). No other tag is added.
2. **Typed values.** `enum` and `default` values are typed by the field's kind, after dereferencing pointers: `int64` for signed integers, `uint64` for unsigned, `float64` for floats, `bool`, and the raw string for strings and `time.Time` (which must be RFC 3339). Each is parsed with the field's bit size, so `enum:"300"` on an `int8` is rejected. `enum` applies to scalar fields only, and on a nullable (pointer) field it also lists `null`, because OpenAPI 3.0.3 validators check `enum` even for `null`; a non-scalar `default` is read as JSON. Limits are finite numbers; lengths are non-negative integers; a pattern must compile with `regexp`.
3. **Leave out and report.** A value that does not parse is left out of the schema, never guessed. `GenerateOpenAPI` keeps its signature; the new `doc.CheckSchemaTags(docs) error` runs the same walk and returns each bad tag as an `errors.NewC(codes.CfgInvalid, ...)` error naming the Go type, field, tag and value, joined with `errors.Join`. `SchemaFromType` leaves bad values out silently, and `CheckSchemaTags` cannot see a ready-made `*doc.Schema` passed to `BodySchema` or `ResponseSchema`.
4. **Documentation only.** The tags do not validate requests.

## Consequences

- A struct without the new tags generates byte-identical output.
- A service can fail its startup or a test on a tag typo with one call.
- `cloneSchema` copies the new pointers and `Default`, so routes registered with the same `*Schema` do not share them.
- OpenAPI patterns are ECMA-262; checking them with Go's RE2 rejects a few valid ECMA patterns (look-arounds, back-references) and accepts a few RE2-only ones.

## Alternatives

- **Read `validate` rules** (`min=`, `max=`, `oneof=`). Rejected by owner decision D2: the library does not own the validator's rule language, and its rules do not map one-to-one onto OpenAPI keywords.
- **Panic on a bad tag.** Rejected: library code must not panic on caller input (constitution, Principle V).
- **Log a bad tag.** Rejected: `api` does not write logs as a side effect (Principle IX); the caller decides through `CheckSchemaTags`.
- **Return an error from `GenerateOpenAPI`.** Rejected: a breaking signature change for every caller (Principle V).
