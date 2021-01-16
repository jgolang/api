package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

var (
	// DefaultSuccessTitle doc ...
	DefaultSuccessTitle = "Successful!"
	// DefaultSuccessMessage doc ..
	DefaultSuccessMessage = "The request has been successful!"
	// SuccessType success response type the value is "success"
	SuccessType core.ResponseType = "success"
)

// Success success response type the value is "success"
type Success core.ResponseData

// Write ...
func (success Success) Write(w http.ResponseWriter) {
	success.ResponseType = SuccessType
	if success.Title == "" {
		success.Title = DefaultSuccessTitle
	}
	if success.Message == "" {
		success.Message = DefaultSuccessMessage
	}
	if success.StatusCode == 0 {
		success.StatusCode = http.StatusOK
	}
	api.Write(core.ResponseData(success), w)
}
