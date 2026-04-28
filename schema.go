package api

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
}

// SchemaFromType infers an OpenAPI schema from a Go value or reflect.Type.
func SchemaFromType(value interface{}) *Schema {
	if schema, ok := value.(*Schema); ok {
		return schema
	}
	if value == nil {
		return &Schema{}
	}
	if typ, ok := value.(reflect.Type); ok {
		return schemaFromReflectType(typ, make(map[reflect.Type]bool))
	}
	return schemaFromReflectType(reflect.TypeOf(value), make(map[reflect.Type]bool))
}

func schemaFromReflectType(typ reflect.Type, seen map[reflect.Type]bool) *Schema {
	if typ == nil {
		return &Schema{}
	}
	if typ == reflect.TypeOf(time.Time{}) {
		return &Schema{Type: "string", Format: "date-time"}
	}
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		schema := schemaFromReflectType(typ, seen)
		schema.Nullable = true
		return schema
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
		return &Schema{Type: "array", Items: schemaFromReflectType(typ.Elem(), seen)}
	case reflect.Map:
		schema := &Schema{Type: "object"}
		if typ.Key().Kind() == reflect.String {
			schema.AdditionalProperties = schemaFromReflectType(typ.Elem(), seen)
		}
		return schema
	case reflect.Struct:
		return structSchema(typ, seen)
	case reflect.Interface:
		return &Schema{}
	default:
		return &Schema{Type: "string"}
	}
}

func structSchema(typ reflect.Type, seen map[reflect.Type]bool) *Schema {
	if seen[typ] {
		return &Schema{Type: "object"}
	}
	seen[typ] = true
	defer delete(seen, typ)

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
			embedded := schemaFromReflectType(field.Type, seen)
			for propName, propSchema := range embedded.Properties {
				schema.Properties[propName] = propSchema
			}
			schema.Required = append(schema.Required, embedded.Required...)
			continue
		}
		if name == "" {
			name = field.Name
		}
		fieldSchema := schemaFromReflectType(field.Type, seen)
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
