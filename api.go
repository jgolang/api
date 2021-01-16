package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

// PPPGMethodsKey POST, PUT, PATCH, and GET http methods ..
const PPPGMethodsKey = "pppg"

// PPPMethodsKey POST, PUT and PATCH http methods ..
const PPPMethodsKey = "ppp"

// PPMethodsKey POST and PUT http methods ..
const PPMethodsKey = "pp"

// MethodPostKey POST http method key ..
const MethodPostKey = "post"

// MethodGetKey GET http method key ..
const MethodGetKey = "get"

// MethodPutKey PUT http method key ..
const MethodPutKey = "put"

// MethodPatchKey http method key ..
const MethodPatchKey = "patch"

var api = core.New(
	RequestValidator{},
	ResponseFormatter{},
	ResponseWriter{},
	&Security{},
	&mapMethods,
)

// RegisterNewAPIResponseFormatter doc ...
func RegisterNewAPIResponseFormatter(f core.APIResponseFormatter) {
	api.RegisterNewAPIResponseFormatter(f)
}

// RegisterNewAPIResponseWriter doc ...
func RegisterNewAPIResponseWriter(f core.APIResponseWriter) {
	api.RegisterNewAPIResponseWriter(f)
}

// RegisterNewAPIRequestValidator doc ...
func RegisterNewAPIRequestValidator(v core.APIRequestValidator) {
	api.RegisterNewAPIRequestValidator(v)
}

// AddNewMapMethod doc ...
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
