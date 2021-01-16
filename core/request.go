package core

import "net/http"

// APIRequestValidator doc ...
type APIRequestValidator interface {
	ValidateRequest(*http.Request) (*RequestData, error)
}
