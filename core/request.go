package core

import "net/http"

// APIRequestReceiver implemnt this interface to process request body
// information.
type APIRequestReceiver interface {
	// ProcessBody API request body information
	ProcessBody(*http.Request) (*RequestDataContext, error)
	// ProcessEncryptedBody API request url encode data
	ProcessEncryptedBody(*http.Request) (*RequestEncryptedData, error)
	// GetRouteVar returns the route var for the current request, if any.
	GetRouteVar(string, *http.Request) string
}
