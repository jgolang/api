# 0002 — stdadapter routes through ServeMux patterns; a bad route panics

- Status: accepted (2026-11-21)
- Issue: API-08 (#22), PR #38

## Context

`stdadapter` matched `{name}` segments with its own matcher on top of `http.ServeMux`, ranking overlapping routes and deciding 404 and 405 itself. Since Go 1.22, `ServeMux` supports method and wildcard patterns, so the adapter duplicated what the standard library already does (`KISS-01`). The owner chose to ship the switch in v1 as a breaking change (Q9). Draft PR #38 made the switch but kept the first of two conflicting routes and reported the conflict through `api.PrintError`; the review asked to panic instead (Q14).

## Decision

Each route is registered as one Go 1.22 pattern, `METHOD path` (`GET /tasks/{id}`); a path ending in `/` gets `{$}` so it matches only itself. Matching, precedence, 404 and 405 come from `net/http`. Route values are still copied into `api.SetRouteVars`, so `api.GetRouteVar` works unchanged, and `r.PathValue` now works too.

Behavior changes:

1. Precedence is ServeMux's "most specific wins" (`/tasks/new` beats `/tasks/{id}`). Two routes conflict when some request path matches both and neither is more specific: `/tasks/{id}` and `/tasks/{name}`, or crossing routes such as `/tasks/{id}/notes` and `/tasks/new/{kind}` (both match `/tasks/new/notes`). The adapter no longer ranks them. A catch-all `/` already on the caller's `*http.ServeMux` still answers paths no route matches.
2. A request path with a trailing slash no longer matches a route without one (`/tasks/42/` is 404 for `/tasks/{id}`); a registered `/tasks/` still matches only `/tasks/`.
3. An escaped `%2F` inside a segment matches a wildcard (`/files/a%2Fb` matches `/files/{name}` with `name` = `a/b`), because ServeMux matches on the escaped path. A wildcard value can therefore contain `/` and `..` (`/one/..%2F..%2Fetc` gives `name` = `../../etc`; v0.10.0 answered 404). Handlers must validate it before using it in a file path; the README security notes say so.
4. Wildcard names must be Go identifiers (`{user_id}`, not `{user-id}`) and whole segments (`/files/{name}`, not `/files/{name}.json`). `{name}` is the only supported wildcard: ServeMux's `{name...}` and `{$}` panic at registration, because `doc` would publish them as invalid OpenAPI paths and no route needs them yet (YAGNI). The method must be an upper-case RFC 9110 token: `""` would register a method-less catch-all, `GE T` a pattern that never matches, and `get` a route that does not match its documentation, so all three panic.
5. 405 is decided by the whole mux and carries an `Allow` header (`GET, HEAD, PUT`). A catch-all `/` on the caller's mux takes those requests instead, so they get its answer, not 405.
6. A GET route also answers HEAD. Method-keyed logic sees `HEAD`: the skip path `GET:/health` (`api.GenSkipPath`) does not cover `HEAD /health`.
7. Unclean paths (`//tasks`, `/a/../tasks`) keep ServeMux's redirect to the cleaned path (307); this did not change, though earlier notes said 301.
8. `GODEBUG=httpmuxgo121=1` turns off pattern matching and breaks every route, and it also turns off the conflict panic. It can come from the environment, a `//go:debug httpmuxgo121=1` directive, or a `godebug` line in the main module's `go.mod`. The adapter requires the Go 1.22 mux and does not try to detect the setting.

A conflicting route, an invalid method, or an invalid or unsupported pattern makes `Router.Handle` panic, as `http.ServeMux.Handle` does. For a conflict, ServeMux's message names both patterns. It is a programming error in the route table, found at start-up, and documented on `Handle` (constitution V). The route is registered on the mux before its OpenAPI metadata, so a rejected route is neither served nor documented. Nothing is printed. `api.Router` and the `api.Get`/`Post`/… helpers say that a router may panic this way.

## Consequences

- Breaking for every `stdadapter` user; each change above is listed in the README routing section and pinned by a row in `stdadapter/behavior_test.go`. The existing adapter tests pass unchanged.
- A route table with conflicts that used to start (keeping the first route) now panics at start-up.
- `BenchmarkRouterServe`, re-measured for this ADR rather than taken from the PR #38 draft (five routes, `GET /tasks/42/notes`, `-count 5`, median, Go 1.26.11 linux/amd64): master 1681 ns/op, 1088 B/op, 13 allocs/op; ServeMux patterns 1054 ns/op, 880 B/op, 8 allocs/op (1681 → 1054 ns/op).
- Other adapters are unaffected; they still call `api.SetRouteVars` or replace `api.GetRouteVar`.

## Alternatives

- **Keep the hand-written matcher.** No breaking change, but it duplicates `net/http`, is slower, and diverges from what Go users expect of `ServeMux`.
- **Keep the first conflicting route and report it (PR #38).** Starts despite a broken route table, hides which handler serves a path, and reports through `api.PrintError`, a package-level logger the v1 instance epic deprecates (constitution IX).
