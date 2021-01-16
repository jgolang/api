package core

import "net/http"

// APIRequestValidater doc ...
type APIRequestValidater interface {
	ValidateRequest(*http.Request) (*RequestData, error)
}
