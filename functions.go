package api

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// PrintError wrapper function.
var PrintError func(...interface{}) = log.Print

// Print wrapper function.
var Print func(string, ...interface{}) = log.Printf

// Fatal wrapper function.
var Fatal func(...interface{}) = log.Fatal

// GetHeaderValueString gets header value as string.
func GetHeaderValueString(key string, r *http.Request) (string, Response) {
	value, err := api.GetHeaderValueString(key, r)
	if err != nil {
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
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
		PrintError(err)
		return value, Error{
			Title:   "Error getting route var type Bool",
			Message: fmt.Sprintf("The route var %v has not been obtained", urlVarName),
		}
	}
	return value, nil
}

// GetQueryParamValueString gets query param value as string.
func GetQueryParamValueString(queryParamName string, r *http.Request) (string, Response) {
	value, err := api.GetQueryParamValueString(queryParamName, r)
	if err != nil {
		PrintError(err)
		return value, Error{
			Title:   "Error getting query param!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil

}

// GetQueryParamValueInt gets query param value as integer.
func GetQueryParamValueInt(queryParamName string, r *http.Request) (int, Response) {
	value, err := api.GetQueryParamValueInt(queryParamName, r)
	if err != nil {
		PrintError(err)
		return value, Error{
			Title:   "Error getting query param type Int!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// GetQueryParamValueInt64 gets query param value as integer 64.
func GetQueryParamValueInt64(queryParamName string, r *http.Request) (int64, Response) {
	value, err := api.GetQueryParamValueInt64(queryParamName, r)
	if err != nil {
		PrintError(err)
		return value, Error{
			Title:   "Error getting query param type Int64!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// GetQueryParamValueFloat64 gets query param value as float 64.
func GetQueryParamValueFloat64(queryParamName string, r *http.Request) (float64, Response) {
	value, err := api.GetQueryParamValueFloat64(queryParamName, r)
	if err != nil {
		PrintError(err)
		return value, Error{
			Title:   "Error getting query param type Float64!",
			Message: fmt.Sprintf("The query parameter %v has not been obtained", queryParamName),
		}
	}
	return value, nil
}

// GetQueryParamValueBool gets param value as bool.
func GetQueryParamValueBool(queryParamName string, r *http.Request) (bool, Response) {
	value, err := api.GetQueryParamValueBool(queryParamName, r)
	if err != nil {
		PrintError(err)
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
		PrintError(err)
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
	return nil, fmt.Errorf("Context requestData not found")
}

// UpdateRequestContext update request context.
func UpdateRequestContext(requestData *RequestContext, r *http.Request) *http.Request {
	return SetContextValue(
		RequestDataContextContextKey,
		requestData,
		r,
	)
}

// PrintFullEvent set true value for allow print full event request
var PrintFullEvent bool = false

// LogRequest prints API request in log.
func LogRequest(method, uri, eventID, form string, headers http.Header, rawBody []byte) {
	var requestBody string
	if rawBody != nil && len(rawBody) != 0 {
		if len(rawBody) > 2000 && !PrintFullEvent {
			requestBody = fmt.Sprintf("REQUEST_BODY: %v%v%v", string(rawBody[:1000]), " ***** SKIPPED ***** ", string(rawBody[len(rawBody)-1000:]))
		} else {
			requestBody = fmt.Sprintf("REQUEST_BODY: %v", string(rawBody))
		}
	}
	Print("REQUEST_EVENT_ID: %v \nREQUEST_URI: [%v] %v \n%v", eventID, method, uri, requestBody)
}

// LogResponse prints API response in log.
func LogResponse(eventID string, res *httptest.ResponseRecorder) {
	var responseBody string
	rawBody := res.Body.Bytes()
	if rawBody != nil && len(rawBody) != 0 {
		if len(rawBody) > 2000 && !PrintFullEvent {
			responseBody = fmt.Sprintf("RESPONSE_BODY: %v%v%v", string(rawBody[:1000]), " ***** SKIPPED ***** ", string(rawBody[len(rawBody)-1000:]))
		} else {
			responseBody = fmt.Sprintf("RESPONSE_BODY: %v", string(rawBody))
		}
	}
	Print("RESPONSE_EVENT_ID: %v \nSTATUS_CODE: %v %v \n%v", eventID, res.Code, http.StatusText(res.Code), responseBody)
}

func generateEventID(ctx context.Context, prefix, uri string) string {
	if traceID := getOtelTraceID(ctx); traceID != "" {
		return traceID
	}
	eventIDPayload := fmt.Sprintf("%v:%v:%v", prefix, time.Now().UnixNano(), uri)
	buf := []byte(eventIDPayload)
	return fmt.Sprintf("%x", md5.Sum(buf))
}

// getTraceID retrieves the trace ID from the provided context
func getOtelTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}

	traceID := span.SpanContext().TraceID().String()
	return traceID
}

func ParamValidatorV0(v any) (string, error) {
	// not validate any param by default
	return "", nil
}
