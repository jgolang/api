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
	"strconv"
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

// logNamedError reports err from reading the value of the header, query
// parameter or route variable called name. The default instance passes it to
// PrintError as v0 does. An instance from New adds the name as "key"; when
// the name is sensitive it logs only the kind of error, since errors such as
// strconv's quote the value.
func (a *API) logNamedError(r *http.Request, msg, name string, err error) {
	if !a.fromNew {
		PrintError(err)
		return
	}
	if !a.logs(r.Context(), slog.LevelError) {
		return
	}
	errAttr := slog.Any("error", err)
	if isSensitiveKey(name, a.sensitiveLogKeys) {
		errAttr = slog.String("error", sensitiveValueError(err))
	}
	a.logger.LogAttrs(r.Context(), slog.LevelError, msg, slog.String("key", name), errAttr)
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

// logRequest logs the request ProcessRequest received. The default instance
// writes the v0 line through LogRequest, body included when LogRequestBody
// is set; an instance from New never logs the body or the parsed form (it
// may come from the body) and masks the query values whose key is sensitive.
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
// the request. A panic with http.ErrAbortHandler is a deliberate abort, so it
// is logged at debug level as "handler aborted".
func (a *API) logPanic(r *http.Request, eventID string, recovered any) {
	if !a.fromNew {
		if err, ok := recovered.(error); ok {
			PrintError(err)
		} else {
			PrintError("Not response: ", fmt.Sprintf("%v", r))
		}
		return
	}
	level, msg := slog.LevelError, "panic recovered"
	if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		level, msg = slog.LevelDebug, "handler aborted"
	}
	if !a.logs(r.Context(), level) {
		return
	}
	a.logger.LogAttrs(r.Context(), level, msg,
		slog.String("event_id", eventID),
		slog.Any("panic", recovered),
	)
}

// redactURI masks the query values of uri whose key is sensitive and, in an
// absolute-form URI (scheme://user:pass@host/...), the user information.
// Path segments are not masked.
func redactURI(uri string, sensitiveKeys []string) string {
	// The user information goes first: a raw "?" in a password would
	// otherwise be taken for the start of the query.
	path, query, found := strings.Cut(redactUserInfo(uri), "?")
	if !found || query == "" {
		if found {
			return path + "?"
		}
		return path
	}
	return path + "?" + redactQuery(query, sensitiveKeys)
}

// redactUserInfo replaces the user information of an absolute-form request
// target with redactedLogValue: everything from "://" to the last "@" before
// the first "#", so a raw "/" or "?" in a password is masked too.
func redactUserInfo(target string) string {
	scheme, rest, found := strings.Cut(target, "://")
	if !found || strings.ContainsAny(scheme, "/?#") {
		return target
	}
	beforeFragment, _, _ := strings.Cut(rest, "#")
	at := strings.LastIndex(beforeFragment, "@")
	if at < 0 {
		return target
	}
	return scheme + "://" + redactedLogValue + rest[at:]
}

// redactQuery masks the values of rawQuery whose unescaped key is sensitive,
// and the values whose key cannot be unescaped. It rebuilds the query part by
// part, splitting on "&" and ";" and keeping each separator, so the other
// parameters keep their order and encoding; a part without "=" has no value
// and is kept.
func redactQuery(rawQuery string, sensitiveKeys []string) string {
	if rawQuery == "" {
		return rawQuery
	}
	var redacted strings.Builder
	redacted.Grow(len(rawQuery))
	for rawQuery != "" {
		end := strings.IndexAny(rawQuery, "&;")
		part, separator := rawQuery, ""
		if end >= 0 {
			part, separator = rawQuery[:end], rawQuery[end:end+1]
			rawQuery = rawQuery[end+1:]
		} else {
			rawQuery = ""
		}
		redacted.WriteString(redactQueryPart(part, sensitiveKeys))
		redacted.WriteString(separator)
	}
	return redacted.String()
}

func redactQueryPart(part string, sensitiveKeys []string) string {
	rawKey, _, hasValue := strings.Cut(part, "=")
	if !hasValue {
		return part
	}
	key, err := url.QueryUnescape(rawKey)
	if err != nil || isSensitiveKey(key, sensitiveKeys) {
		return rawKey + "=" + redactedLogValue
	}
	return part
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
// the instance's formatter, keeping the response headers except the ones
// that described the failed body (withoutBodyHeaders); the error body's own
// Content-Type applies. The body carries no event ID or token, since the
// request is not available here. If the formatter returns no body, or it
// cannot be encoded either, it answers 500 with no body and no Content-Type,
// still with the other kept headers.
func (writer ResponseWriter) writeEncodeFailure(headers map[string]string, w http.ResponseWriter) {
	headers = withoutBodyHeaders(headers)
	failure := Error500()
	formatted := writer.formatter.Format(core.ResponseData{
		Title:          failure.Title,
		Message:        failure.Message,
		ResponseCode:   failure.ResponseCode,
		HTTPStatusCode: failure.HTTPStatusCode,
		ResponseType:   ErrorType,
	})
	if formatted == nil {
		writer.writeBare500(headers, w)
		return
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(formatted.Body); err != nil {
		writer.logError("response cannot be encoded", err)
		writer.writeBare500(headers, w)
		return
	}
	if contentType := formattedContentType(formatted.Headers); contentType != "" {
		headers["Content-Type"] = contentType
	}
	writer.writeEncoded(headers, http.StatusInternalServerError, body.Bytes(), w)
}

func (writer ResponseWriter) writeBare500(headers map[string]string, w http.ResponseWriter) {
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

// logError writes an error record through the instance logger, if any. The
// writer has no request, so the record carries no request context.
func (writer ResponseWriter) logError(msg string, err error) {
	if writer.logger == nil {
		return
	}
	writer.logger.LogAttrs(context.Background(), slog.LevelError, msg, slog.Any("error", err))
}
