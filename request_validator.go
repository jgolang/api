package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/jgolang/api/core"
)

// RequestValidator implementation
type RequestValidator struct {
}

// ValidateRequest doc
func (v RequestValidator) ValidateRequest(r *http.Request) (*core.RequestData, error) {
	var request JSONRequest
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(rawBody, &request)
	if err != nil {
		return nil, err
	}
	requestData := core.RequestData{
		UUID:            request.Header.DeviceUUID,
		DeviceType:      request.Header.DeviceType,
		DeviceBrand:     request.Header.DeviceBrand,
		DeviceModel:     request.Header.DeviceModel,
		OS:              request.Header.OS,
		OSVersion:       request.Header.OSVersion,
		Lang:            request.Header.Lang,
		Timezone:        request.Header.Timezone,
		AppVersion:      request.Header.AppVersion,
		AppBuildVersion: request.Header.AppBuildVersion,
		AppName:         request.Header.AppName,
		Token:           request.Header.Token,
		RawBody:         rawBody,
		Data:            request.Content,
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
