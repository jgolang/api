package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// RequestReceiverV2 implementation of core.APIRequestReceiver interface.
// The zero value parses with CurrentRequestMode.
type RequestReceiverV2 struct {
	// mode is set by New; empty means the default instance's package variable.
	mode RequestMode
}

// ProcessEncryptedBody reads the encrypted payload from the values data and
// metadata, taken from the form body or the URL query (r.FormValue). Both are
// required; a missing one returns an error with the validation code
// codes.ValRequired (400 through WriteError). A body that cannot be parsed
// returns the parse error. On any error
// the returned value is empty.
func (receiver RequestReceiverV2) ProcessEncryptedBody(r *http.Request) (*core.RequestEncryptedData, error) {
	var requestData core.RequestEncryptedData
	if err := r.ParseForm(); err != nil {
		return &requestData, err
	}

	data := r.FormValue("data")
	metadata := r.FormValue("metadata")

	if data == "" || metadata == "" {
		return &requestData, errors.NewC(codes.ValRequired, "does not provide required information")
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

	mode := receiver.requestMode()
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

// withMode returns receiver bound to mode unless it already has one.
func (receiver RequestReceiverV2) withMode(mode RequestMode) RequestReceiverV2 {
	if receiver.mode == "" {
		receiver.mode = mode
	}
	return receiver
}

func (receiver RequestReceiverV2) requestMode() RequestMode {
	if receiver.mode == "" {
		return CurrentRequestMode
	}
	return receiver.mode
}

// GetRouteVar returns the route variables for the current request, if any
func (receiver RequestReceiverV2) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}
