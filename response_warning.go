package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

var (
	// DefaultWarningTitle doc ...
	DefaultWarningTitle = "Alert!"
	// DefaultWarningMessage doc ..
	DefaultWarningMessage = "The application has been successful but with potential problems!"
	// WarningType warning response type the value is "warning"
	WarningType core.ResponseType = "warning"
)

// Warning warning response type the value is "warning"
type Warning core.ResponseData

// Send ...
func (warning Warning) Send(w http.ResponseWriter) {
	warning.ResponseType = WarningType
	if warning.Title == "" {
		warning.Title = DefaultWarningTitle
	}
	if warning.Message == "" {
		warning.Message = DefaultWarningMessage
	}
	if warning.StatusCode == 0 {
		warning.StatusCode = http.StatusOK
	}
	api.Respond(core.ResponseData(warning), w)
}
