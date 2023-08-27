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

// MethodPatchKey PATCH http method key ..
const MethodPatchKey = "patch"

var api = core.New(
	RequestReceiver{},
	ResponseFormatter{},
	ResponseWriter{},
	&SecurityGuaranter{},
	&mapMethods,
)

// Write API response in JSON format in screen. You can to define response
// JSON format implemented the APIResponseFormatter interface.
func Write(data core.ResponseData, w http.ResponseWriter) {
	api.Write(data, w)
}

// ProcessEncryptedBody API request. You can to define request url encoding format and how to
// validate it implemented the APIRequestReciver interface.
func ProcessEncryptedBody(r *http.Request) (*EncryptedRequest, error) {
	requestData, err := api.ProcessEncryptedBody(r)
	return &EncryptedRequest{
		RequestEncryptedData: requestData,
	}, err
}

// ProcessBody API request. You can to define request JSON format and how to
// validate it implemented the APIRequestReciver interface.
func ProcessBody(r *http.Request) (*Request, error) {
	requestData, err := api.ProcessBody(r)
	return &Request{
		RequestData: requestData,
	}, err
}

// RegisterNewAPIResponseFormatter inject a new implementation in the
// APIResponseFormatter interface.
func RegisterNewAPIResponseFormatter(formatter core.APIResponseFormatter) {
	api.RegisterNewAPIResponseFormatter(formatter)
}

// RegisterNewAPIResponseWriter inject a new implementation in the
// core.APIResponseWriter interface.
func RegisterNewAPIResponseWriter(writer core.APIResponseWriter) {
	api.RegisterNewAPIResponseWriter(writer)
}

// RegisterNewAPIRequestReceiver inject a new implementation in the
// core.APIRequestReceiver interface.
func RegisterNewAPIRequestReceiver(receiver core.APIRequestReceiver) {
	api.RegisterNewAPIRequestReceiver(receiver)
}

// RegisterNewAPISecurityGuarantor inject a new implementation in the
// core.APISecurityGuarantor interface
func RegisterNewAPISecurityGuarantor(guarantor core.APISecurityGuarantor) {
	api.RegisterNewAPISecurityGuarantor(guarantor)
}

// ValidateMethods validates if a method exist in a methods map.
func ValidateMethods(keyMapMethod, method string) bool {
	return api.ValidateMethods(keyMapMethod, method)
}

// AddNewMapMethod add a new method in a map of methods.
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
