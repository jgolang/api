package doc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// OpenAPI is the generated OpenAPI 3 document.
type OpenAPI struct {
	OpenAPI    string                          `json:"openapi"`
	Info       API                             `json:"info"`
	Paths      map[string]map[string]Operation `json:"paths"`
	Components *Components                     `json:"components,omitempty"`
	Security   []map[string][]string           `json:"security,omitempty"`
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
	// Public writes security: [] and ignores Security when set.
	Public bool `json:"-"`
}

// MarshalJSON writes security: [] for a public operation; omitempty would
// otherwise drop the empty list that OpenAPI uses to mean "no security". It
// leaves HTML escaping to the caller's encoder. A struct that embeds Operation
// gets this method too and marshals as Operation.
func (operation Operation) MarshalJSON() ([]byte, error) {
	type plain Operation
	if !operation.Public {
		return marshalNoEscape(plain(operation))
	}
	return marshalNoEscape(struct {
		plain
		Security []map[string][]string `json:"security"`
	}{plain: plain(operation), Security: []map[string][]string{}})
}

// UnmarshalJSON reads security: [] back as Public, so a decoded spec marshals
// to the same document. It replaces the whole Operation; decode into a zero
// value.
func (operation *Operation) UnmarshalJSON(data []byte) error {
	type plain Operation
	var decoded struct {
		plain
		Security *[]map[string][]string `json:"security"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*operation = Operation(decoded.plain)
	if decoded.Security != nil {
		operation.Security = *decoded.Security
		operation.Public = len(*decoded.Security) == 0
	}
	return nil
}

// marshalNoEscape encodes value without HTML escaping; the outer encoder
// escapes the result when it is configured to.
func marshalNoEscape(value interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
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
	Schema   *Schema                  `json:"schema,omitempty"`
	Example  interface{}              `json:"example,omitempty"`
	Examples map[string]ExampleObject `json:"examples,omitempty"`
}

// ExampleObject is an OpenAPI example object.
type ExampleObject struct {
	Summary string      `json:"summary,omitempty"`
	Value   interface{} `json:"value"`
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

// GenerateOpenAPI renders an OpenAPI 3.0 document from a docs.
func GenerateOpenAPI(docs *Docs) OpenAPI {
	doc := OpenAPI{
		OpenAPI: "3.0.3",
		Paths:   make(map[string]map[string]Operation),
	}
	if docs == nil {
		return doc
	}
	doc.Info = docs.API()
	schemas := newComponentSchemaWalker()
	for name, scheme := range docs.SecuritySchemes() {
		ensureComponents(&doc)
		if doc.Components.SecuritySchemes == nil {
			doc.Components.SecuritySchemes = make(map[string]SecurityScheme)
		}
		doc.Components.SecuritySchemes[name] = scheme
	}
	for _, route := range docs.Routes() {
		if route.Hidden {
			continue
		}
		path := normalizeOpenAPIPath(route.Path)
		method := strings.ToLower(route.Method)
		if doc.Paths[path] == nil {
			doc.Paths[path] = make(map[string]Operation)
		}
		doc.Paths[path][method] = buildOperation(route, schemas)
		if route.Public {
			continue
		}
		for _, security := range route.Security {
			ensureSecurityScheme(&doc, security)
		}
	}
	for _, security := range docs.DefaultSecurity() {
		doc.Security = append(doc.Security, map[string][]string{security: {}})
		ensureSecurityScheme(&doc, security)
	}
	if len(schemas.components) > 0 {
		ensureComponents(&doc)
		doc.Components.Schemas = schemas.components
	}
	return doc
}

// ensureSecurityScheme adds a bearer JWT scheme named name unless the docs
// already declare one.
func ensureSecurityScheme(doc *OpenAPI, name string) {
	ensureComponents(doc)
	if doc.Components.SecuritySchemes == nil {
		doc.Components.SecuritySchemes = make(map[string]SecurityScheme)
	}
	if _, ok := doc.Components.SecuritySchemes[name]; !ok {
		doc.Components.SecuritySchemes[name] = SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
	}
}

func buildOperation(route Route, schemas *schemaWalker) Operation {
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
			schema = schemas.schemaForBody(route.Body)
		}
		operation.RequestBody = &RequestBodyObject{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {Schema: schema, Example: route.BodyExample},
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
			schema = schemas.schemaForBody(response.Body)
		}
		if schema != nil {
			media := MediaType{Schema: schema, Example: response.Example}
			if len(response.Examples) > 0 {
				media.Example = nil
				media.Examples = make(map[string]ExampleObject, len(response.Examples))
				for name, value := range response.Examples {
					media.Examples[name] = ExampleObject{Value: value}
				}
			}
			object.Content = map[string]MediaType{"application/json": media}
		}
		operation.Responses[strconv.Itoa(response.Status)] = object
	}
	if len(operation.Responses) == 0 {
		operation.Responses["204"] = ResponseObject{Description: "No Content"}
	}
	operation.Public = route.Public
	if route.Public {
		return operation
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

// schemaForBody returns the schema of a route body or response. A body passed
// as &T{} is a sample value; the pointer does not make the whole body
// nullable, so the body stays a plain $ref.
func (walker *schemaWalker) schemaForBody(value interface{}) *Schema {
	if schema, ok := value.(*Schema); ok {
		return schema
	}
	if value == nil {
		return &Schema{}
	}
	typ := typeOfValue(value)
	for typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return walker.schemaFromReflectType(typ)
}

func (walker *schemaWalker) componentRef(typ reflect.Type) *Schema {
	name := walker.componentName(typ)
	if _, ok := walker.components[name]; !ok {
		walker.components[name] = &Schema{Type: "object"}
		walker.components[name] = walker.structSchema(typ)
	}
	return &Schema{Ref: "#/components/schemas/" + name}
}

// refToAllOf moves a $ref that has sibling keys into allOf, because
// OpenAPI 3.0.3 ignores every key written beside $ref. A plain $ref is
// returned unchanged.
func refToAllOf(schema *Schema) *Schema {
	if schema.Ref == "" || !hasRefSiblings(schema) {
		return schema
	}
	wrapped := *schema
	wrapped.AllOf = []*Schema{{Ref: schema.Ref}}
	wrapped.Ref = ""
	return &wrapped
}

// hasRefSiblings reports whether any key beside $ref is set, such as a
// pointer's nullable or a field tag.
func hasRefSiblings(schema *Schema) bool {
	return !reflect.DeepEqual(*schema, Schema{Ref: schema.Ref})
}

func (walker *schemaWalker) resolveLocalRef(schema *Schema) *Schema {
	if schema != nil && schema.Ref == "" && len(schema.AllOf) == 1 {
		schema = schema.AllOf[0]
	}
	if schema == nil || schema.Ref == "" {
		return schema
	}
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(schema.Ref, prefix) {
		return schema
	}
	resolved := walker.components[strings.TrimPrefix(schema.Ref, prefix)]
	if resolved == nil {
		return schema
	}
	return resolved
}

func (walker *schemaWalker) componentName(typ reflect.Type) string {
	if name, ok := walker.names[typ]; ok {
		return name
	}
	base := schemaTypeName(typ)
	name := base
	for i := 2; ; i++ {
		used, exists := walker.usedNames[name]
		if !exists || used == typ {
			walker.names[typ] = name
			walker.usedNames[name] = typ
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
