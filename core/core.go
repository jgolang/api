package core

import (
	"encoding/json"
	"net/http"
)

// New doc ...
func New(v APIRequestValidator, f APIResponseFormatter, r APIWriter, s APISecurity, mapMethods *MapMethods) *API {
	return &API{
		requestValidator: v,
		formatter:        f,
		responder:        r,
		security:         s,
		MapMethods:       mapMethods,
	}
}

// API doc ...
type API struct {
	requestValidator APIRequestValidator
	formatter        APIResponseFormatter
	responder        APIWriter
	security         APISecurity
	MapMethods       *MapMethods
}

// MapMethods doc ...
type MapMethods map[string][]string

// Write doc ...
func (api *API) Write(data ResponseData, w http.ResponseWriter) {
	responseFormatted := api.formatter.Format(data)
	api.responder.Write(responseFormatted, w)
}

// ValidateRequest doc ...
func (api *API) ValidateRequest(r *http.Request) (*RequestData, error) {
	return api.requestValidator.ValidateRequest(r)
}

// RegisterNewAPIResponseFormatter doc ...
func (api *API) RegisterNewAPIResponseFormatter(f APIResponseFormatter) {
	api.formatter = f
}

// RegisterNewAPIResponder doc ...
func (api *API) RegisterNewAPIResponder(r APIWriter) {
	api.responder = r
}

// ValidateCustomToken doc ...
func (api *API) ValidateCustomToken(token string, customValidator CustomTokenValidator) (json.RawMessage, bool) {
	return api.security.ValidateCustomToken(token, customValidator)
}

// ValidateBasicToken doc ...
func (api *API) ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.security.ValidateBasicToken(token)
}

// RegisterNewAPIRequestValidator doc ...
func (api *API) RegisterNewAPIRequestValidator(v APIRequestValidator) {
	api.requestValidator = v
}

// RegisterNewAPISecurity doc ...
func (api *API) RegisterNewAPISecurity(s APISecurity) {
	api.security = s
}

// AddMapMethod doc ...
func (api *API) AddMapMethod(key string, methods []string) {
	mapMethods := *api.MapMethods
	mapMethods[key] = methods
	api.MapMethods = &mapMethods
}
