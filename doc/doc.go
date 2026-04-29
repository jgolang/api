// Package doc contains OpenAPI documentation metadata, schema generation, and handlers.
package doc

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
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:  status,
			Body:    body,
			Example: example,
			Headers: append([]ResponseHeaderInfo(nil), headers...),
		})
	}
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

// ResponseWithDescriptionAndExample adds a response model, description, and example.
func ResponseWithDescriptionAndExample(status int, description string, body interface{}, example interface{}) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:      status,
			Description: description,
			Body:        body,
			Example:     example,
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
