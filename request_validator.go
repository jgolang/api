package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/jgolang/api/core"
)

// RequestContextKey type
type RequestContextKey string

// RequestDataContextKey request data context key to finds request context
const RequestDataContextKey = RequestContextKey("requestData")

// RequestReceiver implementation of core.APIRequestReceiver interface
type RequestReceiver struct{}

// ProcessEncryptedBody process API request encription information.
func (receiver RequestReceiver) ProcessEncryptedBody(r *http.Request) (*core.RequestEncryptedData, error) {
	var request JSONEncryptedBody
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(rawBody, &request)
	if err != nil {
		return nil, err
	}
	requestData := core.RequestEncryptedData{
		Data:     request.Data,
		Metadata: request.DeviceUUID,
	}
	return &requestData, nil
}

// ProcessBody process API request body information.
func (receiver RequestReceiver) ProcessBody(r *http.Request) (*core.RequestData, error) {
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
		UUID:          request.Header.UUID,
		DeviceType:    request.Header.DeviceType,
		DeviceBrand:   request.Header.DeviceBrand,
		DeviceModel:   request.Header.DeviceModel,
		DeviceOS:      request.Header.OS,
		OSVersion:     request.Header.OSVersion,
		OSTimezone:    request.Header.Timezone,
		AppLanguage:   request.Header.Lang,
		AppVersion:    request.Header.AppVersion,
		AppBuildInfo:  request.Header.AppBuildVersion,
		AppName:       request.Header.AppName,
		SecurityToken: request.Header.SecurityToken,
		RawBody:       rawBody,
		Content:       core.JSONContent(request.Content),
	}
	return &requestData, nil
}

// GetRouteVar returns the route variables for the current request, if any
func (receiver RequestReceiver) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}

// GetRouteVar returns the route variables for the current request, if any
// define it as: api.GetRouteVar = myCustomGetRouteVarFunc
var GetRouteVar func(string, *http.Request) string = func(string, *http.Request) string {
	PrintError("Define a GetRouteVar function in this package")
	return ""
}
