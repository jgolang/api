package core

import (
	"encoding/json"
	"fmt"

	"github.com/jgolang/helpers/info"
)

// RequestEncryptedData contains all encryptions information to process the API request.
type RequestEncryptedData struct {
	// Request Metadata
	Metadata string

	// Data contains more data
	Data string

	// You can to use this map to extend request information
	Info info.Info
}

// RequestData contains all information to process the API request.
type RequestData struct {
	// Client device UUID
	UUID string

	// Client device type
	DeviceType string

	// Client device brand
	DeviceBrand string

	// Client device model
	DeviceModel string

	// Client device operating system
	DeviceOS string

	// Client device operating system version
	OSVersion string

	// Client device operating system timezone
	OSTimezone string

	// Client App language config
	AppLanguage string

	// Client App version
	AppVersion string

	// Client App build information
	AppBuildInfo string

	// Client App name
	AppName string

	// Client security token
	SecurityToken string

	// DeviceSerial device serial number
	DeviceSerial string

	// DeviceId device unique id
	DeviceId string

	// Latitude device latitude
	Latitude string

	// Longitude device longitude
	Longitude string

	// Request event ID
	EventID string

	// HTTP request headers
	Headers map[string]string

	// You can to use this map to extend request information
	Info info.Info

	// You can use this property to add the body content for your
	// API request in json format.
	Content JSONContent

	// RawBody content
	RawBody []byte

	// Data contains more data
	Data interface{}
}

// DecodeContent decodes RequestData.Content property from json to a struct.
func (data *RequestData) DecodeContent(v interface{}) error {
	return data.Content.Decode(v)
}

// AddInfo adds new item to AditionalInfo map.
func (data *RequestData) AddInfo(key, value string) {
	data.Info.Set(key, value)
}

// Set additional info value.
func (data *RequestData) Set(key string, value interface{}) {
	data.Info.Set(key, value)
}

// Get additional info value.
func (data *RequestData) Get(key string) (value interface{}) {
	return data.Info.Get(key)
}

// GetString gets additional info value as string.
func (data *RequestData) GetString(key string) string {
	return data.Info.GetString(key)
}

// GetInt gets additional info value as int.
func (data *RequestData) GetInt(key string) int {
	return data.Info.GetInt(key)
}

// GetInt64 gets additional info value as int64.
func (data *RequestData) GetInt64(key string) int64 {
	return data.Info.GetInt64(key)
}

// GetFloat gets additional ifno value as float64.
func (data *RequestData) GetFloat(key string) float64 {
	return data.Info.GetFloat(key)
}

// GetBool gets additional info value as bool.
func (data *RequestData) GetBool(key string) bool {
	return data.Info.GetBool(key)
}

// GetStruct unmarhal a struct in additional info map.
func (data *RequestData) GetStruct(key string, v interface{}) error {
	return data.Info.GetStruct(key, v)
}

// AddHeader adds new header to Headers map.
func (data *RequestData) AddHeader(key, value string) {
	if data.Headers == nil {
		data.Headers = make(map[string]string)
	}
	data.Headers[key] = value
}

// JSONContent use to set a json request body.
// Use to parse json request body to a structure.
type JSONContent []byte

// Decode func decodes json content to an any structure.
func (content JSONContent) Decode(v interface{}) error {
	err := json.Unmarshal(content, v)
	if err != nil {
		return err
	}
	return nil
}

// MarshalJSON returns m as the JSON encoding of m.
func (content JSONContent) MarshalJSON() ([]byte, error) {
	if content == nil {
		return []byte("null"), nil
	}
	return content, nil
}

// UnmarshalJSON sets *m to a copy of data.
func (content *JSONContent) UnmarshalJSON(data []byte) error {
	if content == nil {
		return fmt.Errorf("core.JSONContent: UnmarshalJSON on nil pointer")
	}
	*content = append((*content)[0:0], data...)
	return nil
}
