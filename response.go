package api

import (
	"encoding/json"
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
		return r.Header.Get(SecurityTokenHeaderKey)
	}
	if requestData.SecurityToken != "" {
		return requestData.SecurityToken
	}
	return r.Header.Get(SecurityTokenHeaderKey)
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
type ResponseWriter struct{}

// Write the API response in screen.
func (writer ResponseWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	for key := range response.Headers {
		w.Header().Set(key, response.Headers[key])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.HTTPStatusCode)
	err := json.NewEncoder(w).Encode(response.Body)
	if err != nil {
		Fatal(err)
	}
	return
}
