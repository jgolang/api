package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

var (
	// DefaultInfoTitle default informative title.
	DefaultInfoTitle = "Information!"

	// DefaultInfoMessage  default informative message.
	DefaultInfoMessage = "The request has been successful!"

	// InformativeType info response type the value is "info".
	InformativeType core.ResponseType = "info"

	// DefaultInfoCode default informative code.
	DefaultInfoCode = "success"
)

// Informative info response type the value is "info".
type Informative core.ResponseData

// Write informative message in screen.
func (info Informative) Write(w http.ResponseWriter, r *http.Request) {
	info.EventID = getEventID(r)
	info.ResponseType = InformativeType
	if info.Title == "" {
		info.Title = DefaultInfoTitle
	}
	if info.Message == "" {
		info.Message = DefaultInfoMessage
	}
	if info.HTTPStatusCode == 0 {
		info.HTTPStatusCode = http.StatusContinue
	}
	if info.ResponseCode == "" {
		info.ResponseCode = ResponseCodes.Informative
	}
	if info.SecurityToken == "" {
		info.SecurityToken = getSecurityToken(r)
	}
	api.Write(core.ResponseData(info), w)
}
