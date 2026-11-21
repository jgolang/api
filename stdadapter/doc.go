// Package stdadapter is the api.Router adapter for net/http: it registers
// routes on an http.ServeMux as Go 1.22 method and wildcard patterns and
// records them for the OpenAPI document. Importing it registers the adapter
// under the name "std" for api.NewRouter. ADR 0002
// (docs/adr/0002-stdadapter-servemux.md) records the routing behavior and
// why a conflicting route panics.
package stdadapter
