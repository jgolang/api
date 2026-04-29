// Package doc contains OpenAPI documentation metadata, schema generation, and handlers.
package doc

// RequestOf documents a JSON request envelope with a concrete content payload type.
type RequestOf[T any] struct {
	Header  RequestInfo `json:"header,omitempty"`
	Content *T          `json:"content,omitempty"`
}

// ResponseOf documents a JSON response envelope with a concrete content payload type.
type ResponseOf[T any] struct {
	Header  ResponseInfo `json:"header,omitempty"`
	Content *T           `json:"content,omitempty"`
}

// ErrorResponse documents an error response without a content payload.
type ErrorResponse struct {
	Header ResponseInfo `json:"header,omitempty"`
}

// RequestInfo documents the request envelope header.
type RequestInfo struct {
	UUID            string `json:"uuid,omitempty" example:"ADAD3-ADD33-AFSFK"`
	DeviceType      string `json:"device_type,omitempty" example:"phone"`
	DeviceBrand     string `json:"device_brand,omitempty" example:"Samsung"`
	DeviceModel     string `json:"device_model,omitempty" example:"A11"`
	OS              string `json:"os,omitempty" example:"android"`
	OSVersion       string `json:"os_version,omitempty" example:"14"`
	Lang            string `json:"lang,omitempty" example:"es"`
	Timezone        string `json:"timezone,omitempty" example:"America/Mexico_City"`
	AppVersion      string `json:"app_version,omitempty" example:"3.0.0"`
	AppBuildVersion string `json:"app_build_version,omitempty" example:"1.0.0.10"`
	AppName         string `json:"app_name,omitempty" example:"My App"`
	SecurityToken   string `json:"token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	DeviceId        string `json:"device_id,omitempty" example:"device-123"`
	DeviceSerial    string `json:"device_serial,omitempty" example:"serial-123"`
	Latitude        string `json:"lat,omitempty" example:"19.4326"`
	Longitude       string `json:"lon,omitempty" example:"-99.1332"`
}

// ResponseInfo documents the response envelope header.
type ResponseInfo struct {
	Type    string            `json:"type" example:"success"`
	Title   string            `json:"title,omitempty" example:"Success"`
	Message string            `json:"message,omitempty" example:"Operation completed"`
	Code    string            `json:"code,omitempty" example:"OK"`
	Token   string            `json:"token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	Action  string            `json:"action,omitempty" example:"refresh"`
	EventID string            `json:"event_id,omitempty" example:"f716243f2c92df55fcd8f67018b1dcfb"`
	Info    map[string]string `json:"info,omitempty" example:"{\"field\":\"value\"}"`
}

// Request returns a typed request wrapper for OpenAPI documentation.
func Request[T any]() RequestOf[T] {
	return RequestOf[T]{}
}

// Success returns a typed success response wrapper for OpenAPI documentation.
func Success[T any]() ResponseOf[T] {
	return ResponseOf[T]{}
}

// Error returns an error response wrapper for OpenAPI documentation.
func Error() ErrorResponse {
	return ErrorResponse{}
}

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
	return func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{
			Status:      status,
			Description: description,
			Body:        body,
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
