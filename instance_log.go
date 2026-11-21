package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jgolang/api/core"
)

// instanceSensitiveLogKeys returns a fresh list of the keys an instance from
// New masks: the built-in default list, then added.
func instanceSensitiveLogKeys(added []string) []string {
	return append(defaultSensitiveLogKeys(), added...)
}

// log returns the logger a writes through: its own, or slog.Default at call
// time on the default instance.
func (a *API) log() *slog.Logger {
	if a.logger == nil {
		return slog.Default()
	}
	return a.logger
}

// sensitiveKeys returns the keys a masks: its own, or on the default
// instance the default keys plus SensitiveLogKeys at call time, so the
// variable can add keys but never remove a default one.
func (a *API) sensitiveKeys() []string {
	if a.sensitiveLogKeys == nil {
		return append(defaultSensitiveLogKeys(), SensitiveLogKeys...)
	}
	return a.sensitiveLogKeys
}

// logs reports whether a writes records at level. An instance without
// WithLogger never does.
func (a *API) logs(ctx context.Context, level slog.Level) bool {
	return a.log().Enabled(ctx, level)
}

// logError writes an error record with message msg for err, met while
// serving r.
func (a *API) logError(r *http.Request, msg string, err error) {
	if !a.logs(r.Context(), slog.LevelError) {
		return
	}
	a.log().LogAttrs(r.Context(), slog.LevelError, msg, slog.Any("error", err))
}

// logNamedError reports err from reading the value of the header, query
// parameter or route variable called name. It adds the name as "key"; when
// the name is sensitive it logs only the kind of error, since errors such as
// strconv's quote the value.
func (a *API) logNamedError(r *http.Request, msg, name string, err error) {
	if !a.logs(r.Context(), slog.LevelError) {
		return
	}
	errAttr := slog.Any("error", err)
	if isSensitiveKey(name, a.sensitiveKeys()) {
		errAttr = slog.String("error", sensitiveValueError(err))
	}
	a.log().LogAttrs(r.Context(), slog.LevelError, msg, slog.String("key", name), errAttr)
}

// sensitiveValueError describes err without the value it was reading: the
// reason of a strconv.NumError, never its Num, and a fixed text otherwise.
func sensitiveValueError(err error) string {
	var numErr *strconv.NumError
	if errors.As(err, &numErr) && numErr.Err != nil {
		return numErr.Err.Error()
	}
	return "invalid value"
}

// logRequest logs the request ProcessRequest received. It never logs the
// body or the parsed form (it may come from the body) and masks the query
// values whose key is sensitive, in uri and in header values that carry a URL
// (see redactHeaders).
func (a *API) logRequest(r *http.Request, eventID string) {
	if !a.logs(r.Context(), slog.LevelInfo) {
		return
	}
	sensitiveKeys := a.sensitiveKeys()
	attrs := []slog.Attr{
		slog.String("event_id", eventID),
		slog.String("method", r.Method),
		slog.String("uri", redactURI(r.RequestURI, sensitiveKeys)),
	}
	if len(r.Header) != 0 {
		attrs = append(attrs, slog.Any("headers", redactHeaders(r.Header, sensitiveKeys)))
	}
	a.log().LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
}

// logResponse logs the response ProcessRequest wrote, as logRequest does.
func (a *API) logResponse(r *http.Request, eventID string, rec *httptest.ResponseRecorder, duration time.Duration) {
	if !a.logs(r.Context(), slog.LevelInfo) {
		return
	}
	a.log().LogAttrs(r.Context(), slog.LevelInfo, "response",
		slog.String("event_id", eventID),
		slog.String("method", r.Method),
		slog.String("uri", redactURI(r.RequestURI, a.sensitiveKeys())),
		slog.Int("status", rec.Code),
		slog.String("status_text", http.StatusText(rec.Code)),
		slog.Int64("duration_ms", duration.Milliseconds()),
	)
}

// logPanic logs a panic ProcessRequest recovered, with the event ID, the
// panic value's Go type as "panic_type" and, as "panic", the value itself
// only for a runtime.Error. Any other value (panic(err) of a failed parse,
// a formatted string) may carry request data, and masking by key cannot
// apply to an opaque value, so it is logged as redactedLogValue. A panic
// with http.ErrAbortHandler is a deliberate abort, so it is logged at debug
// level as "handler aborted", under the same rule.
func (a *API) logPanic(r *http.Request, eventID string, recovered any) {
	level, msg := slog.LevelError, "panic recovered"
	if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		level, msg = slog.LevelDebug, "handler aborted"
	}
	if !a.logs(r.Context(), level) {
		return
	}
	panicAttr := slog.String("panic", redactedLogValue)
	if runtimeErr, ok := recovered.(runtime.Error); ok {
		panicAttr = slog.Any("panic", runtimeErr)
	}
	a.log().LogAttrs(r.Context(), level, msg,
		slog.String("event_id", eventID),
		panicAttr,
		slog.String("panic_type", fmt.Sprintf("%T", recovered)),
	)
}

// Write the API response in screen. It encodes the body before writing
// anything, so a body that cannot be encoded (e.g. a NaN float) is logged as
// "response cannot be encoded" and answered with 500 and the formatter's
// internal server error body (see writeEncodeFailure) instead of exiting the
// process. If the handler already wrote a status, that status stays and the
// error body is written under it.
//
// http.ErrBodyNotAllowed from writing the body is not reported: Warning and
// Informative default to status 100, which a server sends as informational
// before delivering the body with 200, while the recorder that ProcessRequest
// writes into returns this error.
func (writer ResponseWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(response.Body); err != nil {
		writer.logError("response cannot be encoded", err)
		writer.writeEncodeFailure(response.Headers, w)
		return
	}
	writer.writeEncoded(response.Headers, response.HTTPStatusCode, body.Bytes(), w)
}

// writeEncodeFailure answers 500 with the internal server error body built by
// the writer's formatter, keeping the response headers except the ones
// that described the failed body (withoutBodyHeaders); the error body's own
// Content-Type applies. The body carries no event ID or token, since the
// request is not available here. If the formatter returns no body, or it
// cannot be encoded either, it answers 500 with no body and no Content-Type,
// still with the other kept headers.
func (writer ResponseWriter) writeEncodeFailure(headers map[string]string, w http.ResponseWriter) {
	headers = withoutBodyHeaders(headers)
	failure := Error500()
	formatted := writer.responseFormatter().Format(core.ResponseData{
		Title:          failure.Title,
		Message:        failure.Message,
		ResponseCode:   failure.ResponseCode,
		HTTPStatusCode: failure.HTTPStatusCode,
		ResponseType:   ErrorType,
	})
	if formatted == nil {
		writeBare500(headers, w)
		return
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(formatted.Body); err != nil {
		writer.logError("response cannot be encoded", err)
		writeBare500(headers, w)
		return
	}
	if contentType := formattedContentType(formatted.Headers); contentType != "" {
		headers["Content-Type"] = contentType
	}
	writer.writeEncoded(headers, http.StatusInternalServerError, body.Bytes(), w)
}

// writeBare500 answers 500 with no body and no Content-Type, with headers.
func writeBare500(headers map[string]string, w http.ResponseWriter) {
	setHeaders(headers, w)
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusInternalServerError)
}

// writeEncoded writes an encoded body. Its Content-Type comes only from the
// formatted headers, application/json when they set none: one the handler
// put on w.Header() earlier (say text/html) must never label a JSON body.
// http.ErrBodyNotAllowed is not logged: Warning and Informative default to
// status 100, and the recorder that ProcessRequest writes into returns it for
// their body.
func (writer ResponseWriter) writeEncoded(headers map[string]string, status int, body []byte, w http.ResponseWriter) {
	setHeaders(headers, w)
	contentType := formattedContentType(headers)
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
		writer.logError("response cannot be written", err)
	}
}

// formattedContentType returns the Content-Type set in headers, whatever the
// key's case, or "" when they set none. An empty value counts as unset.
func formattedContentType(headers map[string]string) string {
	for key, value := range headers {
		if strings.EqualFold(key, "Content-Type") && value != "" {
			return value
		}
	}
	return ""
}

// withoutBodyHeaders returns a copy of headers without the headers that
// describe the body (see isBodyHeader), for a response whose body is
// replaced by an error body.
func withoutBodyHeaders(headers map[string]string) map[string]string {
	kept := make(map[string]string, len(headers)+1)
	for key, value := range headers {
		if !isBodyHeader(key) {
			kept[key] = value
		}
	}
	return kept
}

// isBodyHeader reports whether key, whatever its case, names a header that
// describes a response body.
func isBodyHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Content-Type", "Content-Encoding", "Content-Length", "Content-Range",
		"Content-Disposition", "Content-Language", "Content-Location", "Etag", "Last-Modified":
		return true
	}
	return false
}

func setHeaders(headers map[string]string, w http.ResponseWriter) {
	for key := range headers {
		w.Header().Set(key, headers[key])
	}
}

// logError writes an error record through the writer's logger. The writer
// has no request, so the record carries no request context.
func (writer ResponseWriter) logError(msg string, err error) {
	logger := writer.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.LogAttrs(context.Background(), slog.LevelError, msg, slog.Any("error", err))
}

// responseFormatter returns the formatter that builds the writer's 500 body:
// the instance's, or the registered one on the default instance's writer.
func (writer ResponseWriter) responseFormatter() core.APIResponseFormatter {
	if writer.formatter == nil {
		return responseFormatter
	}
	return writer.formatter
}
