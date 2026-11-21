// Package core defines the pluggable parts of an API: APIRequestReceiver
// parses requests, APIResponseFormatter builds response bodies,
// APIResponseWriter writes them, APISecurityGuarantor validates tokens, and
// ParamValidator checks request params. API, built with New, composes one of
// each with MapMethods. Package api provides the built-in implementations.
package core
