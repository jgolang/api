package api

import (
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	Schemas         map[string]*Schema        `json:"schemas,omitempty"`
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
	OperationID string                    `json:"operationId,omitempty"`
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
	Description string                   `json:"description"`
	Headers     map[string]OpenAPIHeader `json:"headers,omitempty"`
	Content     map[string]MediaType     `json:"content,omitempty"`
}

// OpenAPIHeader is an OpenAPI header object.
type OpenAPIHeader struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
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
	schemas := newOpenAPISchemaBuilder()
	for name, scheme := range registry.SecuritySchemes() {
		ensureComponents(&doc)
		if doc.Components.SecuritySchemes == nil {
			doc.Components.SecuritySchemes = make(map[string]SecurityScheme)
		}
		doc.Components.SecuritySchemes[name] = scheme
	}
	for _, route := range registry.Routes() {
		path := normalizeOpenAPIPath(route.Path)
		method := strings.ToLower(route.Method)
		if doc.Paths[path] == nil {
			doc.Paths[path] = make(map[string]Operation)
		}
		doc.Paths[path][method] = buildOperation(route, schemas)
		for _, security := range route.Security {
			ensureComponents(&doc)
			if doc.Components.SecuritySchemes == nil {
				doc.Components.SecuritySchemes = make(map[string]SecurityScheme)
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
	if len(schemas.components) > 0 {
		ensureComponents(&doc)
		doc.Components.Schemas = schemas.components
	}
	return doc
}

func buildOperation(route Route, schemas *openAPISchemaBuilder) Operation {
	operation := Operation{
		OperationID: route.OperationID,
		Summary:     route.Summary,
		Description: route.Description,
		Tags:        route.Tags,
		Responses:   make(map[string]ResponseObject),
	}
	for _, param := range route.Parameters {
		schema := param.Schema
		if schema == nil {
			schema = schemaFromDataType(param.Type)
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
			schema = schemas.SchemaFromType(route.Body)
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
		if len(response.Headers) > 0 {
			object.Headers = make(map[string]OpenAPIHeader, len(response.Headers))
			for _, header := range response.Headers {
				schema := header.Schema
				if schema == nil {
					schema = schemaFromDataType(header.Type)
				}
				object.Headers[header.Name] = OpenAPIHeader{
					Description: header.Description,
					Schema:      schema,
				}
			}
		}
		schema := response.Schema
		if schema == nil && response.Body != nil {
			schema = schemas.SchemaFromType(response.Body)
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

func schemaFromDataType(typ DataType) *Schema {
	schema := &Schema{Type: string(typ)}
	if typ == Int {
		schema.Format = "int32"
	}
	return schema
}

func ensureComponents(doc *OpenAPI) {
	if doc.Components == nil {
		doc.Components = &Components{}
	}
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

type openAPISchemaBuilder struct {
	components map[string]*Schema
	names      map[reflect.Type]string
	usedNames  map[string]reflect.Type
}

func newOpenAPISchemaBuilder() *openAPISchemaBuilder {
	return &openAPISchemaBuilder{
		components: make(map[string]*Schema),
		names:      make(map[reflect.Type]string),
		usedNames:  make(map[string]reflect.Type),
	}
}

func (builder *openAPISchemaBuilder) SchemaFromType(value interface{}) *Schema {
	if schema, ok := value.(*Schema); ok {
		return schema
	}
	if value == nil {
		return &Schema{}
	}
	if typ, ok := value.(reflect.Type); ok {
		return builder.schemaFromReflectType(typ)
	}
	return builder.schemaFromReflectType(reflect.TypeOf(value))
}

func (builder *openAPISchemaBuilder) schemaFromReflectType(typ reflect.Type) *Schema {
	if typ == nil {
		return &Schema{}
	}
	if typ == reflect.TypeOf(time.Time{}) {
		return &Schema{Type: "string", Format: "date-time"}
	}
	if typ.Kind() == reflect.Ptr {
		schema := cloneSchema(builder.schemaFromReflectType(typ.Elem()))
		schema.Nullable = true
		return schema
	}
	if typ.Kind() == reflect.Struct && typ.Name() != "" {
		return builder.componentRef(typ)
	}
	switch typ.Kind() {
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return &Schema{Type: "integer", Format: "int32"}
	case reflect.Int64:
		return &Schema{Type: "integer", Format: "int64"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &Schema{Type: "integer", Format: "int32"}
	case reflect.Uint64:
		return &Schema{Type: "integer", Format: "int64"}
	case reflect.Float32:
		return &Schema{Type: "number", Format: "float"}
	case reflect.Float64:
		return &Schema{Type: "number", Format: "double"}
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Slice, reflect.Array:
		return &Schema{Type: "array", Items: builder.schemaFromReflectType(typ.Elem())}
	case reflect.Map:
		schema := &Schema{Type: "object"}
		if typ.Key().Kind() == reflect.String {
			schema.AdditionalProperties = builder.schemaFromReflectType(typ.Elem())
		}
		return schema
	case reflect.Struct:
		return builder.structSchema(typ)
	case reflect.Interface:
		return &Schema{}
	default:
		return &Schema{Type: "string"}
	}
}

func (builder *openAPISchemaBuilder) componentRef(typ reflect.Type) *Schema {
	name := builder.componentName(typ)
	if _, ok := builder.components[name]; !ok {
		builder.components[name] = &Schema{Type: "object"}
		builder.components[name] = builder.structSchema(typ)
	}
	return &Schema{Ref: "#/components/schemas/" + name}
}

func (builder *openAPISchemaBuilder) structSchema(typ reflect.Type) *Schema {
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		name, omitEmpty, skip := jsonFieldName(field)
		if skip {
			continue
		}
		if field.Anonymous && name == "" {
			embedded := builder.schemaFromReflectType(field.Type)
			embedded = builder.resolveLocalRef(embedded)
			for propName, propSchema := range embedded.Properties {
				schema.Properties[propName] = propSchema
			}
			schema.Required = append(schema.Required, embedded.Required...)
			continue
		}
		if name == "" {
			name = field.Name
		}
		fieldSchema := builder.schemaFromReflectType(field.Type)
		applySchemaTags(fieldSchema, field)
		schema.Properties[name] = fieldSchema
		if isRequiredField(field, omitEmpty) {
			schema.Required = append(schema.Required, name)
		}
	}
	if len(schema.Properties) == 0 {
		schema.Properties = nil
	}
	if len(schema.Required) == 0 {
		schema.Required = nil
	}
	return schema
}

func (builder *openAPISchemaBuilder) resolveLocalRef(schema *Schema) *Schema {
	if schema == nil || schema.Ref == "" {
		return schema
	}
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(schema.Ref, prefix) {
		return schema
	}
	resolved := builder.components[strings.TrimPrefix(schema.Ref, prefix)]
	if resolved == nil {
		return schema
	}
	return resolved
}

func (builder *openAPISchemaBuilder) componentName(typ reflect.Type) string {
	if name, ok := builder.names[typ]; ok {
		return name
	}
	base := schemaTypeName(typ)
	name := base
	for i := 2; ; i++ {
		used, exists := builder.usedNames[name]
		if !exists || used == typ {
			builder.names[typ] = name
			builder.usedNames[name] = typ
			return name
		}
		name = base + strconv.Itoa(i)
	}
}

func schemaTypeName(typ reflect.Type) string {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Slice, reflect.Array:
		return "ArrayOf" + schemaTypeName(typ.Elem())
	default:
		return schemaNameFromString(typ.Name())
	}
}

func schemaNameFromString(value string) string {
	if value == "" {
		return "Schema"
	}
	if open := strings.Index(value, "["); open >= 0 && strings.HasSuffix(value, "]") {
		base := schemaNameFromString(value[:open])
		args := value[open+1 : len(value)-1]
		for _, arg := range splitGenericArgs(args) {
			base += schemaNameFromTypeString(arg)
		}
		return base
	}
	return exportedIdentifier(lastTypeSegment(value))
}

func schemaNameFromTypeString(value string) string {
	value = strings.TrimSpace(value)
	for strings.HasPrefix(value, "*") {
		value = strings.TrimPrefix(value, "*")
	}
	if strings.HasPrefix(value, "[]") {
		return "ArrayOf" + schemaNameFromTypeString(strings.TrimPrefix(value, "[]"))
	}
	return schemaNameFromString(value)
}

func splitGenericArgs(value string) []string {
	var args []string
	start := 0
	depth := 0
	for i, char := range value {
		switch char {
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(value[start:]))
	return args
}

func lastTypeSegment(value string) string {
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		value = value[slash+1:]
	}
	if dot := strings.LastIndex(value, "."); dot >= 0 {
		value = value[dot+1:]
	}
	return value
}

func exportedIdentifier(value string) string {
	var builder strings.Builder
	upperNext := true
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			upperNext = true
			continue
		}
		if builder.Len() == 0 && unicode.IsDigit(char) {
			builder.WriteString("Schema")
		}
		if upperNext {
			builder.WriteRune(unicode.ToUpper(char))
			upperNext = false
			continue
		}
		builder.WriteRune(char)
	}
	if builder.Len() == 0 {
		return "Schema"
	}
	return builder.String()
}
