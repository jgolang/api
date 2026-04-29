package api

import (
	"encoding/json"
)

// JSONRequest struct used to parse the request content section.
type JSONRequest struct {
	Header  JSONRequestInfo `json:"header,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

// JSONRequestOf documents a JSONRequest with a concrete content payload type.
//
// It is intended for OpenAPI schema generation only. Runtime requests keep using
// JSONRequest for backward compatibility.
type JSONRequestOf[T any] struct {
	Header  JSONRequestInfo `json:"header,omitempty"`
	Content *T              `json:"content,omitempty"`
}

// RequestDoc returns a typed request wrapper for OpenAPI documentation.
func RequestDoc[T any]() JSONRequestOf[T] {
	return JSONRequestOf[T]{}
}

// JSONRequestInfo request info section fields for encrypted requests.
type JSONRequestInfo struct {
	UUID            string `json:"uuid,omitempty" example:"ADAD3-ADD33-AFSFK"`
	DeviceType      string `json:"device_type,omitempty" example:"phone"`
	DeviceBrand     string `json:"device_brand,omitempty" example:"Samsung"`
	DeviceModel     string `json:"device_model,omitempty" example:"A11"`
	OS              string `json:"os,omitempty" example:"android"`
	OSVersion       string `json:"os_version,omitempty" example:"14"`
	Lang            string `json:"lang,omitempty" example:"es"`
	Timezone        string `json:"timezone,omitempty" example:"America/Mexico_City"`
	AppVersion      string `json:"app_version,omitempty" example:"3.0.0"`
	AppBuildVersion string `json:"app_build_version,omitempty" example:"1.0.0.10"`
	AppName         string `json:"app_name,omitempty" example:"My App"`
	SecurityToken   string `json:"token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	DeviceId        string `json:"device_id,omitempty" example:"device-123"`
	DeviceSerial    string `json:"device_serial,omitempty" example:"serial-123"`
	Latitude        string `json:"lat,omitempty" example:"19.4326"`
	Longitude       string `json:"lon,omitempty" example:"-99.1332"`
}

// JSONEncryptedBody struct used to parse the encrypted request and
// response body.
type JSONEncryptedBody struct {
	Data       string `json:"data" example:"encrypted-payload"`
	DeviceUUID string `json:"deviceUUID" example:"ADAD3-ADD33-AFSFK"`
}
