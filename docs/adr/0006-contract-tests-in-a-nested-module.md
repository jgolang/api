# 0006 — Contract tests live in a nested module

- Status: accepted (2026-11-25); records the layout built in story 6.2 (2026-11-21)
- Initiative: api docs match the wire (epic "quality gate and contract tests", story 2; owner decision D3)
- PR: #96 (commits `937a184`, `2fb250c`)
- Release: v1.2.0 (test-only; no exported change)

## Context

Nothing checked that the responses an instance really sends (status, media type, body) match the spec `doc` generates for them, so spec and wire could drift unnoticed. Checking a response against an OpenAPI document needs a validator; the initiative chose kin-openapi.

The owner decided that kin-openapi stays out of `api`'s main `go.mod` (D3): the root module depends only on `jgolang/errors` and `jgolang/helpers`, and the constitution moves optional integrations into a nested module so that importing the root package does not pull them in (Principle VI, Technology Constraints). A nested module that tested `api` by importing it would have to require `github.com/jgolang/api` and resolve it to the working tree, which takes a `replace` directive pointing at a local path or a `go.work`; Principle VI forbids committing either (initiative constraint C4).

## Decision

1. **A nested, test-only module.** `contracttest/` is the module `github.com/jgolang/api/contracttest`, with only test files (`contract_test.go`, `spec_test.go`). Its `go.mod` requires kin-openapi and its indirect dependencies, never `github.com/jgolang/api`, and has no `replace`. There is no `go.work`.
2. **The root module records, the nested module reads.** The root module's own tests build the routes, generate the spec and run real requests through `a.Handler`, then write the spec to `testdata/openapi/` and the exchanges (method, path, request, status, content type, headers, body) to `testdata/contract/`. They are rewritten only with `go test -update .`; without the flag a stale file fails the test. `contracttest` reads `../testdata` and never imports `api`.
3. **What it checks.** `TestRecordedResponsesMatchSpec` validates each recorded response against its spec with `openapi3filter.ValidateResponse`. Object schemas are made strict first (`additionalProperties: false` where the schema has properties and does not set it), so an undeclared field fails. `TestValidatorRejectsMismatches` proves the harness fails on an undeclared field, a wrong media type and an undocumented status. `TestSpecsAreValid`, added in story 6.3 (#97), validates every `testdata/openapi/*.json` spec with examples checked against their schemas, so a test that writes a new spec there is covered without changes.
4. **CI and release.** CI runs `go vet`, `go test` and `govulncheck -test` in `contracttest` after the root checks (`-test` because the module has only test files). `CONTRIBUTING.md` lists the same commands and makes the contract test a release check.

## Consequences

- `api`'s `go.mod` and `go.sum` do not change; services that import `api` never download kin-openapi.
- The contract check runs on recordings, not on live calls. A behavior change reaches it only after the root tests are rerun with `-update` and the new recordings are committed, so a golden diff is the review point for any change to the spec or the wire.
- Every recorded scenario starts as a root test: each epic that documents something new adds a spec or exchange file under `testdata/` (envelope, none, plain, problem and custom-formatter variants, library rejections, component responses, constraint tags, named examples).
- Contributors run checks in two modules; `gofmt -l .` in the root covers both.
- kin-openapi decides what "valid" means. Where it rejects a valid OpenAPI 3.0 construct, such as a `"value": null` example, the generator avoids the construct (ADR 0004, amendment for story 3.1, item 5).

## Alternatives

- **kin-openapi in the root module's tests.** Simplest to write, but a test-only requirement still lands in `api`'s `go.mod` and `go.sum`, against owner decision D3.
- **A nested module that imports `api`.** Would test live handlers, but needs a committed `replace github.com/jgolang/api => ../` or a `go.work`, which Principle VI forbids (C4); without one it would test the last published `api`, not the change under review.
