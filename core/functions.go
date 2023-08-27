package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// GetRouteVarValueString gets route variable value as string.
func (api API) GetRouteVarValueString(urlVarName string, r *http.Request) (string, error) {
	value := api.receiver.GetRouteVar(urlVarName, r)
	if value == "" {
		return value, fmt.Errorf("The route var %v has not been obtained", urlVarName)
	}
	return value, nil
}

// GetRouteVarValueInt gets route variable value as integer.
func (api API) GetRouteVarValueInt(urlVarName string, r *http.Request) (int, error) {
	s := api.receiver.GetRouteVar(urlVarName, r)
	value, err := strconv.Atoi(s)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetRouteVarValueInt64 gets route variable value as integer 64.
func (api API) GetRouteVarValueInt64(urlVarName string, r *http.Request) (int64, error) {
	s := api.receiver.GetRouteVar(urlVarName, r)
	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetRouteVarValueFloat64 gets route variable value as float 64.
func (api API) GetRouteVarValueFloat64(urlVarName string, r *http.Request) (float64, error) {
	s := api.receiver.GetRouteVar(urlVarName, r)
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetRouteVarValueBool gets route variable value as bool.
func (api API) GetRouteVarValueBool(urlVarName string, r *http.Request) (bool, error) {
	s := api.receiver.GetRouteVar(urlVarName, r)
	value, err := strconv.ParseBool(s)
	if err != nil {
		return false, err
	}
	return value, nil
}

// GetHeaderValueString gets header value as string.
func (api API) GetHeaderValueString(key string, r *http.Request) (string, error) {
	value := r.Header.Get(key)
	if value == "" {
		return value, fmt.Errorf("The %v key header has not been obtained", key)
	}
	return value, nil
}

// GetHeaderValueInt gets header value as integer.
func (api API) GetHeaderValueInt(key string, r *http.Request) (int, error) {
	value, err := strconv.Atoi(r.Header.Get(key))
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetHeaderValueInt64 gets header value as integer 64.
func (api API) GetHeaderValueInt64(key string, r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(r.Header.Get(key), 10, 64)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetHeaderValueFloat64 gets header value as float 64.
func (api API) GetHeaderValueFloat64(key string, r *http.Request) (float64, error) {
	value, err := strconv.ParseFloat(r.Header.Get(key), 64)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetHeaderValueBool gets header values as bool.
func (api API) GetHeaderValueBool(key string, r *http.Request) (bool, error) {
	value, err := strconv.ParseBool(r.Header.Get(key))
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetQueryParamValueString gets query param value as string.
func (api API) GetQueryParamValueString(queryParamName string, r *http.Request) (string, error) {
	value := r.URL.Query().Get(queryParamName)
	if value == "" {
		return value, fmt.Errorf("The query parameter %v has not been obtained", queryParamName)
	}
	return value, nil
}

// GetQueryParamValueInt gets query param value as integer.
func (api API) GetQueryParamValueInt(queryParamName string, r *http.Request) (int, error) {
	value, err := strconv.Atoi(r.URL.Query().Get(queryParamName))
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetQueryParamValueInt64 gets query param value as integer 64.
func (api API) GetQueryParamValueInt64(queryParamName string, r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(r.URL.Query().Get(queryParamName), 10, 64)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetQueryParamValueFloat64 gets query param value as float 64.
func (api API) GetQueryParamValueFloat64(queryParamName string, r *http.Request) (float64, error) {
	value, err := strconv.ParseFloat(r.URL.Query().Get(queryParamName), 64)
	if err != nil {
		return value, err
	}
	return value, nil
}

// GetQueryParamValueBool gets param value as bool.
func (api API) GetQueryParamValueBool(queryParamName string, r *http.Request) (bool, error) {
	value, err := strconv.ParseBool(r.URL.Query().Get(queryParamName))
	if err != nil {
		return false, err
	}
	return value, nil
}

// UnmarshalBody parses request body to a struct.
func (api API) UnmarshalBody(v interface{}, r *http.Request) error {
	bodyRequest, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bodyRequest, v)
	if err != nil {
		return err
	}
	return nil
}

// ValidateMethods validates if a method exist in a methods map.
func (api *API) ValidateMethods(keyMapMethod, method string) bool {
	methodAccepted := false
	mapMethods := *api.MapMethods
	for _, mtd := range mapMethods[keyMapMethod] {
		if mtd == method {
			methodAccepted = true
		}
	}
	return methodAccepted
}
