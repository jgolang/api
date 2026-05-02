package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/envelope"
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
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	requestData := core.RequestDataContext{
		Context: r.Context(),
		RawBody: rawBody,
	}

	headerData := newRequestContextFromHeaders(r)
	requestData.UUID = headerData.UUID
	requestData.DeviceType = headerData.DeviceType
	requestData.DeviceBrand = headerData.DeviceBrand
	requestData.DeviceModel = headerData.DeviceModel
	requestData.DeviceOS = headerData.DeviceOS
	requestData.OSVersion = headerData.OSVersion
	requestData.OSTimezone = headerData.OSTimezone
	requestData.AppLanguage = headerData.AppLanguage
	requestData.AppVersion = headerData.AppVersion
	requestData.AppBuildInfo = headerData.AppBuildInfo
	requestData.AppName = headerData.AppName
	requestData.SecurityToken = headerData.SecurityToken
	requestData.EventID = headerData.EventID

	if len(bytesTrimSpace(rawBody)) == 0 {
		return &requestData, nil
	}

	mode := CurrentRequestMode
	if mode == RequestModeAuto && envelopeRequestBody(rawBody) {
		mode = RequestModeEnvelope
	}
	if mode == RequestModeAuto {
		mode = RequestModePlain
	}

	if mode == RequestModeEnvelope {
		var request envelope.JSONRequest
		err = json.Unmarshal(rawBody, &request)
		if err != nil {
			return nil, err
		}
		if requestData.UUID == "" {
			requestData.UUID = request.Header.UUID
		}
		if requestData.DeviceType == "" {
			requestData.DeviceType = request.Header.DeviceType
		}
		if requestData.DeviceBrand == "" {
			requestData.DeviceBrand = request.Header.DeviceBrand
		}
		if requestData.DeviceModel == "" {
			requestData.DeviceModel = request.Header.DeviceModel
		}
		if requestData.DeviceOS == "" {
			requestData.DeviceOS = request.Header.OS
		}
		if requestData.OSVersion == "" {
			requestData.OSVersion = request.Header.OSVersion
		}
		if requestData.OSTimezone == "" {
			requestData.OSTimezone = request.Header.Timezone
		}
		if requestData.AppLanguage == "" {
			requestData.AppLanguage = request.Header.Lang
		}
		if requestData.AppVersion == "" {
			requestData.AppVersion = request.Header.AppVersion
		}
		if requestData.AppBuildInfo == "" {
			requestData.AppBuildInfo = request.Header.AppBuildVersion
		}
		if requestData.AppName == "" {
			requestData.AppName = request.Header.AppName
		}
		if requestData.SecurityToken == "" {
			requestData.SecurityToken = request.Header.SecurityToken
		}
		requestData.Content = core.JSONContent(request.Content)
		return &requestData, nil
	}

	if !json.Valid(rawBody) {
		return nil, io.ErrUnexpectedEOF
	}
	requestData.Content = core.JSONContent(rawBody)
	return &requestData, nil
}

// GetRouteVar returns the route variables for the current request, if any
func (receiver RequestReceiverV2) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}
