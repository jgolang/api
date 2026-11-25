# 0009 — A $ref with sibling keys is written as allOf

- Status: accepted (2026-11-25); records the rule built in story 1.1 (2026-11-21)
- Initiative: api docs match the wire (epic "generator fixes and hygiene", story 1)
- PR: #87 (commit `6874df7`)
- Release: v1.2.0 (MINOR: new field `doc.Schema.AllOf`; spec output changes for pointer and tagged struct fields)

## Context

`GenerateOpenAPI` writes each named struct type once under `components.schemas` and references it with `$ref`. A pointer field (`Owner *Assignee`), or a struct field with a tag such as `description`, was written as `{"$ref": ..., "nullable": true, "description": ...}`. OpenAPI 3.0.3 ignores every key beside `$ref`, so clients lost the field's nullability and description, and the spec was not clean 3.0.3.

## Decision

1. **Wrap in `allOf`.** When a schema the generator builds holds a `$ref` and any other key (`nullable`, `description`, `format`, `example`, or a constraint tag added later), it is written as `{"allOf": [{"$ref": ...}], <other keys>}`. The rule runs on the pointer branch of the walker and after a field's tags are applied, so it covers pointer fields, slice items and map values (`[]*Owner`) and tagged struct fields. `doc.Schema` gains `AllOf []*Schema`, added as its last field so existing key order does not change.
2. **Any key counts.** "Has siblings" means the schema differs from `Schema{Ref: ...}`, not a list of known keys, so a tag added later is wrapped without another change (ADR 0005's `deprecated` on a referenced field relies on it).
3. **Pointer body samples stay a plain `$ref`.** A body or response passed as a pointer sample (`doc.Body(&T{})`, `doc.Status(200, &T{})`) is dereferenced before the walk: a pointer sample is a way to pass a value, not a declaration that the whole body may be `null`.
4. **Plain references stay plain.** A non-pointer struct field without tags is still a plain `$ref`, and an embedded `*Struct` still merges its properties into the parent (the walker follows a one-entry `allOf` when it resolves the embedded reference). `doc.SchemaFromType`, which inlines and writes no references, is unchanged.

The wire format does not change.

## Consequences

- The spec carries the nullability and descriptions of referenced fields, and no generated object holds `$ref` beside another key.
- Spec output changes for every pointer or tagged struct field that references a component, such as the envelope doc types' `Content *T`, which becomes a nullable `allOf`. Generated clients may model such a field differently.
- `doc.Schema.AllOf` is a new exported field, so the story shipped as MINOR, not the PATCH its ticket planned (inside the v1.2.0 MINOR release).
- A service that wants a nullable top-level body cannot say so with a pointer sample; it passes a `*doc.Schema` with `BodySchema` or `ResponseSchema`.

## Alternatives

- **Keep `$ref` siblings.** No output change, but the generator writes OpenAPI 3.0.3, which ignores them, so nullability and descriptions stay lost.
- **Treat a pointer body sample as nullable too.** The first draft of the story did this: one rule for every pointer, but every `doc.Body(&T{})` body became a nullable `allOf`. Review rejected it, since a pointer sample does not mean a null body.
