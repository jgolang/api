package core

import "net/http"

// APIResponseFormatter doc ...
type APIResponseFormatter interface {
	Format(ResponseData) *ResponseFormatted
}

// APIWriter doc ...
type APIWriter interface {
	Write(*ResponseFormatted, http.ResponseWriter)
}
