package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

// PPPGMethodsKey POST, PUT, PATCH, and GET http methods to validate in NewRequestBodyMiddleware
const PPPGMethodsKey = "pppg"

// PPPMethodsKey POST, PUT and PATCH http methods to validate in NewRequestBodyMiddleware
const PPPMethodsKey = "ppp"

// PPMethodsKey POST and PUT http methods to validate in NewRequestBodyMiddleware
const PPMethodsKey = "pp"

// MethodPostKey POST http method to validate in NewRequestBodyMiddleware
const MethodPostKey = "post"

// MethodGetKey GET http method to validate in NewRequestBodyMiddleware
const MethodGetKey = "get"

// MethodPutKey PUT http method to validate in NewRequestBodyMiddleware
const MethodPutKey = "put"

// MethodPatchKey http method to validate in NewRequestBodyMiddleware
const MethodPatchKey = "patch"

// The api variable provides the API useful functions and implements of API-core package
var api = core.New(
	RequestValidator{},
	ResponseFormatter{},
	ResponseWriter{},
	&Security{},
	&mapMethods,
)

// RegisterNewAPIResponseFormatter register a new custom API response formatter to this implementation of API-core package
func RegisterNewAPIResponseFormatter(f core.APIResponseFormatter) {
	api.RegisterNewAPIResponseFormatter(f)
}

// RegisterNewAPIResponseWriter register a new custom API response writer to this implementation of API-core package
func RegisterNewAPIResponseWriter(f core.APIResponseWriter) {
	api.RegisterNewAPIResponseWriter(f)
}

// RegisterNewAPIRequestValidator register a new request validator to this implementation API-core package
func RegisterNewAPIRequestValidator(v core.APIRequestValidator) {
	api.RegisterNewAPIRequestValidator(v)
}

// AddNewMapMethod add a new methods map to validate in a custom implementation of API-core package
func AddNewMapMethod(key string, methods []string) {
	api.AddMapMethod(key, methods)
}

var mapMethods core.MapMethods

func init() {
	mapMethods = make(core.MapMethods)
	mapMethods[PPPGMethodsKey] = []string{
		http.MethodPost,
		http.MethodGet,
		http.MethodPut,
		http.MethodPatch,
	}
	mapMethods[PPPMethodsKey] = []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
	}
	mapMethods[PPMethodsKey] = []string{
		http.MethodPost,
		http.MethodPut,
	}
	mapMethods[MethodPostKey] = []string{
		http.MethodPost,
	}
	mapMethods[MethodGetKey] = []string{
		http.MethodGet,
	}
	mapMethods[MethodPutKey] = []string{
		http.MethodPut,
	}
	mapMethods[MethodPutKey] = []string{
		http.MethodPut,
	}
}
