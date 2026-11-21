package api

import (
	"bytes"
	"encoding/json"
	"errors"
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
// The zero value is the default instance's writer, described on Write; New
// binds its logger and formatter to it, so an instance's built-in writer
// logs through the instance logger and builds the 500 body with the
// instance formatter instead. Both send the Content-Type set in the
// formatted Headers (any key case; an empty value counts as unset), else
// application/json; one a handler set on w.Header() is overwritten.
type ResponseWriter struct {
	// fromNew is set by New; false is the default instance's v0 behavior.
	fromNew   bool
	logger    *slog.Logger
	formatter core.APIResponseFormatter
}

// Write the API response in screen. On the default instance, the body is
// encoded before anything is written. A body that cannot be encoded (e.g. a
// NaN float) is reported through PrintError and answered with 500 and the
// internal server error body, built by the registered response formatter,
// with the response headers kept. The error body carries no event ID or token, since the
// request is not available here. If the handler already wrote a status, that
// status stays and the error body is written under it.
//
// http.ErrBodyNotAllowed from writing the body is not reported: Warning and
// Informative default to status 100, which a server sends as informational
// before delivering the body with 200, while the recorder that ProcessRequest
// writes into returns this error.
func (writer ResponseWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	if writer.fromNew {
		writer.writeForInstance(response, w)
		return
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(response.Body); err != nil {
		PrintError(err)
		writeEncodeFailure(response.Headers, w)
		return
	}
	writeEncoded(response.Headers, response.HTTPStatusCode, body.Bytes(), w)
}

// writeEncodeFailure answers 500 with the internal server error body built by
// the registered formatter. If that body cannot be encoded either, it answers
// 500 with no body.
func writeEncodeFailure(headers map[string]string, w http.ResponseWriter) {
	failure := Error500()
	formatted := responseFormatter.Format(core.ResponseData{
		Title:          failure.Title,
		Message:        failure.Message,
		ResponseCode:   failure.ResponseCode,
		HTTPStatusCode: failure.HTTPStatusCode,
		ResponseType:   ErrorType,
	})
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(formatted.Body); err != nil {
		PrintError(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeEncoded(headers, http.StatusInternalServerError, body.Bytes(), w)
}

func writeEncoded(headers map[string]string, status int, body []byte, w http.ResponseWriter) {
	for key := range headers {
		w.Header().Set(key, headers[key])
	}
	contentType := formattedContentType(headers)
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
		PrintError(err)
	}
}
