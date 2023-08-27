package core

import "net/http"

// APIResponseFormatter implement this interface to format the API
// response information to JSON.
type APIResponseFormatter interface {
	// Format the response body information.
	Format(ResponseData) *ResponseFormatted
}

// APIResponseWriter implement this interface to write API response
// information in screen.
type APIResponseWriter interface {
	// Write the API response in screen.
	Write(*ResponseFormatted, http.ResponseWriter)
}
