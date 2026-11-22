package api

import (
	"net/http"
	"reflect"

	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
)

// DocSuccess documents a success response for status whose content is
// sample, wrapped as the instance's built-in formatter sends it: an envelope
// with header and content in ResponseModeEnvelope, the content alone in
// ResponseModeNone and ResponseModeProblem. ResponseModePlain is documented
// as envelope for now. sample is a Go value of the content type, and its
// type is what is documented, so a *doc.Schema or a reflect.Type passed as
// sample documents that Go type; doc.ResponseSchema documents a schema
// written by hand. A pointer sample documents the type it points to; a nil
// sample documents a body without content (an envelope with only its
// header, or JSON null in none mode). A status that never carries a body
// (1xx, 204 and 304) is documented without content in every mode. Only the
// schema is written, never an example, since a zero-value sample is not
// what the route sends.
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
	contentAlone := mode == ResponseModeNone || mode == ResponseModeProblem
	return func(route *doc.Route) {
		response := doc.RouteResponse{Status: status}
		switch {
		case contentAlone && contentType == nil:
			response.Schema = nullBodySchema()
		case contentAlone:
			response.Body = contentType
		default:
			response.Body = envelopeType(contentType)
		}
		route.Responses = append(route.Responses, response)
	}
}

// statusCarriesBody reports whether a response with status may have a body;
// net/http sends none for 1xx, 204 and 304.
func statusCarriesBody(status int) bool {
	return status >= http.StatusOK && status != http.StatusNoContent && status != http.StatusNotModified
}

// DocError documents an error response for status as the instance's built-in
// formatter sends it: an envelope with only its header, or JSON null in
// ResponseModeNone, where an error has no content. ResponseModePlain and
// ResponseModeProblem are documented as envelope for now. An empty
// description documents the HTTP status text. The mode is read as in
// DocSuccess, and only the schema is written.
func (a *API) DocError(status int, description string) doc.RouteOption {
	documentsNull := a.documentedResponseMode() == ResponseModeNone
	return func(route *doc.Route) {
		response := doc.RouteResponse{Status: status, Description: description}
		if documentsNull {
			response.Schema = nullBodySchema()
		} else {
			response.Body = envelope.Error()
		}
		route.Responses = append(route.Responses, response)
	}
}

// documentedResponseMode returns the mode the instance's built-in formatter
// formats with. The default instance has no mode of its own.
func (a *API) documentedResponseMode() ResponseMode {
	if a == nil || a.responseMode == "" {
		return normalizeResponseMode(CurrentResponseMode)
	}
	return a.responseMode
}

// envelopeType builds the shape of envelope.Success[T]() for a content type
// known only at run time, since a method cannot be generic. The struct is
// anonymous, so the spec inlines it while header and content stay $refs.
// A nil contentType gives an envelope with only its header.
func envelopeType(contentType reflect.Type) reflect.Type {
	fields := []reflect.StructField{{
		Name: "Header",
		Type: reflect.TypeOf(envelope.ResponseInfo{}),
		Tag:  `json:"header"`,
	}}
	if contentType != nil {
		fields = append(fields, reflect.StructField{
			Name: "Content",
			Type: reflect.PointerTo(contentType),
			Tag:  `json:"content,omitempty"`,
		})
	}
	return reflect.StructOf(fields)
}

// nullBodySchema describes a JSON null body. OpenAPI 3.0.3 needs a type
// beside nullable, and the enum leaves null as the only valid value.
func nullBodySchema() *doc.Schema {
	return &doc.Schema{Type: "object", Nullable: true, Enum: []any{nil}}
}
