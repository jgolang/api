package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

var (
	// DefaultWarningTitle default warning title.
	DefaultWarningTitle = "Alert!"

	// DefaultWarningMessage default warning message.
	DefaultWarningMessage = "The application has been successful but with potential problems!"

	// WarningType warning response type the value is "warning"
	WarningType core.ResponseType = "warning"

	// DefaultWarningCode default warning code.
	DefaultWarningCode = "warning"
)

// Warning warning response type the value is "warning".
type Warning core.ResponseData

// Write warning response in screen.
func (warning Warning) Write(w http.ResponseWriter, r *http.Request) {
	warning.EventID = getEventID(r)
	warning.ResponseType = WarningType
	if warning.Title == "" {
		warning.Title = DefaultWarningTitle
	}
	if warning.Message == "" {
		warning.Message = DefaultWarningMessage
	}
	if warning.HTTPStatusCode == 0 {
		warning.HTTPStatusCode = http.StatusContinue
	}
	if warning.ResponseCode == "" {
		warning.ResponseCode = ResponseCodes.Warning
	}
	if warning.SecurityToken == "" {
		warning.SecurityToken = getSecurityToken(r)
	}
	api.Write(core.ResponseData(warning), w)
}
