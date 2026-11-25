package core

import "net/http"

// Middleware provides a convenient mechanism for filtering HTTP requests
// entering the application. It returns a new handler which may perform various
// operations and should finish by calling the next HTTP handler.
type Middleware func(next http.HandlerFunc) http.HandlerFunc

// MiddlewaresChain provides syntactic sugar to create a new middleware
// which will be the result of chaining the ones received as parameters.
//
// The first middleware is the outermost: MiddlewaresChain(a, b)(h) runs a,
// then b, then h. The middlewares wrap final once, when the chain is applied,
// not per request. The chain writes straight to the http.ResponseWriter it is
// given, nothing is buffered, so responses stream and Flush and
// http.ResponseController reach the connection.
func MiddlewaresChain(mw ...Middleware) Middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		for i := len(mw) - 1; i >= 0; i-- {
			final = mw[i](final)
		}
		return final
	}
}
