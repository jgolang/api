// Package doc contains OpenAPI documentation metadata, schema generation, and handlers.
package doc

import "reflect"

// Summary sets the OpenAPI operation summary.
func Summary(summary string) RouteOption {
	return func(route *Route) {
		route.Summary = summary
	}
}

// OperationID sets the OpenAPI operationId used by client generators.
func OperationID(id string) RouteOption {
	return func(route *Route) {
		route.OperationID = id
	}
}

// Description sets the OpenAPI operation description.
func Description(description string) RouteOption {
	return func(route *Route) {
		route.Description = description
	}
}

// Tags sets the OpenAPI operation tags.
func Tags(tags ...string) RouteOption {
	return func(route *Route) {
		route.Tags = append(route.Tags, tags...)
	}
}

// Body sets the request body model inferred with reflection.
func Body(body interface{}) RouteOption {
	return func(route *Route) {
		route.Body = body
	}
}

// BodyWithExample sets the request body model and an OpenAPI media type example.
func BodyWithExample(body interface{}, example interface{}) RouteOption {
	return func(route *Route) {
		route.Body = body
		route.BodyExample = example
	}
}

// BodySchema sets a manual request body schema.
func BodySchema(schema *Schema) RouteOption {
	return func(route *Route) {
		route.BodySchema = schema
	}
}

// Status adds a response model for a status code.
func Status(status int, body interface{}) RouteOption {
	return ResponseWithDescription(status, "", body)
}

// StatusWithExample adds a response model and an OpenAPI media type example.
func StatusWithExample(status int, body interface{}, example interface{}, headers ...ResponseHeaderInfo) RouteOption {
	return ResponseWithDescriptionAndExample(status, "", body, example, headers...)
}

// Responds adds a response model for a status code.
func Responds(status int, body interface{}) RouteOption {
	return Status(status, body)
}

// StatusWithHeaders adds a response model and documented response headers.
func StatusWithHeaders(status int, body interface{}, headers ...ResponseHeaderInfo) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:  status,
			Body:    body,
			Headers: append([]ResponseHeaderInfo(nil), headers...),
		})
	}
}

// ResponseWithDescription adds a response model and description.
func ResponseWithDescription(status int, description string, body interface{}) RouteOption {
	return ResponseWithDescriptionAndExample(status, description, body, nil)
}

// ResponseWithDescriptionAndExample adds a response model, description, example, and optional headers.
func ResponseWithDescriptionAndExample(status int, description string, body interface{}, example interface{}, headers ...ResponseHeaderInfo) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:      status,
			Description: description,
			Body:        body,
			Example:     example,
			Headers:     append([]ResponseHeaderInfo(nil), headers...),
		})
	}
}

// ResponseSchema adds a response with a manual schema.
func ResponseSchema(status int, description string, schema *Schema) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:      status,
			Description: description,
			Schema:      schema,
		})
	}
}

// ResponseHeader documents a header returned by a response.
func ResponseHeader(name string, typ DataType, description string) ResponseHeaderInfo {
	return ResponseHeaderInfo{
		Name:        name,
		Type:        typ,
		Description: description,
	}
}

// Query adds a query parameter.
func Query(name string, typ DataType, required bool) RouteOption {
	return parameter("query", name, typ, required, "")
}

// QueryWithDescription adds a query parameter with an OpenAPI description.
func QueryWithDescription(name string, typ DataType, required bool, description string) RouteOption {
	return parameter("query", name, typ, required, description)
}

// Header adds a request header parameter.
func Header(name string, typ DataType, required bool) RouteOption {
	return parameter("header", name, typ, required, "")
}

// HeaderWithDescription adds a request header parameter with an OpenAPI description.
func HeaderWithDescription(name string, typ DataType, required bool, description string) RouteOption {
	return parameter("header", name, typ, required, description)
}

// Path adds a path parameter.
func Path(name string, typ DataType, required bool) RouteOption {
	return parameter("path", name, typ, required, "")
}

// PathWithDescription adds a path parameter with an OpenAPI description.
func PathWithDescription(name string, typ DataType, required bool, description string) RouteOption {
	return parameter("path", name, typ, required, description)
}

// QueryEnum documents a query parameter whose accepted values are values,
// written as schema.enum. typ and values must match (doc.Bool with true,
// doc.Int with 10, 20). Repeated values are written once; with no values the
// parameter is documented like QueryWithDescription. Documentation only: the
// request is not validated.
func QueryEnum(name string, typ DataType, required bool, description string, values ...interface{}) RouteOption {
	return enumParameter("query", name, typ, required, description, values)
}

// PathEnum documents a path parameter whose accepted values are values, like
// QueryEnum. Path parameters are always written as required.
func PathEnum(name string, typ DataType, required bool, description string, values ...interface{}) RouteOption {
	return enumParameter("path", name, typ, required, description, values)
}

func enumParameter(in string, name string, typ DataType, required bool, description string, values []interface{}) RouteOption {
	return func(route *Route) {
		param := Parameter{
			Name:        name,
			In:          in,
			Type:        typ,
			Required:    required,
			Description: description,
		}
		if enum := uniqueValues(values); len(enum) > 0 {
			param.Schema = &Schema{Type: string(typ), Enum: enum}
		}
		route.Parameters = append(route.Parameters, param)
	}
}

// uniqueValues returns a new slice with each value once, in first-seen order;
// JSON Schema requires enum items to be unique. DeepEqual avoids the panic ==
// would raise on slice or map values.
func uniqueValues(values []interface{}) []interface{} {
	unique := make([]interface{}, 0, len(values))
	for _, value := range values {
		seen := false
		for _, kept := range unique {
			if reflect.DeepEqual(kept, value) {
				seen = true
				break
			}
		}
		if !seen {
			unique = append(unique, value)
		}
	}
	return unique
}

// Public marks the operation as public: the spec writes security: [], which
// overrides any default security and the operation's Security options.
func Public() RouteOption {
	return func(route *Route) {
		route.Public = true
	}
}

// Security adds a named security requirement to the operation.
func Security(name string) RouteOption {
	return func(route *Route) {
		route.Security = append(route.Security, name)
	}
}

func parameter(in string, name string, typ DataType, required bool, description string) RouteOption {
	return func(route *Route) {
		route.Parameters = append(route.Parameters, Parameter{
			Name:        name,
			In:          in,
			Type:        typ,
			Required:    required,
			Description: description,
		})
	}
}
