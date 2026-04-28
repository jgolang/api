package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// OpenAPI is the generated OpenAPI 3 document.
type OpenAPI struct {
	OpenAPI    string                          `json:"openapi"`
	Info       Info                            `json:"info"`
	Paths      map[string]map[string]Operation `json:"paths"`
	Components *Components                     `json:"components,omitempty"`
}

// Components contains reusable OpenAPI components.
type Components struct {
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme is an OpenAPI security scheme object.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
}

// Operation is an OpenAPI operation object.
type Operation struct {
	Summary     string                    `json:"summary,omitempty"`
	Description string                    `json:"description,omitempty"`
	Tags        []string                  `json:"tags,omitempty"`
	Parameters  []OpenAPIParameter        `json:"parameters,omitempty"`
	RequestBody *RequestBodyObject        `json:"requestBody,omitempty"`
	Responses   map[string]ResponseObject `json:"responses"`
	Security    []map[string][]string     `json:"security,omitempty"`
}

// OpenAPIParameter is an OpenAPI parameter object.
type OpenAPIParameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBodyObject is an OpenAPI requestBody object.
type RequestBodyObject struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

// MediaType is an OpenAPI media type object.
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// ResponseObject is an OpenAPI response object.
type ResponseObject struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// GenerateOpenAPI renders an OpenAPI 3.0 document from a registry.
func GenerateOpenAPI(registry *Registry) OpenAPI {
	doc := OpenAPI{
		OpenAPI: "3.0.3",
		Paths:   make(map[string]map[string]Operation),
	}
	if registry == nil {
		return doc
	}
	doc.Info = registry.Info()
	for name, scheme := range registry.SecuritySchemes() {
		if doc.Components == nil {
			doc.Components = &Components{SecuritySchemes: make(map[string]SecurityScheme)}
		}
		doc.Components.SecuritySchemes[name] = scheme
	}
	for _, route := range registry.Routes() {
		path := normalizeOpenAPIPath(route.Path)
		method := strings.ToLower(route.Method)
		if doc.Paths[path] == nil {
			doc.Paths[path] = make(map[string]Operation)
		}
		doc.Paths[path][method] = buildOperation(route)
		for _, security := range route.Security {
			if doc.Components == nil {
				doc.Components = &Components{SecuritySchemes: make(map[string]SecurityScheme)}
			}
			if _, ok := doc.Components.SecuritySchemes[security]; !ok {
				doc.Components.SecuritySchemes[security] = SecurityScheme{
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
				}
			}
		}
	}
	return doc
}

func buildOperation(route Route) Operation {
	operation := Operation{
		Summary:     route.Summary,
		Description: route.Description,
		Tags:        route.Tags,
		Responses:   make(map[string]ResponseObject),
	}
	for _, param := range route.Parameters {
		schema := param.Schema
		if schema == nil {
			schema = &Schema{Type: string(param.Type)}
			if param.Type == Int {
				schema.Format = "int32"
			}
		}
		operation.Parameters = append(operation.Parameters, OpenAPIParameter{
			Name:        param.Name,
			In:          param.In,
			Required:    param.Required || param.In == "path",
			Description: param.Description,
			Schema:      schema,
		})
	}
	sort.Slice(operation.Parameters, func(i, j int) bool {
		if operation.Parameters[i].In == operation.Parameters[j].In {
			return operation.Parameters[i].Name < operation.Parameters[j].Name
		}
		return operation.Parameters[i].In < operation.Parameters[j].In
	})
	if route.Body != nil || route.BodySchema != nil {
		schema := route.BodySchema
		if schema == nil {
			schema = SchemaFromType(route.Body)
		}
		operation.RequestBody = &RequestBodyObject{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {Schema: schema},
			},
		}
	}
	for _, response := range route.Responses {
		description := response.Description
		if description == "" {
			description = http.StatusText(response.Status)
		}
		if description == "" {
			description = "Response"
		}
		object := ResponseObject{Description: description}
		schema := response.Schema
		if schema == nil && response.Body != nil {
			schema = SchemaFromType(response.Body)
		}
		if schema != nil {
			object.Content = map[string]MediaType{
				"application/json": {Schema: schema},
			}
		}
		operation.Responses[strconv.Itoa(response.Status)] = object
	}
	if len(operation.Responses) == 0 {
		operation.Responses["204"] = ResponseObject{Description: "No Content"}
	}
	for _, security := range route.Security {
		operation.Security = append(operation.Security, map[string][]string{security: {}})
	}
	return operation
}

func normalizeOpenAPIPath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
