package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RequestMode controls how request payloads are parsed.
type RequestMode string

const (
	RequestModeNone     RequestMode = "none"
	RequestModePlain    RequestMode = "plain"
	RequestModeEnvelope RequestMode = "envelope"
	RequestModeAuto     RequestMode = "auto"
)

// ResponseMode controls how response payloads are formatted.
type ResponseMode string

const (
	ResponseModeNone     ResponseMode = "none"
	ResponseModePlain    ResponseMode = "plain"
	ResponseModeEnvelope ResponseMode = "envelope"
)

// CurrentRequestMode defines the active request parsing mode.
var CurrentRequestMode = RequestModePlain

// CurrentResponseMode defines the active response formatting mode.
var CurrentResponseMode = ResponseModeEnvelope

// SetRequestMode updates the request parsing mode.
func SetRequestMode(mode RequestMode) {
	switch mode {
	case RequestModeNone, RequestModePlain, RequestModeEnvelope, RequestModeAuto:
		CurrentRequestMode = mode
	default:
		CurrentRequestMode = RequestModePlain
	}
}

// SetResponseMode updates the response formatting mode.
func SetResponseMode(mode ResponseMode) {
	switch mode {
	case ResponseModeNone, ResponseModePlain, ResponseModeEnvelope:
		CurrentResponseMode = mode
	default:
		CurrentResponseMode = ResponseModeEnvelope
	}
}

func newRequestContextFromHeaders(r *http.Request) coreRequestContext {
	ctx := coreRequestContext{}
	if r == nil {
		return ctx
	}
	ctx.UUID = r.Header.Get("X-Request-ID")
	if ctx.UUID == "" {
		ctx.UUID = r.Header.Get("UUID")
	}
	ctx.DeviceType = r.Header.Get("DeviceType")
	ctx.DeviceBrand = r.Header.Get("DeviceBrand")
	ctx.DeviceModel = r.Header.Get("DeviceModel")
	ctx.DeviceOS = r.Header.Get("DeviceOS")
	ctx.OSVersion = r.Header.Get("OSVersion")
	ctx.OSTimezone = r.Header.Get("OSTimezone")
	ctx.AppLanguage = r.Header.Get("AppLanguage")
	if ctx.AppLanguage == "" {
		ctx.AppLanguage = r.Header.Get("Accept-Language")
	}
	ctx.AppVersion = r.Header.Get("AppVersion")
	ctx.AppBuildInfo = r.Header.Get("AppBuildVersion")
	ctx.AppName = r.Header.Get("AppName")
	ctx.DeviceSerial = r.Header.Get("DeviceSerial")
	ctx.DeviceID = r.Header.Get("DeviceID")
	ctx.Latitude = r.Header.Get("Latitude")
	ctx.Longitude = r.Header.Get("Longitude")
	ctx.SecurityToken = r.Header.Get(SecurityTokenHeaderKey)
	if ctx.SecurityToken == "" {
		ctx.SecurityToken = bearerTokenFromHeader(r.Header.Get("Authorization"))
	}
	ctx.EventID = r.Header.Get(EventIDHeaderKey)
	if ctx.EventID == "" {
		ctx.EventID = r.Header.Get("X-Request-ID")
	}
	return ctx
}

type coreRequestContext struct {
	UUID          string
	DeviceType    string
	DeviceBrand   string
	DeviceModel   string
	DeviceOS      string
	OSVersion     string
	OSTimezone    string
	AppLanguage   string
	AppVersion    string
	AppBuildInfo  string
	AppName       string
	SecurityToken string
	DeviceSerial  string
	DeviceID      string
	Latitude      string
	Longitude     string
	EventID       string
}

func envelopeRequestBody(rawBody []byte) bool {
	if len(bytesTrimSpace(rawBody)) == 0 {
		return false
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return false
	}
	if len(body) == 0 || len(body) > 2 {
		return false
	}
	if _, ok := body["content"]; ok {
		return true
	}
	_, hasHeader := body["header"]
	return hasHeader
}

func bytesTrimSpace(rawBody []byte) []byte {
	return []byte(strings.TrimSpace(string(rawBody)))
}

func bearerTokenFromHeader(value string) string {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
