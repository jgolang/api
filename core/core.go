package core

import (
	"encoding/json"
	"net/http"
)

// New API core developer toolkit.
func New(receiver APIRequestReceiver, formater APIResponseFormatter, writer APIResponseWriter, guarantor APISecurityGuarantor, mapMethods *MapMethods) *API {
	return &API{
		receiver:   receiver,
		formatter:  formater,
		writer:     writer,
		guarantor:  guarantor,
		MapMethods: mapMethods,
	}
}

// API core developer toolkit.
type API struct {
	// Reciver contains functions to proccess and validate information
	// of API request.
	receiver APIRequestReceiver

	// Formatter contains functions to parse information to
	// JSON for API response.
	formatter APIResponseFormatter

	// Writer contains functions to writer response in screen.
	writer APIResponseWriter

	// Guarantor contains functions to validate tokens.
	guarantor APISecurityGuarantor

	// MapMethods contain a array of HTTP methos for API validations.
	MapMethods *MapMethods
}

// MapMethods you can to use this map to define your methods that allow or
// block in your API module.
type MapMethods map[string][]string

// Write API response in JSON format in screen. You can to define response
// JSON format implemented the APIResponseFormatter interface.
func (api *API) Write(data ResponseData, w http.ResponseWriter) {
	responseFormatted := api.formatter.Format(data)
	api.writer.Write(responseFormatted, w)
}

// ProcessEncryptedBody API request. You can to define request body encription and how to
// validate it implemented the APIRequestReciver interface.
func (api *API) ProcessEncryptedBody(r *http.Request) (*RequestEncryptedData, error) {
	return api.receiver.ProcessEncryptedBody(r)
}

// ProcessBody API request. You can to define request JSON format and how to
// validate it implemented the APIRequestReciver interface.
func (api *API) ProcessBody(r *http.Request) (*RequestData, error) {
	return api.receiver.ProcessBody(r)
}

// ValidateCustomToken validate token with a custom method.
func (api *API) ValidateCustomToken(token string, customValidator CustomTokenValidator) (json.RawMessage, bool) {
	return api.guarantor.ValidateCustomToken(token, customValidator)
}

// ValidateBasicToken validate token with a basic auth token validation method.
func (api *API) ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.guarantor.ValidateBasicToken(token)
}

// RegisterNewAPIRequestReceiver inject a new implementation in the
// APIRequestReceiver interface.
func (api *API) RegisterNewAPIRequestReceiver(receiver APIRequestReceiver) {
	api.receiver = receiver
}

// RegisterNewAPISecurityGuarantor inject a new implementation in the
// APISecurityGuarantor interface
func (api *API) RegisterNewAPISecurityGuarantor(guarantor APISecurityGuarantor) {
	api.guarantor = guarantor
}

// RegisterNewAPIResponseFormatter inject a new implementation in the
// APIResponseFormatter interface.
func (api *API) RegisterNewAPIResponseFormatter(formatter APIResponseFormatter) {
	api.formatter = formatter
}

// RegisterNewAPIResponseWriter inject a new implementation in the
// APIResponseWriter interface.
func (api *API) RegisterNewAPIResponseWriter(writer APIResponseWriter) {
	api.writer = writer
}

// AddMapMethod add a new method in a map of methods.
func (api *API) AddMapMethod(key string, methods []string) {
	mapMethods := *api.MapMethods
	mapMethods[key] = methods
	api.MapMethods = &mapMethods
}
