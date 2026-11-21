# 0002 — stdadapter routes through ServeMux patterns; a bad route panics

- Status: accepted (2026-11-21)
- Issue: API-08 (#22), PR #38

## Context

`stdadapter` matched `{name}` segments with its own matcher on top of `http.ServeMux`, ranking overlapping routes and deciding 404 and 405 itself. Since Go 1.22, `ServeMux` supports method and wildcard patterns, so the adapter duplicated what the standard library already does (`KISS-01`). The owner chose to ship the switch in v1 as a breaking change (Q9). Draft PR #38 made the switch but kept the first of two conflicting routes and reported the conflict through `api.PrintError`; the review asked to panic instead (Q14).

## Decision

Each route is registered as one Go 1.22 pattern, `METHOD path` (`GET /tasks/{id}`); a path ending in `/` gets `{$}` so it matches only itself. Matching, precedence, 404 and 405 come from `net/http`. Route values are still copied into `api.SetRouteVars`, so `api.GetRouteVar` works unchanged, and `r.PathValue` now works too.

Behavior changes:

1. Precedence is ServeMux's "most specific wins" (`/tasks/new` beats `/tasks/{id}`); routes that overlap with equal specificity (`/tasks/{id}` and `/tasks/{name}`) conflict instead of being ranked by the adapter.
2. A request path with a trailing slash no longer matches a route without one (`/tasks/42/` is 404 for `/tasks/{id}`); a registered `/tasks/` still matches only `/tasks/`.
3. An escaped `%2F` inside a segment matches a wildcard (`/files/a%2Fb` matches `/files/{name}` with `name` = `a/b`), because ServeMux matches on the escaped path.
4. Wildcard names must be Go identifiers (`{user_id}`, not `{user-id}`) and whole segments (`/files/{name}`, not `/files/{name}.json`).
5. 405 is decided by the whole mux and carries an `Allow` header (`GET, HEAD, PUT`).
6. A GET route also answers HEAD.
7. Unclean paths (`//tasks`, `/a/../tasks`) keep ServeMux's redirect to the cleaned path (307); this did not change, though earlier notes said 301.
8. `GODEBUG=httpmuxgo121=1` turns off pattern matching and breaks every route. The adapter requires the Go 1.22 mux and does not try to detect the setting.

A conflicting route (the same method and path, or wildcards in the same place) or an invalid pattern makes `Router.Handle` panic, as `http.ServeMux.Handle` does, with ServeMux's message naming both patterns. It is a programming error in the route table, found at start-up, and documented on `Handle` (constitution V). The route is registered on the mux before its OpenAPI metadata, so a rejected route is not documented. Nothing is printed.

## Consequences

- Breaking for every `stdadapter` user; each change above is listed in the README routing section and pinned by a row in `stdadapter/behavior_test.go`. The existing adapter tests pass unchanged.
- A route table with conflicts that used to start (keeping the first route) now panics at start-up.
- `BenchmarkRouterServe` (five routes, `GET /tasks/42/notes`, `-count 5`, median, Go 1.26.11 linux/amd64): master 1681 ns/op, 1088 B/op, 13 allocs/op; ServeMux patterns 1054 ns/op, 880 B/op, 8 allocs/op.
- Other adapters are unaffected; they still call `api.SetRouteVars` or replace `api.GetRouteVar`.

## Alternatives

- **Keep the hand-written matcher.** No breaking change, but it duplicates `net/http`, is slower, and diverges from what Go users expect of `ServeMux`.
- **Keep the first conflicting route and report it (PR #38).** Starts despite a broken route table, hides which handler serves a path, and reports through `api.PrintError`, a package-level logger the v1 instance epic deprecates (constitution IX).
