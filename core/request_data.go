package core

// RequestData doc ..
type RequestData struct {
	UUID            string
	DeviceType      string
	DeviceBrand     string
	DeviceModel     string
	OS              string
	OSVersion       string
	Lang            string
	Timezone        string
	AppVersion      string
	AppBuildVersion string
	AppName         string
	Token           string
	Headers         map[string]string
	AdditionalInfo  map[string]string
	RawBody         []byte
	Data            interface{}
}

// AddAdditionalInfo func ...
func (data *RequestData) AddAdditionalInfo(key, value string) {
	data.AdditionalInfo[key] = value
}

// AddHeader doc
func (data *RequestData) AddHeader(key, value string) {
	data.Headers[key] = value
}
