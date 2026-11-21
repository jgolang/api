// Package api is a small HTTP request/response framework: it parses request
// bodies (plain JSON or the header/content envelope), writes success, error,
// warning and informative responses in a consistent JSON format, and maps
// coded errors from github.com/jgolang/errors to HTTP statuses.
//
// Configure an instance with New and its options, and wrap the outermost
// handler (usually the router) with (*API).Handler; the package-level
// writers and middlewares then use that instance for the request.
//
// Without an instance, the package-level functions use a default instance
// that behaves like New() with its defaults. It logs through slog.Default,
// and the deprecated package variables and RegisterNew* functions still
// configure its modes, sensitive keys and components; see
// docs/migration-v1.md for the move to New.
package api
