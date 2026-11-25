package api

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
	"slices"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/errors/codes"
)

// DocSuccess documents a success response for status whose content is
// sample, wrapped as the instance's built-in formatter sends it: an envelope
// with header and content in ResponseModeEnvelope, the flat object of
// envelope.PlainResponseOf in ResponseModePlain, and the content alone in
// ResponseModeNone and ResponseModeProblem. sample is a Go value of the
// content type, and its type is what is documented, so a *doc.Schema or a
// reflect.Type passed as sample documents that Go type; doc.ResponseSchema
// documents a schema written by hand. A pointer sample documents the type it
// points to; a nil sample documents a body without content (an envelope with
// only its header, a plain object whose content is null, or JSON null in
// none and problem modes). A status that never carries a body (1xx, 204 and
// 304) is documented without content in every mode. Only the schema is
// written, never an example, since a zero-value sample is not what the route
// sends; DocSuccessExample adds one built from a sample with real values.
//
// The mode is read when DocSuccess is called: an API from New uses its own,
// and the default instance, including a nil API, reads CurrentResponseMode.
// A custom formatter set with WithResponseFormatter is not inspected, so its
// routes keep documenting their bodies with the doc options. As with every
// response option, a later option for the same status replaces it.
func (a *API) DocSuccess(status int, sample any) doc.RouteOption {
	response := a.documentedSuccess(status, sample)
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, response)
	}
}

// DocSuccessExample documents the same response as DocSuccess, with headers,
// and adds an example: the body the instance's response formatter builds
// for a success carrying sample, as Success.Write builds it (default title
// and message, ResponseCodes.Success, no event ID or token since there is no
// request), decoded from its JSON. A Content-Type the formatter sets becomes
// the documented media type. Pass a sample with real values: it is what the
// example shows.
//
// The example is built when DocSuccessExample is called, with the formatter
// of an API from New (a custom one set with WithResponseFormatter included)
// or, on the default instance, the registered one. Values the formatter
// computes, such as timestamps, are frozen at that moment. A status that
// never carries a body, a formatter that returns nil, a body that cannot be
// encoded and a JSON null body leave the response without an example.
func (a *API) DocSuccessExample(status int, sample any, headers ...doc.ResponseHeaderInfo) doc.RouteOption {
	response := a.documentedSuccess(status, sample)
	response.Headers = append([]doc.ResponseHeaderInfo(nil), headers...)
	if statusCarriesBody(status) {
		response.Example, response.ContentType = a.successExample(status, sample)
	}
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, response)
	}
}

// documentedSuccess returns the response DocSuccess documents for status
// and sample in the instance's mode.
func (a *API) documentedSuccess(status int, sample any) doc.RouteResponse {
	if !statusCarriesBody(status) {
		return doc.RouteResponse{Status: status}
	}
	contentType := reflect.TypeOf(sample)
	for contentType != nil && contentType.Kind() == reflect.Ptr {
		contentType = contentType.Elem()
	}
	return successResponse(a.documentedResponseMode(), status, contentType)
}

// successExample formats a success response for status carrying sample with
// the instance's formatter and returns its body as the writer would send it,
// decoded with UseNumber so numbers stay exact, and the Content-Type the
// formatter sets. A body the writer could not send, or JSON null, gives no
// example: the response is then documented by its schema alone.
func (a *API) successExample(status int, sample any) (example any, contentType string) {
	return a.formattedExample(core.ResponseData{
		Title:          DefaultSuccessTitle,
		Message:        DefaultSuccessMessage,
		HTTPStatusCode: status,
		ResponseCode:   ResponseCodes.Success,
		ResponseType:   SuccessType,
		Content:        sample,
	})
}

// formattedExample formats data with the instance's formatter and returns
// its body as the writer would send it, decoded with UseNumber so numbers
// stay exact, and the Content-Type the formatter sets. The example is nil,
// so none is documented, when the formatter returns nil, the body cannot be
// encoded or it is JSON null.
func (a *API) formattedExample(data core.ResponseData) (example any, contentType string) {
	formatted := a.responseFormatter().Format(data)
	if formatted == nil {
		return nil, ""
	}
	contentType = formattedContentType(formatted.Headers)
	encoded, err := json.Marshal(formatted.Body)
	if err != nil {
		return nil, contentType
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&example); err != nil {
		return nil, contentType
	}
	return example, contentType
}

// responseFormatter returns the formatter the instance's writes use: the one
// New bound, or the registered one on the default instance.
func (a *API) responseFormatter() core.APIResponseFormatter {
	if a == nil || a.formatter == nil {
		return responseFormatter
	}
	return a.formatter
}

// successResponse returns the response DocSuccess documents in mode for a
// body that carries contentType, nil when it has no content.
func successResponse(mode ResponseMode, status int, contentType reflect.Type) doc.RouteResponse {
	response := doc.RouteResponse{Status: status}
	switch mode {
	case ResponseModeNone, ResponseModeProblem:
		if contentType == nil {
			response.Schema = nullBodySchema()
		} else {
			response.Body = contentType
		}
	case ResponseModePlain:
		// Plain mode writes every key, so a missing content is a null,
		// which *any allows.
		content := reflect.TypeFor[*any]()
		if contentType != nil {
			content = reflect.PointerTo(contentType)
		}
		response.Body = responseWrapperType(plainResponseType, content)
	default:
		// The envelope omits an empty content, so a nil contentType leaves
		// only its header.
		var content reflect.Type
		if contentType != nil {
			content = reflect.PointerTo(contentType)
		}
		response.Body = responseWrapperType(envelopeResponseType, content)
	}
	return response
}

// statusCarriesBody reports whether a response with status may have a body;
// net/http sends none for 1xx, 204 and 304.
func statusCarriesBody(status int) bool {
	return status >= http.StatusOK && status != http.StatusNoContent && status != http.StatusNotModified
}

// DocError documents an error response for status as the instance's built-in
// formatter sends it: an envelope with only its header in
// ResponseModeEnvelope, envelope.PlainErrorResponse in ResponseModePlain,
// envelope.ProblemDetails under application/problem+json in
// ResponseModeProblem, and JSON null in ResponseModeNone, where an error has
// no content. An empty description documents the HTTP status text. The mode
// is read as in DocSuccess, and only the schema is written.
func (a *API) DocError(status int, description string) doc.RouteOption {
	mode := a.documentedResponseMode()
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, errorResponse(mode, status, description))
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
// without a code, named by the internal server error code; a repeated name
// keeps its first example.
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
		name := string(response.ResponseCode)
		if documentedCodes[name] {
			continue
		}
		documentedCodes[name] = true

		documented, ok := byStatus[response.HTTPStatusCode]
		if !ok {
			created := errorResponse(mode, response.HTTPStatusCode, "")
			documented = &created
			byStatus[response.HTTPStatusCode] = documented
		}
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
// documented.
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

// documentedResponseMode returns the mode the instance's built-in formatter
// formats with. The default instance has no mode of its own.
func (a *API) documentedResponseMode() ResponseMode {
	if a == nil || a.responseMode == "" {
		return normalizeResponseMode(CurrentResponseMode)
	}
	return a.responseMode
}

// The doc types whose fields responseWrapperType copies. Their content type
// is a placeholder that responseWrapperType replaces.
var (
	envelopeResponseType = reflect.TypeFor[envelope.ResponseOf[struct{}]]()
	plainResponseType    = reflect.TypeFor[envelope.PlainResponseOf[struct{}]]()
)

// responseWrapperType builds the shape of a generic doc type, such as
// envelope.ResponseOf[T], for a content type known only at run time, since
// a method cannot be generic. It copies the fields of wrapper, tags
// included, and gives its Content field the type content, or drops the
// field when content is nil. The struct is anonymous, so the spec inlines
// it while its named field types stay $refs.
func responseWrapperType(wrapper, content reflect.Type) reflect.Type {
	fields := make([]reflect.StructField, 0, wrapper.NumField())
	for i := range wrapper.NumField() {
		field := wrapper.Field(i)
		if field.Name == "Content" {
			if content == nil {
				continue
			}
			field.Type = content
		}
		fields = append(fields, field)
	}
	return reflect.StructOf(fields)
}

// nullBodySchema describes a JSON null body. OpenAPI 3.0.3 needs a type
// beside nullable, and the enum leaves null as the only valid value.
func nullBodySchema() *doc.Schema {
	return &doc.Schema{Type: "object", Nullable: true, Enum: []any{nil}}
}
