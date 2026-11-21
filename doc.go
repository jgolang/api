// Package api is a small HTTP request/response framework: it parses request
// bodies (plain JSON or the header/content envelope), writes success, error,
// warning and informative responses in a consistent JSON format, and maps
// coded errors from github.com/jgolang/errors to HTTP statuses.
//
// Configure an instance with New and its options, and wrap the outermost
// handler (usually the router) with (*API).Handler; the package-level
// writers and middlewares then use that instance for the request.
//
// Without an instance, the package-level functions use a deprecated default
// instance configured by package variables and the RegisterNew* functions.
// It behaves as in v0 so existing programs keep working; see
// docs/migration-v1.md for the move to New.
package api
