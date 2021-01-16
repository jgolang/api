package api

import (
	"encoding/json"
)

// JSONRequest struct used to parse the request content section
type JSONRequest struct {
	Info    JSONRequestInfo `json:"info,omitempty"`
	Content json.RawMessage `json:"content"`
}

// JSONRequestInfo request info section fields for encrypted requests
type JSONRequestInfo struct {
	DeviceUUID  string `json:"device_uuid,omitempty"`
	DeviceType  string `json:"device,omitempty"`
	DeviceOS    string `json:"device_os,omitempty"`
	OSVersion   string `json:"os_version,omitempty"`
	OSTimezone  string `json:"os_timezone,omitempty"`
	AppLanguage string `json:"app_lang,omitempty"`
	AppVersion  string `json:"app_version,omitempty"`
	AppName     string `json:"app_name,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}
