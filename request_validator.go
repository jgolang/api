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

// RequestReceiver implementation of core.APIRequestReceiver interface.
// The zero value parses with CurrentRequestMode.
type RequestReceiver struct {
	// mode is set by New; empty means the default instance's package variable.
	mode RequestMode
}

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
	return processRequestBody(r, receiver.requestMode(), true)
}

// processRequestBody is the ProcessBody of both built-in receivers. Only
// RequestReceiver fills the device serial, device ID, latitude and longitude
// (deviceFields); RequestReceiverV2 never did, and keeps it that way.
func processRequestBody(r *http.Request, mode RequestMode, deviceFields bool) (*core.RequestDataContext, error) {
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
	if deviceFields {
		requestData.DeviceSerial = headerData.DeviceSerial
		requestData.DeviceId = headerData.DeviceID
		requestData.Latitude = headerData.Latitude
		requestData.Longitude = headerData.Longitude
	}
	requestData.EventID = headerData.EventID

	if len(bytesTrimSpace(rawBody)) == 0 {
		return &requestData, nil
	}

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
		fillFromEnvelopeHeader(&requestData, request.Header, deviceFields)
		requestData.Content = core.JSONContent(request.Content)
		return &requestData, nil
	}

	if !json.Valid(rawBody) {
		return nil, io.ErrUnexpectedEOF
	}
	requestData.Content = core.JSONContent(rawBody)
	return &requestData, nil
}

// fillFromEnvelopeHeader fills the fields that the HTTP headers left empty
// from the envelope header, so an HTTP header always wins.
func fillFromEnvelopeHeader(requestData *core.RequestDataContext, header envelope.RequestInfo, deviceFields bool) {
	fillEmpty(&requestData.UUID, header.UUID)
	fillEmpty(&requestData.DeviceType, header.DeviceType)
	fillEmpty(&requestData.DeviceBrand, header.DeviceBrand)
	fillEmpty(&requestData.DeviceModel, header.DeviceModel)
	fillEmpty(&requestData.DeviceOS, header.OS)
	fillEmpty(&requestData.OSVersion, header.OSVersion)
	fillEmpty(&requestData.OSTimezone, header.Timezone)
	fillEmpty(&requestData.AppLanguage, header.Lang)
	fillEmpty(&requestData.AppVersion, header.AppVersion)
	fillEmpty(&requestData.AppBuildInfo, header.AppBuildVersion)
	fillEmpty(&requestData.AppName, header.AppName)
	fillEmpty(&requestData.SecurityToken, header.SecurityToken)
	if deviceFields {
		fillEmpty(&requestData.DeviceSerial, header.DeviceSerial)
		fillEmpty(&requestData.DeviceId, header.DeviceId)
		fillEmpty(&requestData.Latitude, header.Latitude)
		fillEmpty(&requestData.Longitude, header.Longitude)
	}
}

func fillEmpty(field *string, value string) {
	if *field == "" {
		*field = value
	}
}

// withMode returns receiver bound to mode.
func (receiver RequestReceiver) withMode(mode RequestMode) RequestReceiver {
	receiver.mode = mode
	return receiver
}

func (receiver RequestReceiver) requestMode() RequestMode {
	if receiver.mode == "" {
		return CurrentRequestMode
	}
	return receiver.mode
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
