package core

import "net/http"

// APIResponseFormatter doc ...
type APIResponseFormatter interface {
	Format(ResponseData) *ResponseFormatted
}

// APIResponder doc ...
type APIResponder interface {
	Respond(*ResponseFormatted, http.ResponseWriter)
}
