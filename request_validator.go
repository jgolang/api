package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/envelope"
)

// RequestContextKey type
type RequestContextKey string

// RequestDataContextContextKey request data context key to finds request context
const RequestDataContextContextKey = RequestContextKey("requestData")

const routeVarsContextKey = RequestContextKey("routeVars")

// RequestReceiver implementation of core.APIRequestReceiver interface
type RequestReceiver struct{}

// ProcessEncryptedBody process API request encription information.
func (receiver RequestReceiver) ProcessEncryptedBody(r *http.Request) (*core.RequestEncryptedData, error) {
	var request envelope.EncryptedBody
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
func (receiver RequestReceiver) ProcessBody(r *http.Request) (*core.RequestDataContext, error) {
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
	requestData.DeviceSerial = headerData.DeviceSerial
	requestData.DeviceId = headerData.DeviceID
	requestData.Latitude = headerData.Latitude
	requestData.Longitude = headerData.Longitude
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
		if requestData.DeviceSerial == "" {
			requestData.DeviceSerial = request.Header.DeviceSerial
		}
		if requestData.DeviceId == "" {
			requestData.DeviceId = request.Header.DeviceId
		}
		if requestData.Latitude == "" {
			requestData.Latitude = request.Header.Latitude
		}
		if requestData.Longitude == "" {
			requestData.Longitude = request.Header.Longitude
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
func (receiver RequestReceiver) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}

// GetRouteVar returns the route variables for the current request, if any
// define it as: api.GetRouteVar = myCustomGetRouteVarFunc
var GetRouteVar func(string, *http.Request) string = func(key string, r *http.Request) string {
	return getRouteVarFromContext(key, r)
}

func getRouteVarFromContext(key string, r *http.Request) string {
	vars, ok := r.Context().Value(routeVarsContextKey).(map[string]string)
	if !ok {
		return ""
	}
	return vars[key]
}

// SetRouteVars stores route variables in the request context.
func SetRouteVars(vars map[string]string, r *http.Request) *http.Request {
	return SetContextValue(routeVarsContextKey, vars, r)
}
