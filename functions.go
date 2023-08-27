package api

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
)

// PrintError defines a function to print the errors that occurred
// in this package, this function can be redefined.
// By default, this package implement the log.Print() function.
var PrintError func(...interface{}) = log.Print

// Printf defines a function to print the messages that occurred
// in this package, this function can be redefined.
// By default, this package implement the log.Printf() function.
var Printf func(string, ...interface{}) = log.Printf

// Fatal defines a function to print the fatal errors that occurred
// in this package, this function can be redefined.
// By default, this package implement the log.Fatal() function.
var Fatal func(...interface{}) = log.Fatal

// GetHeaderValueString doc ...
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

// GetHeaderValueInt doc ...
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

// GetHeaderValueInt64 doc ...
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

// GetHeaderValueFloat64 doc ...
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

// GetHeaderValueBool doc ...
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

// GetRouteVarValueString ...
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

// GetRouteVarValueInt ...
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

// GetRouteVarValueInt64 ...
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

// GetRouteVarValueFloat64 ...
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

// GetRouteVarValueBool ...
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

// GetQueryParamValueString ...
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

// GetQueryParamValueInt ...
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

// GetQueryParamValueInt64 ...
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

// GetQueryParamValueFloat64 ...
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

// GetQueryParamValueBool ...
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

// UnmarshalBody doc ...
func UnmarshalBody(v interface{}, r *http.Request) Response {
	err := api.UnmarshalBody(v, r)
	if err != nil {
		PrintError(err)
		return Error{
			Title:   "Not unmarshal JSON struct!",
			Message: "Error when unmarshal JSON structure",
		}
	}
	return nil
}

// LogRequest doc ...
func LogRequest(method, uri, eventID, form string, headers http.Header, rawBody []byte) {
	if eventID != "" {
		Printf("EVENT ID: %v", eventID)
	}
	Printf("REQUEST: [%v] %v\nREQUEST HEADERS: %v", method, uri, headers)
	if form != "" && len(form) != 0 {
		if len(form) > 2000 && os.Getenv("PRINT_FULL_EVENT") == "" {
			Printf("REQUEST FORM:\n%v", form[:1000], "••• SKIPPED •••", form[:1000])
		} else {
			Printf("REQUEST FORM:\n%v", form)
		}
	}
	if rawBody != nil && len(rawBody) != 0 {
		if len(rawBody) > 2000 && os.Getenv("PRINT_FULL_EVENT") == "" {
			Printf("REQUEST BODY:\n%v", string(rawBody[:1000]), " ••• SKIPPED ••• ", string(rawBody[len(rawBody)-1000:]))
		} else {
			Printf("REQUEST BODY:\n%v", string(rawBody))
		}
	}
}

// LogResponse doc ...
func LogResponse(res *httptest.ResponseRecorder) {
	Printf("STATUS CODE: %v %v\nRESPONSE HEADERS: %v", res.Code, http.StatusText(res.Code), res.Header())
	responseBody := res.Body.Bytes()
	if responseBody != nil && len(responseBody) != 0 {
		if len(responseBody) > 2000 && os.Getenv("PRINT_FULL_EVENT") == "" {
			Printf("RESPONSE BODY:\n%v", string(responseBody[:1000]), " ••• SKIPPED ••• ", string(responseBody[len(responseBody)-1000:]))
		} else {
			Printf("RESPONSE BODY:\n%v", string(responseBody))
		}
	}
}
