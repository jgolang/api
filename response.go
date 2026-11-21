package api

import (
	"bytes"
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
type ResponseWriter struct{}

// Write the API response in screen. The body is encoded before anything is
// written, so a body that cannot be encoded (e.g. a NaN float) is reported
// through PrintError and answered with the default internal server error.
func (writer ResponseWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(response.Body); err != nil {
		PrintError(err)
		writeEncodeFailure(w)
		return
	}
	for key := range response.Headers {
		w.Header().Set(key, response.Headers[key])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.HTTPStatusCode)
	if _, err := w.Write(body.Bytes()); err != nil {
		PrintError(err)
	}
}

// writeEncodeFailure answers 500 with the default internal server error body.
// It holds only strings, so encoding it cannot fail.
func writeEncodeFailure(w http.ResponseWriter) {
	failure := Error500()
	formatted := ResponseFormatter{}.Format(core.ResponseData{
		Title:          failure.Title,
		Message:        failure.Message,
		ResponseCode:   failure.ResponseCode,
		HTTPStatusCode: failure.HTTPStatusCode,
		ResponseType:   ErrorType,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(formatted.HTTPStatusCode)
	if err := json.NewEncoder(w).Encode(formatted.Body); err != nil {
		PrintError(err)
	}
}
