package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jgolang/api/core"
)

// RequestReceiverV2 implementation of core.APIRequestReceiver interface
type RequestReceiverV2 struct{}

// ProcessEncryptedBody API request encription information.
func (receiver RequestReceiverV2) ProcessEncryptedBody(r *http.Request) (*core.RequestEncryptedData, error) {
	var requestData core.RequestEncryptedData
	r.ParseForm()

	data := r.FormValue("data")
	metadata := r.FormValue("metadata")

	if data != "" || metadata != "" {
		return &requestData, fmt.Errorf("does not provide required information")
	}

	requestData.Data = data
	requestData.Metadata = metadata

	return &requestData, nil
}

// ProcessBody API request body information.
func (receiver RequestReceiverV2) ProcessBody(r *http.Request) (*core.RequestDataContext, error) {
	var request JSONRequest
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(rawBody, &request)
	if err != nil {
		return nil, err
	}
	requestData := core.RequestDataContext{
		Context:       r.Context(),
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
func (receiver RequestReceiverV2) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}
