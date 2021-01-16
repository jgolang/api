package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/jgolang/api/core"
)

// RequestValidator implementation
type RequestValidator struct {
}

// ValidateRequest doc
func (v RequestValidator) ValidateRequest(r *http.Request) (*core.RequestData, error) {
	var request JSONRequest
	rawBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(rawBody, &request)
	if err != nil {
		return nil, err
	}
	requestData := core.RequestData{
		DeviceUUID:  request.Info.DeviceUUID,
		DeviceType:  request.Info.DeviceType,
		DeviceOS:    request.Info.DeviceOS,
		OSVersion:   request.Info.OSVersion,
		OSTimezone:  request.Info.OSTimezone,
		AppLanguage: request.Info.AppLanguage,
		AppVersion:  request.Info.AppVersion,
		AppName:     request.Info.AppName,
		SessionID:   request.Info.SessionID,
		RawBody:     rawBody,
		Data:        request.Content,
	}
	return &requestData, nil
}

// GetRouteVar returns the route variables for the current request, if any
func (v RequestValidator) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}

// GetRouteVar returns the route variables for the current request, if any
// define it as: api.GetRouteVar = myCustomGetRouteVarFunc
var GetRouteVar func(string, *http.Request) string = func(string, *http.Request) string {
	PrintError("Define a GetRouteVar function in this package")
	return ""
}
