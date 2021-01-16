package core

import "net/http"

// APIRequestValidator doc ...
type APIRequestValidator interface {
	ValidateRequest(*http.Request) (*RequestData, error)
	// GetRouteVar returns the route var for the current request, if any.
	GetRouteVar(string, *http.Request) string
}
