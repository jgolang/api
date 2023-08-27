package core

// ResponseType contains all the response types identifers
type ResponseType string

// ResponseCode type
type ResponseCode string

// ResponseFormatted contain formatted information to be responsed.
type ResponseFormatted struct {
	Headers        map[string]string
	HTTPStatusCode int
	Body           interface{}
}

// ResponseData contain all information to generate the HTTP API response.
type ResponseData struct {
	// Title of response
	Title string

	// Message descriptor response
	Message string

	// HTTP status code respose
	HTTPStatusCode int

	// Custom code of respoonse
	ResponseCode ResponseCode

	// Response type: error, success, warning, etc.
	ResponseType ResponseType

	// The user security token
	SecurityToken string

	// Indicate actions for devices.
	Actions string

	// Request unique identifier
	EventID string

	// Headers for HTTP response
	Headers map[string]string

	// You can to use this map to add custom informations to generate
	// your API response.
	Info map[string]string

	// You can use this property to add the body content for your
	// API response.
	Content interface{}
}

// AddInfo adds new item to AditionalInfo map.
func (data *ResponseData) AddInfo(key, value string) {
	if data.Info == nil {
		data.Info = make(map[string]string)
	}
	data.Info[key] = value
}

// AddHeader adds new header to Headers map.
func (data *ResponseData) AddHeader(key, value string) {
	if data.Headers == nil {
		data.Headers = make(map[string]string)
	}
	data.Headers[key] = value
}
