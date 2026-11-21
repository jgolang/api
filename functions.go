package api

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/jgolang/api/core"
)

// PrintError wrapper function. Since v1.0.0 the package no longer calls it:
// the default instance logs errors through slog.Default and an instance from
// New through its WithLogger logger.
//
// Deprecated: it is ignored; use api.New(api.WithLogger(...)) or
// slog.SetDefault instead.
var PrintError func(...interface{}) = log.Print

// Print wrapper function. Since v1.0.0 only the deprecated LogRequest,
// LogResponse and LogResponseWithDuration call it: the default instance logs
// requests and responses through slog.Default and an instance from New
// through its WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) or slog.SetDefault instead.
var Print func(string, ...interface{}) = log.Printf

// Fatal wrapper function. The package no longer calls it: a response that
// cannot be encoded is logged and answered with 500.
//
// Deprecated: it is ignored; use api.New(api.WithLogger(...)) or
// slog.SetDefault instead.
var Fatal func(...interface{}) = log.Fatal

// GetHeaderValueString gets header value as string.
func GetHeaderValueString(key string, r *http.Request) (string, Response) {
	value, err := api.GetHeaderValueString(key, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "header value not obtained", key, err)
		return value, Error{
			Title:   "Error getting header value!",
			Message: fmt.Sprintf("The %v key header has not been obtained", key),
		}
	}
	return value, nil
}

// GetHeaderValueInt gets header value as integer.
func GetHeaderValueInt(key string, r *http.Request) (int, Response) {
	value, err := api.GetHeaderValueInt(key, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "header value not obtained", key, err)
		return value, Error{
			Title:   "Error getting header value type Int!",
			Message: fmt.Sprintf("The %v key header has not been obtained", key),
		}
	}
	return value, nil
}

// GetHeaderValueInt64 gets header value as integer 64.
func GetHeaderValueInt64(key string, r *http.Request) (int64, Response) {
	value, err := api.GetHeaderValueInt64(key, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "header value not obtained", key, err)
		return value, Error{
			Title:   "Error getting header value type Int64!",
			Message: fmt.Sprintf("The %v key header has not been obtained", key),
		}
	}
	return value, nil
}

// GetHeaderValueFloat64 gets header value as float 64.
func GetHeaderValueFloat64(key string, r *http.Request) (float64, Response) {
	value, err := api.GetHeaderValueFloat64(key, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "header value not obtained", key, err)
		return value, Error{
			Title:   "Error getting header value type Float64!",
			Message: fmt.Sprintf("The %v key header has not been obtained", key),
		}
	}
	return value, nil
}

// GetHeaderValueBool gets header values as bool.
func GetHeaderValueBool(key string, r *http.Request) (bool, Response) {
	value, err := api.GetHeaderValueBool(key, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "header value not obtained", key, err)
		return value, Error{
			Title:   "Error getting header value type Bool!",
			Message: fmt.Sprintf("The %v key header has not been obtained", key),
		}
	}
	return value, nil
}

// GetRouteVarValueString gets route variable value as string.
func GetRouteVarValueString(urlVarName string, r *http.Request) (string, Response) {
	value, err := api.GetRouteVarValueString(urlVarName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "route var not obtained", urlVarName, err)
		return value, Error{
			Title:   "Error getting route var!",
			Message: fmt.Sprintf("The route var %v has not been obtained", urlVarName),
		}
	}
	return value, nil
}

// GetRouteVarValueInt gets route variable value as integer.
func GetRouteVarValueInt(urlVarName string, r *http.Request) (int, Response) {
	value, err := api.GetRouteVarValueInt(urlVarName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "route var not obtained", urlVarName, err)
		return 0, Error{
			Title:   "Error getting route var type Int",
			Message: fmt.Sprintf("The route var %v has not been obtained", urlVarName),
		}
	}
	return value, nil
}

// GetRouteVarValueInt64 gets route variable value as integer 64.
func GetRouteVarValueInt64(urlVarName string, r *http.Request) (int64, Response) {
	value, err := api.GetRouteVarValueInt64(urlVarName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "route var not obtained", urlVarName, err)
		return value, Error{
			Title:   "Error getting route var type Int64",
			Message: fmt.Sprintf("The route var %v has not been obtained", urlVarName),
		}
	}
	return value, nil
}

// GetRouteVarValueFloat64 gets route variable value as float 64.
func GetRouteVarValueFloat64(urlVarName string, r *http.Request) (float64, Response) {
	value, err := api.GetRouteVarValueFloat64(urlVarName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "route var not obtained", urlVarName, err)
		return value, Error{
			Title:   "Error getting route var type Float64",
			Message: fmt.Sprintf("The route var %v has not been obtained", urlVarName),
		}
	}
	return value, nil
}

// GetRouteVarValueBool gets route variable value as bool.
func GetRouteVarValueBool(urlVarName string, r *http.Request) (bool, Response) {
	value, err := api.GetRouteVarValueBool(urlVarName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "route var not obtained", urlVarName, err)
		return value, Error{
			Title:   "Error getting route var type Bool",
			Message: fmt.Sprintf("The route var %v has not been obtained", urlVarName),
		}
	}
	return value, nil
}

// GetQueryParamValueString gets query param value as string.
//
// See also the Query* helpers (QueryString, QueryInt, QueryBool, QueryEnum),
// which tell an absent parameter from an invalid one.
//
// Deprecated: use QueryString, which tells an absent parameter from an invalid
// one and returns an error that WriteError answers with 400.
func GetQueryParamValueString(queryParamName string, r *http.Request) (string, Response) {
	value, err := api.GetQueryParamValueString(queryParamName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "query param not obtained", queryParamName, err)
		return value, Error{
			Title:   "Error getting query param!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil

}

// GetQueryParamValueInt gets query param value as integer.
//
// See also the Query* helpers (QueryString, QueryInt, QueryBool, QueryEnum),
// which tell an absent parameter from an invalid one.
//
// Deprecated: use QueryInt, which tells an absent parameter from an invalid
// one and returns an error that WriteError answers with 400.
func GetQueryParamValueInt(queryParamName string, r *http.Request) (int, Response) {
	value, err := api.GetQueryParamValueInt(queryParamName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "query param not obtained", queryParamName, err)
		return value, Error{
			Title:   "Error getting query param type Int!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// GetQueryParamValueInt64 gets query param value as integer 64.
//
// See also the Query* helpers (QueryString, QueryInt, QueryBool, QueryEnum),
// which tell an absent parameter from an invalid one.
func GetQueryParamValueInt64(queryParamName string, r *http.Request) (int64, Response) {
	value, err := api.GetQueryParamValueInt64(queryParamName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "query param not obtained", queryParamName, err)
		return value, Error{
			Title:   "Error getting query param type Int64!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// GetQueryParamValueFloat64 gets query param value as float 64.
//
// See also the Query* helpers (QueryString, QueryInt, QueryBool, QueryEnum),
// which tell an absent parameter from an invalid one.
func GetQueryParamValueFloat64(queryParamName string, r *http.Request) (float64, Response) {
	value, err := api.GetQueryParamValueFloat64(queryParamName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "query param not obtained", queryParamName, err)
		return value, Error{
			Title:   "Error getting query param type Float64!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// GetQueryParamValueBool gets param value as bool.
//
// See also the Query* helpers (QueryString, QueryInt, QueryBool, QueryEnum),
// which tell an absent parameter from an invalid one.
//
// Deprecated: use QueryBool, which tells an absent parameter from an invalid
// one and returns an error that WriteError answers with 400.
func GetQueryParamValueBool(queryParamName string, r *http.Request) (bool, Response) {
	value, err := api.GetQueryParamValueBool(queryParamName, r)
	if err != nil {
		instanceFrom(r).logNamedError(r, "query param not obtained", queryParamName, err)
		return value, Error{
			Title:   "Error getting query param type Bool!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// UnmarshalBody parses and validates request body to a struct. A body over
// the instance's limit (10 MiB without an instance) is answered with 413.
// Without an instance in r, it replaces r.Body with an http.MaxBytesReader;
// it has no ResponseWriter, so the server is not told to close the
// connection on overflow (ProcessRequest and RequestBody pass theirs).
func UnmarshalBody(v interface{}, r *http.Request) Response {
	boundDefaultBody(nil, r)
	err := api.UnmarshalBody(v, r)
	if err != nil {
		a := instanceFrom(r)
		a.logBodyError(r, "request body not unmarshaled", err, true)
		if bodyTooLarge(err) {
			return bodyTooLargeError()
		}
		return Error{
			Title:   "Not unmarshal JSON struct!",
			Message: "Error when unmarshal JSON structure",
		}
	}
	if errMsg, err := api.ValidateParams(v); err != nil {
		return Error{
			Message:      errMsg,
			ResponseCode: ResponseCodes.InvalidParams,
		}
	}
	return nil
}

// GetContextValue gets requesst context value from context key.
func GetContextValue(key interface{}, r *http.Request) interface{} {
	return r.Context().Value(key)
}

// SetContextValue sets requesst context value from context key.
func SetContextValue(key, value interface{}, r *http.Request) *http.Request {
	ctx := context.WithValue(
		r.Context(),
		key,
		value,
	)
	return r.WithContext(ctx)
}

// GetRequestContext gets request data from http request context.
// This useful when you set Request type of core.RequestDataContext in http request context
// in a middleware implementation.
// Returns a core.RequestDataContext struct from api.RequestDataContextContextKey key.
func GetRequestContext(r *http.Request) (*RequestContext, error) {
	value := r.Context().Value(RequestDataContextContextKey)
	requestData, valid := value.(*RequestContext)
	if valid {
		return requestData, nil
	}
	return nil, fmt.Errorf("context requestData not found")
}

// UpdateRequestContext update request context.
func UpdateRequestContext(requestData *RequestContext, r *http.Request) *http.Request {
	return SetContextValue(
		RequestDataContextContextKey,
		requestData,
		r,
	)
}

// RContext gets request data from http request context.
// This useful when you set Request type of core.RequestDataContext in http request context
// in a middleware implementation.
// Returns a core.RequestDataContext struct from api.RequestDataContextContextKey key.
func RContext(ctx context.Context) (*RequestContext, error) {
	value := ctx.Value(RequestDataContextContextKey)
	requestData, valid := value.(*RequestContext)
	if valid {
		return requestData, nil
	}
	return nil, fmt.Errorf("context requestData not found")
}

// Context returns the request data of ctx: the RequestContext itself, or the
// one ProcessRequest stored in a request context. Otherwise, including a nil
// ctx, it returns an empty RequestContext over ctx (context.Background for
// nil). It returns nil only for a typed nil *RequestContext.
func Context(ctx context.Context) *RequestContext {
	switch v := ctx.(type) {
	case nil:
		return &RequestContext{
			RequestDataContext: &core.RequestDataContext{
				Context: context.Background(),
			},
		}
	case *RequestContext:
		return ctx.(*RequestContext)
	case RequestContext:
		rctx := ctx.(RequestContext)
		return &rctx
	case context.Context:
		// ProcessRequest stores the request data under this key, and the value
		// survives every context derived from the request context.
		if rctx, ok := v.Value(RequestDataContextContextKey).(*RequestContext); ok {
			return rctx
		}
		return &RequestContext{
			RequestDataContext: &core.RequestDataContext{
				Context: v,
			},
		}
	default:
		return &RequestContext{
			RequestDataContext: &core.RequestDataContext{
				Context: v,
			},
		}
	}
}

const redactedLogValue = "[REDACTED]"

// PrintFullEvent set true value for allow print full event request. It also
// logs a non-JSON request body as is instead of its size.
//
// Deprecated: it affects only the deprecated LogRequest, LogResponse and
// LogResponseWithDuration; ProcessRequest never logs bodies.
var PrintFullEvent bool = false

// LogRequestBody enables request body logging.
//
// Deprecated: it affects only the deprecated LogRequest; ProcessRequest
// never logs bodies.
var LogRequestBody bool = false

// LogResponseBody enables response body logging.
//
// Deprecated: it affects only the deprecated LogResponse and
// LogResponseWithDuration; ProcessRequest never logs bodies.
var LogResponseBody bool = false

// MaxLoggedBodyBytes limits logged request and response bodies when PrintFullEvent is false.
//
// Deprecated: it affects only the deprecated LogRequest, LogResponse and
// LogResponseWithDuration; ProcessRequest never logs bodies.
var MaxLoggedBodyBytes int = 2000

// SensitiveLogKeys defines fields, headers and query parameters that must be
// redacted in logs: by default authorization, token, password, secret,
// credential, apikey, api-key, key, cookie, session, code, signature and sig
// (the last three since v1.1.2). A name is sensitive
// when it contains one of the keys, ignoring case.
// The default instance reads it at call time and masks the built-in keys
// plus these, so it can add keys but no longer remove a built-in one; an
// instance from New masks the built-in keys plus the keys added with
// WithSensitiveLogKeys. The deprecated LogRequest masks only these keys.
//
// Deprecated: use api.New(api.WithSensitiveLogKeys(...)) instead.
var SensitiveLogKeys = defaultSensitiveLogKeys()

// defaultSensitiveLogKeys returns a fresh copy of the built-in sensitive key
// list, so changing SensitiveLogKeys never reaches an instance.
func defaultSensitiveLogKeys() []string {
	return []string{
		"authorization",
		"token",
		"password",
		"secret",
		"credential",
		"apikey",
		"api-key",
		"key",
		"cookie",
		"session",
		"code",
		"signature",
		"sig",
	}
}

// LogRequest prints API request in log. The values of sensitive headers,
// JSON body fields, query parameters in uri and form fields are masked (see
// SensitiveLogKeys), as is the user information of an absolute-form uri.
// Other header values have the query values after each "?" masked the same
// way, and the user information too when they carry a URL (Referer, Origin,
// X-Original-URL, ...). A request body that is not JSON is logged as its
// size unless PrintFullEvent is true. The package no longer calls it: ProcessRequest logs through
// slog.Default, or through the instance's WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) instead.
func LogRequest(method, uri, eventID, form string, headers http.Header, rawBody []byte) {
	format := "REQUEST event_id=%q method=%q uri=%q"
	args := []interface{}{eventID, method, redactURI(uri, SensitiveLogKeys)}
	if form != "" {
		format += " form=%q"
		args = append(args, redactQuery(form, SensitiveLogKeys))
	}
	if len(headers) != 0 {
		format += " headers=%s"
		args = append(args, formatLogValue(sanitizeHeadersForLog(headers)))
	}
	if LogRequestBody && len(rawBody) != 0 {
		format += " body=%q"
		args = append(args, formatRequestBodyForLog(rawBody))
	}
	Print(format, args...)
}

// LogResponse prints API response in log. The package no longer calls it:
// ProcessRequest logs through slog.Default, or through the instance's
// WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) instead.
func LogResponse(eventID string, res *httptest.ResponseRecorder) {
	LogResponseWithDuration(eventID, res, 0)
}

// LogResponseWithDuration prints API response in log including request
// duration. The package no longer calls it: ProcessRequest logs through
// slog.Default, or through the instance's WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) instead.
func LogResponseWithDuration(eventID string, res *httptest.ResponseRecorder, duration time.Duration) {
	format := "RESPONSE event_id=%q status=%d status_text=%q"
	args := []interface{}{eventID, res.Code, http.StatusText(res.Code)}
	if duration > 0 {
		format += " duration_ms=%d"
		args = append(args, duration.Milliseconds())
	}
	rawBody := res.Body.Bytes()
	if LogResponseBody && len(rawBody) != 0 {
		format += " body=%q"
		args = append(args, formatBodyForLog(rawBody))
	}
	Print(format, args...)
}

// formatRequestBodyForLog summarizes a non-JSON request body (a form, XML,
// text) as its size, since its sensitive values cannot be masked by key.
func formatRequestBodyForLog(rawBody []byte) string {
	if !PrintFullEvent && !json.Valid(rawBody) {
		return fmt.Sprintf("[non-JSON body, %d bytes]", len(rawBody))
	}
	return formatBodyForLog(rawBody)
}

func formatBodyForLog(rawBody []byte) string {
	body := sanitizeJSONBodyForLog(rawBody)
	if PrintFullEvent || MaxLoggedBodyBytes <= 0 || len(body) <= MaxLoggedBodyBytes {
		return body
	}
	if MaxLoggedBodyBytes <= 20 {
		return body[:MaxLoggedBodyBytes]
	}
	edgeSize := MaxLoggedBodyBytes / 2
	return body[:edgeSize] + " ***** SKIPPED ***** " + body[len(body)-edgeSize:]
}

func sanitizeJSONBodyForLog(rawBody []byte) string {
	var value interface{}
	if err := json.Unmarshal(rawBody, &value); err != nil {
		return string(rawBody)
	}
	return formatLogValue(sanitizeValueForLog(value))
}

func sanitizeHeadersForLog(headers http.Header) http.Header {
	return redactHeaders(headers, SensitiveLogKeys)
}

// redactHeaders returns a copy of headers with the values of the headers
// whose name matches sensitiveKeys replaced by redactedLogValue, and the other
// values masked by redactHeaderValue.
func redactHeaders(headers http.Header, sensitiveKeys []string) http.Header {
	sanitized := make(http.Header, len(headers))
	for key, values := range headers {
		if isSensitiveKey(key, sensitiveKeys) {
			sanitized[key] = []string{redactedLogValue}
			continue
		}
		redacted := make([]string, len(values))
		for i, value := range values {
			redacted[i] = redactHeaderValue(value, sensitiveKeys)
		}
		sanitized[key] = redacted
	}
	return sanitized
}

func sanitizeValueForLog(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			if isSensitiveLogKey(key) {
				sanitized[key] = redactedLogValue
				continue
			}
			sanitized[key] = sanitizeValueForLog(value)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(typed))
		for i, value := range typed {
			sanitized[i] = sanitizeValueForLog(value)
		}
		return sanitized
	default:
		return value
	}
}

func isSensitiveLogKey(key string) bool {
	return isSensitiveKey(key, SensitiveLogKeys)
}

// isSensitiveKey reports whether key contains one of sensitiveKeys, ignoring
// case.
func isSensitiveKey(key string, sensitiveKeys []string) bool {
	key = strings.ToLower(key)
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(key, strings.ToLower(sensitiveKey)) {
			return true
		}
	}
	return false
}

// redactURI masks the query values of uri whose key is sensitive and the
// user information of an absolute-form (scheme://user:pass@host/...),
// network-path (//user:pass@host/...) or authority-form (CONNECT
// user:pass@host:443) target. Path segments are not masked.
func redactURI(uri string, sensitiveKeys []string) string {
	// The query goes first, cut at the first "?" of the raw uri: masking the
	// user information first would cut at an "@" inside the query (such as
	// "?email=a@b.com&token=..."), lose the "?" and log the query in clear.
	redacted, query, found := strings.Cut(uri, "?")
	if found {
		redacted += "?" + redactQuery(query, sensitiveKeys)
	}
	return redactUserInfo(redacted, uri)
}

// redactHeaderValue masks a header value that carries a URL as uri is
// masked. The shape decides, not the header name, so custom proxy headers
// (X-Original-URI, X-Envoy-Original-Path, ...) are covered too. Every "?"
// starts a query that runs to the next "?", so each URL of a value that holds
// several (a Link list) has its sensitive query values masked; a value that
// starts with "/" or contains "://" also has its user information masked, as
// in uri. A value with neither "?" nor "@" is kept as is, and so is an "@"
// without URL shape (From: ops@example.com). A non-URL value with "?" or
// "://" may be over-masked, the safe direction.
func redactHeaderValue(value string, sensitiveKeys []string) string {
	if !strings.ContainsAny(value, "?@") {
		return value
	}
	redacted := redactEveryQuery(value, sensitiveKeys)
	if strings.HasPrefix(value, "/") || strings.Contains(value, "://") {
		return redactUserInfo(redacted, value)
	}
	return redacted
}

// redactEveryQuery masks, with redactQuery, the text after each "?" of value
// up to the next "?".
func redactEveryQuery(value string, sensitiveKeys []string) string {
	parts := strings.Split(value, "?")
	for i := 1; i < len(parts); i++ {
		parts[i] = redactQuery(parts[i], sensitiveKeys)
	}
	return strings.Join(parts, "?")
}

// redactUserInfo replaces the user information of target with
// redactedLogValue: everything from the start of the authority to the last
// "@", so a raw "/", "?" or "#" in a password is masked too (an "@" in the
// path or query over-masks). rawTarget is target before its query was
// masked; when masking removed the only "@" (a raw "?" in a password
// followed by a sensitive key), everything after the authority start is
// masked.
func redactUserInfo(target, rawTarget string) string {
	start := authorityStart(target)
	if start < 0 {
		return target
	}
	if at := strings.LastIndex(target[start:], "@"); at >= 0 {
		return target[:start] + redactedLogValue + target[start+at:]
	}
	if strings.Contains(rawTarget[start:], "@") {
		return target[:start] + redactedLogValue
	}
	return target
}

// authorityStart returns where the authority of target starts, or -1 for an
// origin-form ("/path") or asterisk-form ("*") target, which has none. A
// target with neither "/" nor "scheme://" at its start is taken as
// authority-form; an opaque URI such as "mailto:a@b" is then over-masked.
func authorityStart(target string) int {
	if strings.HasPrefix(target, "//") {
		return len("//")
	}
	if strings.HasPrefix(target, "/") || target == "*" {
		return -1
	}
	if scheme, _, found := strings.Cut(target, "://"); found && !strings.ContainsAny(scheme, "/?#") {
		return len(scheme) + len("://")
	}
	return 0
}

// redactQuery masks the values of rawQuery whose unescaped key is sensitive,
// and the values whose key cannot be unescaped. It reads rawQuery one
// "&"-separated segment at a time: when the segment's own key (up to its
// first "=") is sensitive, its whole value is masked, so a raw ";" in the
// name or the value (token;x=..., token=abc;def) exposes nothing. Otherwise
// the segment is split on ";" and the first part with a sensitive key masks
// the rest of the segment. The other parameters keep their order,
// separators and encoding; a part without "=" has no value and is kept.
func redactQuery(rawQuery string, sensitiveKeys []string) string {
	var redacted strings.Builder
	redacted.Grow(len(rawQuery))
	for i, segment := range strings.Split(rawQuery, "&") {
		if i > 0 {
			redacted.WriteByte('&')
		}
		if masked, isMasked := redactQueryPart(segment, sensitiveKeys); isMasked {
			redacted.WriteString(masked)
			continue
		}
		redacted.WriteString(redactSemicolonParts(segment, sensitiveKeys))
	}
	return redacted.String()
}

// redactSemicolonParts masks a ";"-separated segment from its first part
// with a sensitive key to its end.
func redactSemicolonParts(segment string, sensitiveKeys []string) string {
	parts := strings.Split(segment, ";")
	for i, part := range parts {
		if masked, isMasked := redactQueryPart(part, sensitiveKeys); isMasked {
			return strings.Join(append(parts[:i:i], masked), ";")
		}
	}
	return segment
}

// redactQueryPart returns part with its value masked, and whether it did so.
func redactQueryPart(part string, sensitiveKeys []string) (string, bool) {
	rawKey, _, hasValue := strings.Cut(part, "=")
	if !hasValue {
		return part, false
	}
	key, err := url.QueryUnescape(rawKey)
	if err != nil || isSensitiveKey(key, sensitiveKeys) {
		return rawKey + "=" + redactedLogValue, true
	}
	return part, false
}

func formatLogValue(value interface{}) string {
	rawValue, err := json.Marshal(value)
	if err != nil {
		return `"[unavailable]"`
	}
	return string(rawValue)
}

func generateEventID(ctx context.Context, prefix, uri string) string {
	if traceID := getTraceID(ctx); traceID != "" {
		return traceID
	}
	eventIDPayload := fmt.Sprintf("%v:%v:%v", prefix, time.Now().UnixNano(), uri)
	buf := []byte(eventIDPayload)
	return fmt.Sprintf("%x", md5.Sum(buf))
}

// ParamValidatorV0 is the built-in param validator: it accepts every value.
// Register a real one with RegisterParamValidator.
func ParamValidatorV0(v any) (string, error) {
	// not validate any param by default
	return "", nil
}
