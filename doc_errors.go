package api

import (
	"maps"
	"reflect"
	"slices"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/errors/codes"
)

// DocError documents an error response for status as the instance's built-in
// formatter sends it: an envelope with only its header in
// ResponseModeEnvelope, envelope.PlainErrorResponse in ResponseModePlain,
// envelope.ProblemDetails under application/problem+json in
// ResponseModeProblem, and JSON null in ResponseModeNone, where an error has
// no content. A status that never carries a body (1xx, 204 and 304) is
// documented without content in every mode, as in DocSuccess. An empty
// description documents the HTTP status text. The mode is read as in
// DocSuccess, and only the schema is written.
func (a *API) DocError(status int, description string) doc.RouteOption {
	response := doc.RouteResponse{Status: status, Description: description}
	if statusCarriesBody(status) {
		response = errorResponse(a.documentedResponseMode(), status, description)
	}
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, response)
	}
}

// DocErrors documents the error responses that WriteError sends for
// errorCodes. Each code goes to the status the instance gives its kind (its
// WithKindStatus override, else StatusForKind; 500 for a code without a
// kind), and each status gets one response, in ascending order, with the
// schema DocError documents and the HTTP status text as description. Every
// code adds a named example to its status: the name is the code the
// response carries (the code's Str(), or the default code WriteError uses
// when Str() is empty), the summary its Msg(), and the value the body the
// instance's formatter builds for the Error that WriteError writes for an
// error with that code, without an event ID or token since there is no
// request. A nil code, a nil pointer one included, documents an error
// without a code, named by the internal server error code. A repeated name
// keeps its first example; a later code with that name still documents its
// own status, without an example.
//
// The examples are built when DocErrors is called, like DocSuccessExample:
// with the mode, overrides and formatter of an API from New, or, on the
// default instance (a nil API included), with CurrentResponseMode,
// StatusForKind and the registered formatter. A formatter that returns nil,
// a body that cannot be encoded and a JSON null body (every error in
// ResponseModeNone) leave that code without an example.
// DocErrors never writes a response nor calls the WithErrorHook. With no
// codes it documents nothing, and as with every response option, a later
// option for the same status replaces it.
func (a *API) DocErrors(errorCodes ...codes.Coder) doc.RouteOption {
	responses := a.documentedErrors(errorCodes)
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, responses...)
	}
}

// documentedErrors returns the responses DocErrors documents for
// errorCodes, ordered by status.
func (a *API) documentedErrors(errorCodes []codes.Coder) []doc.RouteResponse {
	mode := a.documentedResponseMode()
	byStatus := make(map[int]*doc.RouteResponse)
	documentedCodes := make(map[string]bool)
	for _, coder := range errorCodes {
		coder = untypedNilCoder(coder)
		response := errorResponseForCode(coder, a.statusFor).withDefaults()
		documented, ok := byStatus[response.HTTPStatusCode]
		if !ok {
			created := errorResponse(mode, response.HTTPStatusCode, "")
			documented = &created
			byStatus[response.HTTPStatusCode] = documented
		}

		// A repeated name drops only its example: its status is still sent.
		name := string(response.ResponseCode)
		if documentedCodes[name] {
			continue
		}
		documentedCodes[name] = true
		a.addErrorExample(documented, name, codeMessage(coder), response)
	}

	statuses := slices.Sorted(maps.Keys(byStatus))
	responses := make([]doc.RouteResponse, 0, len(statuses))
	for _, status := range statuses {
		responses = append(responses, *byStatus[status])
	}
	return responses
}

// addErrorExample adds to documented the named example of response, an
// error with its defaults applied, as the instance's formatter sends it. A
// body with no example (see formattedExample) adds nothing; a Content-Type
// the formatter sets becomes the documented media type.
func (a *API) addErrorExample(documented *doc.RouteResponse, name, summary string, response Error) {
	example, contentType := a.formattedExample(core.ResponseData(response))
	if example == nil {
		return
	}
	if contentType != "" {
		documented.ContentType = contentType
	}
	if documented.Examples == nil {
		documented.Examples = make(map[string]any)
	}
	documented.Examples[name] = doc.ExampleObject{Summary: summary, Value: example}
}

// DocRejections documents the error responses the library's middlewares
// send on their own, which no handler writes: 400 when ProcessRequest or
// RequestBody cannot parse the request body, 401 from BasicToken,
// CustomToken and RequestHeaderSession, 413 for a body over the limit, and
// 500 for a handler that panics under ProcessRequest before writing. It
// adds the four responses in ascending order, each with the schema DocError
// documents and the HTTP status text as description, and one named example
// per body the library sends for that status: invalidRequestContent;
// invalidAuthorizationHeader, invalidBasicToken, invalidBearerToken and
// invalidSessionID; requestContentTooLarge; and internalServerError. Each
// example is the body the instance's formatter builds for the Error the
// middleware writes, without an event ID or token since there is no request.
//
// The examples are built when DocRejections is called, as with DocErrors,
// so they carry the values of DefaultInvalidAuthHeaderMsg and the other
// message variables at that moment. Every status is added whether or not
// the route uses the middleware that sends it; as with every response
// option, a later option for the same status replaces it. The 415 of
// RequestHeaderJSON and headers such as WWW-Authenticate are not
// documented. Since it adds several responses, doc.Docs.AddResponse ignores
// it: it cannot be registered as one component response.
func (a *API) DocRejections() doc.RouteOption {
	responses := a.documentedRejections()
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, responses...)
	}
}

// libraryRejection is an error the library's middlewares answer on their
// own, with the name and summary of its documented example.
type libraryRejection struct {
	name    string
	summary string
	build   func() Error
}

// libraryRejections returns the errors DocRejections documents, ordered by
// status. Each is built by the constructor its middleware writes with.
func libraryRejections() []libraryRejection {
	return []libraryRejection{
		{name: "invalidRequestContent", summary: "ProcessRequest or RequestBody could not parse the request body", build: bodyParseError},
		{name: "invalidAuthorizationHeader", summary: "BasicToken or CustomToken found no Authorization header of their scheme", build: func() Error {
			return unauthorizedError(DefaultInvalidAuthHeaderMsg)
		}},
		{name: "invalidBasicToken", summary: "BasicToken rejected the credentials", build: func() Error {
			return unauthorizedError(DefaultBasicUnauthorizedMsg)
		}},
		{name: "invalidBearerToken", summary: "CustomToken rejected the token", build: func() Error {
			return unauthorizedError(DefaultBearerUnauthorizedMsg)
		}},
		{name: "invalidSessionID", summary: "RequestHeaderSession found no session ID", build: func() Error {
			return unauthorizedError(invalidSessionIDMessage)
		}},
		{name: "requestContentTooLarge", summary: "The request body exceeds the size limit", build: bodyTooLargeError},
		{name: "internalServerError", summary: "The handler panicked under ProcessRequest before writing", build: func() Error {
			return Error500()
		}},
	}
}

// documentedRejections returns the responses DocRejections documents.
func (a *API) documentedRejections() []doc.RouteResponse {
	mode := a.documentedResponseMode()
	var responses []doc.RouteResponse
	for _, rejection := range libraryRejections() {
		response := rejection.build().withDefaults()
		if len(responses) == 0 || responses[len(responses)-1].Status != response.HTTPStatusCode {
			responses = append(responses, errorResponse(mode, response.HTTPStatusCode, ""))
		}
		a.addErrorExample(&responses[len(responses)-1], rejection.name, rejection.summary, response)
	}
	return responses
}

// untypedNilCoder returns nil for a nil coder and for a nil pointer (or
// other nil reference) behind the interface, such as (*codes.Code)(nil),
// whose methods would panic; DocErrors then documents it as an uncoded error.
func untypedNilCoder(coder codes.Coder) codes.Coder {
	if coder == nil {
		return nil
	}
	value := reflect.ValueOf(coder)
	switch value.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		if value.IsNil() {
			return nil
		}
	}
	return coder
}

// codeMessage returns coder's public message, "" for a nil coder.
func codeMessage(coder codes.Coder) string {
	if coder == nil {
		return ""
	}
	return coder.Msg()
}

// errorResponse returns the response DocError documents in mode.
func errorResponse(mode ResponseMode, status int, description string) doc.RouteResponse {
	response := doc.RouteResponse{Status: status, Description: description}
	switch mode {
	case ResponseModeNone:
		response.Schema = nullBodySchema()
	case ResponseModePlain:
		response.Body = envelope.PlainError()
	case ResponseModeProblem:
		response.Body = envelope.Problem()
		response.ContentType = problemContentType
	default:
		response.Body = envelope.Error()
	}
	return response
}
