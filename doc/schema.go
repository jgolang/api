package doc

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

// Schema is a small OpenAPI schema object.
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Description          string             `json:"description,omitempty"`
	Enum                 []interface{}      `json:"enum,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Example              interface{}        `json:"example,omitempty"`
	// AllOf holds a $ref that has sibling keys, such as nullable or a
	// description, because OpenAPI 3.0.3 ignores every key beside $ref.
	AllOf []*Schema `json:"allOf,omitempty"`
}

// SchemaFromType infers an OpenAPI schema from a Go value or reflect.Type.
func SchemaFromType(value interface{}) *Schema {
	if schema, ok := value.(*Schema); ok {
		return schema
	}
	if value == nil {
		return &Schema{}
	}
	return newInlineSchemaWalker().schemaFromReflectType(typeOfValue(value))
}

// typeOfValue returns value itself when it is already a reflect.Type.
func typeOfValue(value interface{}) reflect.Type {
	if typ, ok := value.(reflect.Type); ok {
		return typ
	}
	return reflect.TypeOf(value)
}

// schemaWalker turns a Go type into a schema. With components set it writes
// each named struct once under #/components/schemas and returns a $ref to it;
// without components it expands every struct in place, so the result stands
// alone.
type schemaWalker struct {
	components map[string]*Schema
	names      map[reflect.Type]string
	usedNames  map[string]reflect.Type
	// seen holds the structs being expanded inline, so a recursive type
	// stops at its second visit instead of looping. It is nil with
	// components, where the component placeholder stops recursion and an
	// anonymous struct must still expand inside a component it reaches.
	seen map[reflect.Type]bool
}

func newInlineSchemaWalker() *schemaWalker {
	return &schemaWalker{seen: make(map[reflect.Type]bool)}
}

func newComponentSchemaWalker() *schemaWalker {
	return &schemaWalker{
		components: make(map[string]*Schema),
		names:      make(map[reflect.Type]string),
		usedNames:  make(map[string]reflect.Type),
	}
}

func (walker *schemaWalker) schemaFromReflectType(typ reflect.Type) *Schema {
	if typ == nil {
		return &Schema{}
	}
	if typ == reflect.TypeOf(time.Time{}) {
		return &Schema{Type: "string", Format: "date-time"}
	}
	if typ.Kind() == reflect.Ptr {
		// Every walk returns a fresh schema, so marking it nullable cannot
		// leak into a component or another field.
		schema := walker.schemaFromReflectType(typ.Elem())
		schema.Nullable = true
		return refToAllOf(schema)
	}
	if walker.components != nil && typ.Kind() == reflect.Struct && typ.Name() != "" {
		return walker.componentRef(typ)
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
		return &Schema{Type: "array", Items: walker.schemaFromReflectType(typ.Elem())}
	case reflect.Map:
		schema := &Schema{Type: "object"}
		if typ.Key().Kind() == reflect.String {
			schema.AdditionalProperties = walker.schemaFromReflectType(typ.Elem())
		}
		return schema
	case reflect.Struct:
		return walker.structSchema(typ)
	case reflect.Interface:
		return &Schema{}
	default:
		return &Schema{Type: "string"}
	}
}

func (walker *schemaWalker) structSchema(typ reflect.Type) *Schema {
	if walker.seen != nil {
		if walker.seen[typ] {
			return &Schema{Type: "object"}
		}
		walker.seen[typ] = true
		defer delete(walker.seen, typ)
	}

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
			embedded := walker.resolveLocalRef(walker.schemaFromReflectType(field.Type))
			for propName, propSchema := range embedded.Properties {
				schema.Properties[propName] = propSchema
			}
			schema.Required = append(schema.Required, embedded.Required...)
			continue
		}
		if name == "" {
			name = field.Name
		}
		fieldSchema := walker.schemaFromReflectType(field.Type)
		applySchemaTags(fieldSchema, field)
		schema.Properties[name] = refToAllOf(fieldSchema)
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

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	omitEmpty := false
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func applySchemaTags(schema *Schema, field reflect.StructField) {
	if description := field.Tag.Get("description"); description != "" {
		schema.Description = description
	}
	if format := field.Tag.Get("format"); format != "" {
		schema.Format = format
	}
	if example := field.Tag.Get("example"); example != "" {
		schema.Example = parseExample(example)
	}
}

func parseExample(value string) interface{} {
	var parsed interface{}
	if err := json.Unmarshal([]byte(value), &parsed); err == nil {
		return parsed
	}
	return value
}

func isRequiredField(field reflect.StructField, omitEmpty bool) bool {
	if hasValidateRule(field.Tag.Get("validate"), "required") {
		return true
	}
	return !omitEmpty && !isNullable(field.Type)
}

func hasValidateRule(tag string, rule string) bool {
	for _, part := range strings.Split(tag, ",") {
		name := strings.SplitN(part, "=", 2)[0]
		if name == rule {
			return true
		}
	}
	return false
}

func isNullable(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface:
		return true
	default:
		return false
	}
}
