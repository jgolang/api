package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RequestMode controls how request payloads are parsed.
type RequestMode string

// Request modes of the built-in receivers. An unknown value falls back to
// RequestModePlain.
const (
	// RequestModeNone does not unwrap the body; it is read as plain JSON.
	RequestModeNone RequestMode = "none"
	// RequestModePlain reads the body as the endpoint content (the default).
	RequestModePlain RequestMode = "plain"
	// RequestModeEnvelope reads the body as {"header": ..., "content": ...};
	// the header fills request metadata the HTTP headers did not set.
	// RequestReceiverV2 skips the device serial, device ID and location
	// fields.
	RequestModeEnvelope RequestMode = "envelope"
	// RequestModeAuto reads an envelope when the body looks like one (a JSON
	// object with one or two keys, one of them header or content) and plain
	// JSON otherwise.
	RequestModeAuto RequestMode = "auto"
)

// ResponseMode controls how response payloads are formatted.
type ResponseMode string

// Response modes of the built-in formatter. An unknown value falls back to
// ResponseModeEnvelope.
const (
	// ResponseModeNone writes only the content, as is.
	ResponseModeNone ResponseMode = "none"
	// ResponseModePlain writes the header fields at the top level, next to
	// content.
	ResponseModePlain ResponseMode = "plain"
	// ResponseModeEnvelope writes {"header": ..., "content": ...} (the
	// default).
	ResponseModeEnvelope ResponseMode = "envelope"

	// ResponseModeProblem writes error responses as RFC 9457 problem details
	// (application/problem+json) and any other response as its content
	// alone, as ResponseModeNone.
	ResponseModeProblem ResponseMode = "problem"
)

// CurrentRequestMode defines the active request parsing mode of the default
// instance, the one that serves requests not wrapped by (*API).Handler.
//
// Deprecated: use api.New(api.WithRequestMode(...)) instead. It affects only
// the default instance. A service with a custom receiver, formatter or
// writer passes it with WithRequestReceiver, WithResponseFormatter or
// WithResponseWriter.
var CurrentRequestMode = RequestModePlain

// CurrentResponseMode defines the active response formatting mode of the
// default instance, the one that serves requests not wrapped by
// (*API).Handler.
//
// Deprecated: use api.New(api.WithResponseMode(...)) instead. It affects only
// the default instance. A service with a custom receiver, formatter or
// writer passes it with WithRequestReceiver, WithResponseFormatter or
// WithResponseWriter.
var CurrentResponseMode = ResponseModeEnvelope

// SetRequestMode updates the request parsing mode of the default instance.
// An unknown mode resets it to RequestModePlain.
//
// Deprecated: use api.New(api.WithRequestMode(...)) instead. It affects only
// the default instance. A service with a custom receiver, formatter or
// writer passes it with WithRequestReceiver, WithResponseFormatter or
// WithResponseWriter.
func SetRequestMode(mode RequestMode) {
	CurrentRequestMode = normalizeRequestMode(mode)
}

// SetResponseMode updates the response formatting mode of the default
// instance. An unknown mode resets it to ResponseModeEnvelope.
//
// Deprecated: use api.New(api.WithResponseMode(...)) instead. It affects only
// the default instance. A service with a custom receiver, formatter or
// writer passes it with WithRequestReceiver, WithResponseFormatter or
// WithResponseWriter.
func SetResponseMode(mode ResponseMode) {
	CurrentResponseMode = normalizeResponseMode(mode)
}

func normalizeRequestMode(mode RequestMode) RequestMode {
	switch mode {
	case RequestModeNone, RequestModePlain, RequestModeEnvelope, RequestModeAuto:
		return mode
	default:
		return RequestModePlain
	}
}

func normalizeResponseMode(mode ResponseMode) ResponseMode {
	switch mode {
	case ResponseModeNone, ResponseModePlain, ResponseModeEnvelope, ResponseModeProblem:
		return mode
	default:
		return ResponseModeEnvelope
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
