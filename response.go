package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jgolang/api/core"
)

// Response wrapper to generate core responses.
type Response interface {
	Write(w http.ResponseWriter, r *http.Request)
}

func getSecurityToken(r *http.Request) string {
	requestData, err := GetRequestContext(r)
	if err != nil {
		if token := r.Header.Get(SecurityTokenHeaderKey); token != "" {
			return token
		}
		return bearerTokenFromHeader(r.Header.Get("Authorization"))
	}
	if requestData.SecurityToken != "" {
		return requestData.SecurityToken
	}
	if token := r.Header.Get(SecurityTokenHeaderKey); token != "" {
		return token
	}
	return bearerTokenFromHeader(r.Header.Get("Authorization"))
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
// The zero value calls Fatal when a body cannot be encoded, as v0 does; New
// binds its logger and formatter to it, so an instance's built-in writer
// logs the failure and answers 500 instead.
type ResponseWriter struct {
	// fromNew is set by New; false is the default instance's v0 behavior.
	fromNew   bool
	logger    *slog.Logger
	formatter core.APIResponseFormatter
}

// Write the API response in screen.
func (writer ResponseWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	if writer.fromNew {
		writer.writeForInstance(response, w)
		return
	}
	// A Content-Type set in Headers wins over the JSON default.
	w.Header().Set("Content-Type", "application/json")
	for key := range response.Headers {
		w.Header().Set(key, response.Headers[key])
	}
	w.WriteHeader(response.HTTPStatusCode)
	err := json.NewEncoder(w).Encode(response.Body)
	if err != nil {
		Fatal(err)
	}
}
