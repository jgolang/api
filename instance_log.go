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
	"net/url"
	"strings"
	"time"

	"github.com/jgolang/api/core"
)

// instanceSensitiveLogKeys returns a fresh list of the keys an instance from
// New masks: the v0 list, cookie and session, then added.
func instanceSensitiveLogKeys(added []string) []string {
	keys := append(v0SensitiveLogKeys(), "cookie", "session")
	return append(keys, added...)
}

// logs reports whether a writes records at level. The default instance and
// an instance without WithLogger never do.
func (a *API) logs(ctx context.Context, level slog.Level) bool {
	return a.logger != nil && a.logger.Enabled(ctx, level)
}

// logError reports err, met while serving r. The default instance passes it
// to PrintError as v0 does; an instance from New writes an error record with
// message msg through its logger, if it has one.
func (a *API) logError(r *http.Request, msg string, err error) {
	if !a.fromNew {
		PrintError(err)
		return
	}
	if !a.logs(r.Context(), slog.LevelError) {
		return
	}
	a.logger.LogAttrs(r.Context(), slog.LevelError, msg, slog.Any("error", err))
}

// logRequest logs the request ProcessRequest received. The default instance
// writes the v0 line through LogRequest, body included when LogRequestBody
// is set; an instance from New never logs the body and masks the query and
// form values whose key is sensitive.
func (a *API) logRequest(r *http.Request, eventID string, rawBody []byte) {
	if !a.fromNew {
		LogRequest(r.Method, r.RequestURI, eventID, r.Form.Encode(), r.Header, rawBody)
		return
	}
	if !a.logs(r.Context(), slog.LevelInfo) {
		return
	}
	attrs := []slog.Attr{
		slog.String("event_id", eventID),
		slog.String("method", r.Method),
		slog.String("uri", redactURI(r.RequestURI, a.sensitiveLogKeys)),
	}
	if form := r.Form.Encode(); form != "" {
		attrs = append(attrs, slog.String("form", redactQuery(form, a.sensitiveLogKeys)))
	}
	if len(r.Header) != 0 {
		attrs = append(attrs, slog.Any("headers", redactHeaders(r.Header, a.sensitiveLogKeys)))
	}
	a.logger.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
}

// logResponse logs the response ProcessRequest wrote, as logRequest does.
func (a *API) logResponse(r *http.Request, eventID string, rec *httptest.ResponseRecorder, duration time.Duration) {
	if !a.fromNew {
		LogResponseWithDuration(eventID, rec, duration)
		return
	}
	if !a.logs(r.Context(), slog.LevelInfo) {
		return
	}
	a.logger.LogAttrs(r.Context(), slog.LevelInfo, "response",
		slog.String("event_id", eventID),
		slog.String("method", r.Method),
		slog.String("uri", redactURI(r.RequestURI, a.sensitiveLogKeys)),
		slog.Int("status", rec.Code),
		slog.String("status_text", http.StatusText(rec.Code)),
		slog.Int64("duration_ms", duration.Milliseconds()),
	)
}

// logPanic logs a panic ProcessRequest recovered. The default instance keeps
// the v0 PrintError calls; an instance from New logs the panic value, not
// the request.
func (a *API) logPanic(r *http.Request, eventID string, recovered any) {
	if !a.fromNew {
		if err, ok := recovered.(error); ok {
			PrintError(err)
		} else {
			PrintError("Not response: ", fmt.Sprintf("%v", r))
		}
		return
	}
	if !a.logs(r.Context(), slog.LevelError) {
		return
	}
	a.logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
		slog.String("event_id", eventID),
		slog.Any("panic", recovered),
	)
}

// redactURI masks the query values of uri whose key is sensitive.
func redactURI(uri string, sensitiveKeys []string) string {
	path, query, found := strings.Cut(uri, "?")
	if !found || query == "" {
		return uri
	}
	return path + "?" + redactQuery(query, sensitiveKeys)
}

// redactQuery masks the values of rawQuery whose unescaped key is sensitive.
// It rebuilds the query part by part, so the other parameters keep their
// order and encoding; a part without "=" has no value and is kept.
func redactQuery(rawQuery string, sensitiveKeys []string) string {
	if rawQuery == "" {
		return rawQuery
	}
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		rawKey, _, hasValue := strings.Cut(part, "=")
		if !hasValue {
			continue
		}
		key, err := url.QueryUnescape(rawKey)
		if err != nil {
			key = rawKey
		}
		if isSensitiveKey(key, sensitiveKeys) {
			parts[i] = rawKey + "=" + redactedLogValue
		}
	}
	return strings.Join(parts, "&")
}

// writeForInstance is the Write of an instance's built-in writer. It encodes
// the body before writing anything, so a body that cannot be encoded (e.g. a
// NaN float) is logged and answered with 500 and the instance formatter's
// internal server error body instead of exiting the process.
func (writer ResponseWriter) writeForInstance(response *core.ResponseFormatted, w http.ResponseWriter) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(response.Body); err != nil {
		writer.logError("response cannot be encoded", err)
		writer.writeEncodeFailure(response.Headers, w)
		return
	}
	writer.writeEncoded(response.Headers, response.HTTPStatusCode, body.Bytes(), w)
}

// writeEncodeFailure answers 500 with the internal server error body built by
// the instance's formatter, keeping the response headers. The body carries no
// event ID or token, since the request is not available here. If that body
// cannot be encoded either, it answers 500 with no body.
func (writer ResponseWriter) writeEncodeFailure(headers map[string]string, w http.ResponseWriter) {
	failure := Error500()
	formatted := writer.formatter.Format(core.ResponseData{
		Title:          failure.Title,
		Message:        failure.Message,
		ResponseCode:   failure.ResponseCode,
		HTTPStatusCode: failure.HTTPStatusCode,
		ResponseType:   ErrorType,
	})
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(formatted.Body); err != nil {
		writer.logError("response cannot be encoded", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.writeEncoded(headers, http.StatusInternalServerError, body.Bytes(), w)
}

// writeEncoded writes an encoded body. http.ErrBodyNotAllowed is not logged:
// Warning and Informative default to status 100, and the recorder that
// ProcessRequest writes into returns it for their body.
func (writer ResponseWriter) writeEncoded(headers map[string]string, status int, body []byte, w http.ResponseWriter) {
	for key := range headers {
		w.Header().Set(key, headers[key])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
		writer.logError("response cannot be written", err)
	}
}

// logError writes an error record through the instance logger, if any. The
// writer has no request, so the record carries no request context.
func (writer ResponseWriter) logError(msg string, err error) {
	if writer.logger == nil {
		return
	}
	writer.logger.LogAttrs(context.Background(), slog.LevelError, msg, slog.Any("error", err))
}
