package core

// ResponseType contains all the response types identifers
type ResponseType string

// ResponseFormatted ...
type ResponseFormatted struct {
	Headers    map[string]string
	StatusCode int
	Data       interface{}
}

// ResponseData doc ..
type ResponseData struct {
	Title          string
	Message        string
	StatusCode     int
	ErrorCode      string
	Action         string
	SessionID      string
	ResponseType   ResponseType
	Headers        map[string]string
	AdditionalInfo map[string]string
	Data           interface{}
}

// AddAdditionalInfo func ...
func (data *ResponseData) AddAdditionalInfo(key, value string) {
	data.AdditionalInfo[key] = value
}

// AddHeader doc
func (data *ResponseData) AddHeader(key, value string) {
	data.Headers[key] = value
}
