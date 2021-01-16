package core

import (
	"net/http"
	"net/http/httptest"
)

// Middleware provides a convenient mechanism for filtering HTTP requests
// entering the application. It returns a new handler which may perform various
// operations and should finish by calling the next HTTP handler.
type Middleware func(next http.HandlerFunc) http.HandlerFunc

// MiddlewaresChain provides syntactic sugar to create a new middleware
// which will be the result of chaining the ones received as parameters.
func MiddlewaresChain(mw ...Middleware) Middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(mw) - 1; i >= 0; i-- {
				last = mw[i](last)
			}
			res := httptest.NewRecorder()
			last(res, r)
			for i, header := range res.Header() {
				w.Header()[i] = header
			}
			w.WriteHeader(res.Code)
			w.Write(res.Body.Bytes())
		}
	}
}
