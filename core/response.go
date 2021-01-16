package core

import "net/http"

// APIResponseFormatter doc ...
type APIResponseFormatter interface {
	Format(ResponseData) *ResponseFormatted
}

// APIResponseWriter doc ...
type APIResponseWriter interface {
	Write(*ResponseFormatted, http.ResponseWriter)
}
