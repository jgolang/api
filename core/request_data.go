package core

// RequestData doc ..
type RequestData struct {
	DeviceUUID     string
	DeviceType     string
	DeviceOS       string
	OSVersion      string
	OSTimezone     string
	AppLanguage    string
	AppVersion     string
	AppName        string
	SessionID      string
	Headers        map[string]string
	AdditionalInfo map[string]string
	RawBody        []byte
	Data           interface{}
}

// AddAdditionalInfo func ...
func (data *RequestData) AddAdditionalInfo(key, value string) {
	data.AdditionalInfo[key] = value
}

// AddHeader doc
func (data *RequestData) AddHeader(key, value string) {
	data.Headers[key] = value
}
