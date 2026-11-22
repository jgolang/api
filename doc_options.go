package api

import (
	"net/http"
	"reflect"

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
// sends.
//
// The mode is read when DocSuccess is called: an API from New uses its own,
// and the default instance, including a nil API, reads CurrentResponseMode.
// A custom formatter set with WithResponseFormatter is not inspected, so its
// routes keep documenting their bodies with the doc options. As with every
// response option, a later option for the same status replaces it.
func (a *API) DocSuccess(status int, sample any) doc.RouteOption {
	if !statusCarriesBody(status) {
		return func(route *doc.Route) {
			route.Responses = append(route.Responses, doc.RouteResponse{Status: status})
		}
	}
	contentType := reflect.TypeOf(sample)
	for contentType != nil && contentType.Kind() == reflect.Ptr {
		contentType = contentType.Elem()
	}
	mode := a.documentedResponseMode()
	return func(route *doc.Route) {
		route.Responses = append(route.Responses, successResponse(mode, status, contentType))
	}
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
