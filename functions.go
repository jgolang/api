package api

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"time"

	"github.com/jgolang/api/core"
)

// PrintError wrapper function. It affects only the default instance; an
// instance from New logs errors through its WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) instead.
var PrintError func(...interface{}) = log.Print

// Print wrapper function. It affects only the default instance; an instance
// from New logs requests and responses through its WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) instead.
var Print func(string, ...interface{}) = log.Printf

// Fatal wrapper function. It affects only the default instance, which calls
// it when a response cannot be encoded; an instance from New never exits and
// logs the failure through its WithLogger logger.
//
// Deprecated: use api.New(api.WithLogger(...)) instead.
var Fatal func(...interface{}) = log.Fatal

// GetHeaderValueString gets header value as string.
func GetHeaderValueString(key string, r *http.Request) (string, Response) {
	value, err := api.GetHeaderValueString(key, r)
	if err != nil {
		instanceFrom(r).logError(r, "header value not obtained", err)
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
		instanceFrom(r).logError(r, "header value not obtained", err)
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
		instanceFrom(r).logError(r, "header value not obtained", err)
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
		instanceFrom(r).logError(r, "header value not obtained", err)
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
		instanceFrom(r).logError(r, "header value not obtained", err)
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
		instanceFrom(r).logError(r, "route var not obtained", err)
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
		instanceFrom(r).logError(r, "route var not obtained", err)
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
		instanceFrom(r).logError(r, "route var not obtained", err)
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
		instanceFrom(r).logError(r, "route var not obtained", err)
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
		instanceFrom(r).logError(r, "route var not obtained", err)
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
		instanceFrom(r).logError(r, "query param not obtained", err)
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
		instanceFrom(r).logError(r, "query param not obtained", err)
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
		instanceFrom(r).logError(r, "query param not obtained", err)
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
		instanceFrom(r).logError(r, "query param not obtained", err)
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
		instanceFrom(r).logError(r, "query param not obtained", err)
		return value, Error{
			Title:   "Error getting query param type Bool!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// UnmarshalBody parses and validates request body to a struct
func UnmarshalBody(v interface{}, r *http.Request) Response {
	err := api.UnmarshalBody(v, r)
	if err != nil {
		instanceFrom(r).logError(r, "request body not unmarshaled", err)
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
		ctxType := reflect.TypeOf(ctx)
		ctxValue := reflect.ValueOf(ctx)
		if ctxType.Kind() == reflect.Ptr {
			elem := ctxValue.Elem()
			for i := 0; i < elem.NumField(); i++ {
				field := elem.Field(i)
				rctx, ok := field.Interface().(*RequestContext)
				if ok {
					return rctx
				}
			}
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

// PrintFullEvent set true value for allow print full event request
var PrintFullEvent bool = false

// LogRequestBody enables request body logging.
var LogRequestBody bool = false

// LogResponseBody enables response body logging.
var LogResponseBody bool = false

// MaxLoggedBodyBytes limits logged request and response bodies when PrintFullEvent is false.
var MaxLoggedBodyBytes int = 2000

// SensitiveLogKeys defines fields and headers that must be redacted in logs.
// It affects only the default instance, which reads it at call time; an
// instance from New masks the same keys plus cookie and session, and the
// keys added with WithSensitiveLogKeys.
//
// Deprecated: use api.New(api.WithSensitiveLogKeys(...)) instead.
var SensitiveLogKeys = v0SensitiveLogKeys()

// v0SensitiveLogKeys returns a fresh copy of the v0 sensitive key list, so
// changing SensitiveLogKeys never reaches an instance.
func v0SensitiveLogKeys() []string {
	return []string{
		"authorization",
		"token",
		"password",
		"secret",
		"credential",
		"apikey",
		"api-key",
		"key",
	}
}

// LogRequest prints API request in log.
func LogRequest(method, uri, eventID, form string, headers http.Header, rawBody []byte) {
	format := "REQUEST event_id=%q method=%q uri=%q"
	args := []interface{}{eventID, method, uri}
	if form != "" {
		format += " form=%q"
		args = append(args, form)
	}
	if len(headers) != 0 {
		format += " headers=%s"
		args = append(args, formatLogValue(sanitizeHeadersForLog(headers)))
	}
	if LogRequestBody && len(rawBody) != 0 {
		format += " body=%q"
		args = append(args, formatBodyForLog(rawBody))
	}
	Print(format, args...)
}

// LogResponse prints API response in log.
func LogResponse(eventID string, res *httptest.ResponseRecorder) {
	LogResponseWithDuration(eventID, res, 0)
}

// LogResponseWithDuration prints API response in log including request duration.
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
// whose name matches sensitiveKeys replaced by redactedLogValue.
func redactHeaders(headers http.Header, sensitiveKeys []string) http.Header {
	sanitized := make(http.Header, len(headers))
	for key, values := range headers {
		if isSensitiveKey(key, sensitiveKeys) {
			sanitized[key] = []string{redactedLogValue}
			continue
		}
		sanitized[key] = append([]string(nil), values...)
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

func ParamValidatorV0(v any) (string, error) {
	// not validate any param by default
	return "", nil
}
