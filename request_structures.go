package api

import (
	"encoding/json"
)

// JSONRequest struct used to parse the request content section
type JSONRequest struct {
	Header  Header          `json:"header,omitempty"`
	Content json.RawMessage `json:"content"`
}

// Header request info section fields for encrypted requests
type Header struct {
	DeviceUUID      string `json:"uuid,omitempty"`
	DeviceType      string `json:"device_type,omitempty"`
	DeviceBrand     string `json:"device_brand,omitempty"`
	DeviceModel     string `json:"device_model,omitempty"`
	OS              string `json:"os,omitempty"`
	OSVersion       string `json:"os_version,omitempty"`
	Lang            string `json:"lang,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	AppVersion      string `json:"app_version,omitempty"`
	AppBuildVersion string `json:"app_build_version,omitempty"`
	AppName         string `json:"app_name,omitempty"`
	Token           string `json:"token,omitempty"`
}
