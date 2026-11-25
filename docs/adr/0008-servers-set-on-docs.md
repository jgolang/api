# 0008 — Servers are set on Docs so doc.API stays comparable

- Status: accepted (2026-11-25); records the choice made in story 1.4 (2026-11-21, plan "Loopback 1")
- Initiative: api docs match the wire (epic "generator fixes and hygiene", story 4)
- PR: #90 (commit `1a662bc`)
- Release: v1.2.0 (MINOR: `Docs.SetServers`, `doc.Server`, `doc.Contact`, `doc.License`, `doc.API.Contact`, `doc.API.License`)

## Context

The generated spec could not declare `servers`, so Swagger UI's "Try it out" and generated clients had no base URL, nor `info.contact` and `info.license`. The ticket proposed three new fields on `doc.API`, the struct passed to `doc.New` and returned by `Docs.API`: `Servers []Server`, `Contact *Contact` and `License *License`.

Review found that a slice field makes `doc.API` non-comparable. A caller that compares two `doc.API` values with `==` or uses one as a map key would stop compiling, a breaking change that needs a MAJOR release at v1 (constitution, Principle V). The draft also wrote the servers in two places, in `info` and at the top level, while OpenAPI has them only at the top level.

## Decision

1. **Servers on `Docs`.** `(*doc.Docs).SetServers(servers ...doc.Server)` sets the spec's top-level `servers`, in order, beside `SetDefaultSecurity`. Each call replaces the list, a call with none clears it, and a server with an empty URL is dropped, since OpenAPI requires one. `GenerateOpenAPI` reads the list under the `Docs` lock. A nil `*Docs` is a no-op.
2. **Contact and License as pointers on `doc.API`.** `doc.API` gains only `Contact *Contact` and `License *License`, written to `info.contact` and `info.license`. Pointers keep the struct comparable.
3. **Copies out.** `doc.New` and `Docs.API` copy `Contact` and `License`, so a caller cannot change the stored metadata through a value it holds.
4. **No server variables** and no exported reader for the servers (YAGNI-01): `GenerateOpenAPI` is the reader.

A `doc.API` without the new fields, on a `Docs` without servers, generates byte-identical output.

## Consequences

- `doc.API` stays comparable; existing `==` and map-key callers compile.
- `==` on two `doc.API` values compares the `Contact` and `License` pointers, not what they hold, and each `Docs.API` call returns new copies, so two calls are not `==` when either pointer is set. The `API` godoc says so.
- API metadata lives in two places: identity in `doc.API` (passed once to `doc.New`), servers on `Docs` (set any time before generation). The README and `docs/openapi.md` show both.

## Alternatives

- **`Servers []Server` on `doc.API`, as ticketed.** One place for all metadata, but a non-comparable `doc.API`: a breaking change at v1, and in the draft the servers were written twice.
