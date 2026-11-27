# AGENTS.md

## Project

**api**: HTTP request/response micro-framework for Go: envelope, plain or RFC 9457 responses, statuses from jgolang/errors kinds, router adapters and OpenAPI generation

- **Stack:** Go (`go.mod`).
- **Layout** (from the card `context/repos/api.md` of the workbench):
  - `instance.go`: the `API` instance, `New(opts...)` and `Handler` (`instance.go:88`, `instance.go:428`).
  - `modes.go`: request modes (plain by default) and response modes (envelope by default) (`modes.go:10`, `modes.go:31`).
  - `middlewares.go`: `ProcessRequest`, the full request flow (`middlewares.go:280`).
  - `router.go`, `adapters.go`: the `Router` interface, `Get`/`Post`/…, and the adapter registry (`router.go:11`, `adapters.go:21`).
  - `stdadapter/`: the `net/http` `ServeMux` adapter (`stdadapter/stdadapter.go:34`).
  - `core/`: the pluggable interfaces (`APIRequestReceiver`, `APIResponseFormatter`, `APIResponseWriter`, `APISecurityGuarantor`, `ParamValidator`) composed by `core.API` (`core/core.go:21`).
  - `response_*.go`, `write_error.go`: success, error, warning and informative responses, and `WriteError` (`write_error.go:42`).
  - `doc/`: the OpenAPI builder and handlers (`doc/registry.go:132`, `doc/handlers.go:12`).
  - `envelope/`: wire types for the envelope, plain and problem formats, used for docs.
  - `contracttest/`: nested test-only module; `testdata/`: golden specs and recorded responses.
- **Workbench:** part of the jgolang project; its workbench is `jgolang/workbench-jgolang`.

## Commands

The checks CI runs on Go oldstable and stable (`.github/workflows/ci.yml`); run them before a change is done:

```sh
test -z "$(gofmt -l .)"
go vet ./...
go test -race -cover ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
(cd contracttest && go vet ./... && go test ./... && go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 -test ./...)
```

After an intended change to a generated spec or a recorded response, refresh the goldens and review the diff: `go test -update .` and `go test -update ./doc` (`CONTRIBUTING.md`).

## OpenAPI work

- Validate any new spec construct (a keyword, a `null` example, a new schema shape) against kin-openapi in `contracttest` before approving a plan that relies on it; a construct the validator rejects wastes a build (workbench-jgolang#13).
- A ticket that reads a schema tag says both that the value parses for the field type and that the keyword applies to that type (workbench-jgolang#16, as in #115).
- Call out every golden spec change in the release notes (`CONTRIBUTING.md`).

## Inherited rules

Read [docs/agents/foundations.md](docs/agents/foundations.md) (global rules, managed by the template) and [docs/agents/project.md](docs/agents/project.md) (this project's rules, synced from the workbench by agents-sync) before you work. The rules here and the invariants below apply even if you cannot read them.

## Invariants

No level can change these, not even with a deviation:

- **I1.** Never commit secrets, credentials or personal data to git.
- **I2.** Never force-push or rewrite the history of the main branch or of a branch with an open PR.
- **I3.** Nothing enters the main branch without a PR and without its checks passing.
- **I4.** Never disable or skip tests or checks to make something pass.
- **I5.** Report vulnerabilities privately, never in public issues.
- **I6.** A change is not done until the documentation it affects is updated.
- **I7.** Ask for the owner's decisions, do not assume them; confirm what is hard to revert before doing it.
- **I8.** Respect licenses and ownership: do not copy code or content from third parties or other organizations without permission.

## Deviations

None. When this repo changes something a higher level (global, organization or project) defines, add one line here: what changes, why, and a link to the issue or decision. Delete the line when it no longer applies. Invariants cannot be deviated.
