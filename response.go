package api

import (
	"log/slog"
	"net/http"

	"github.com/jgolang/api/core"
)

// Response wrapper to generate core responses.
type Response interface {
	Write(w http.ResponseWriter, r *http.Request)
}

func getEventID(r *http.Request) string {
	requestData, err := GetRequestContext(r)
	if err != nil {
		return r.Header.Get(EventIDHeaderKey)
	}
	if requestData.EventID != "" {
		return requestData.EventID
	}
	return r.Header.Get(EventIDHeaderKey)
}

// ResponseWriter implementation of core.APIResponseWriter interface.
// The zero value is the default instance's writer: it logs through
// slog.Default and builds its 500 body with the formatter registered with
// RegisterNewAPIResponseFormatter. New binds its logger and formatter to it,
// so an instance's built-in writer logs through the instance logger and
// builds the 500 body with the instance formatter instead. A nil formatted
// response is logged as "response cannot be written" and answered with that
// 500 body. Both send the Content-Type set in the formatted Headers (any key
// case; an empty value counts as unset), else application/json; one a
// handler set on w.Header() is overwritten.
type ResponseWriter struct {
	logger    *slog.Logger
	formatter core.APIResponseFormatter
}
