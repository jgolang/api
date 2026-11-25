package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
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
