package doc

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
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
	// Minimum is the smallest allowed number, read from a minimum tag.
	Minimum *float64 `json:"minimum,omitempty"`
	// Maximum is the largest allowed number, read from a maximum tag.
	Maximum *float64 `json:"maximum,omitempty"`
	// MinLength is the shortest allowed string, read from a minLength tag.
	MinLength *int `json:"minLength,omitempty"`
	// MaxLength is the longest allowed string, read from a maxLength tag.
	MaxLength *int `json:"maxLength,omitempty"`
	// Pattern is the regular expression a string must match, read from a
	// pattern tag.
	Pattern string `json:"pattern,omitempty"`
	// Default is the value the server uses when the field is absent, read
	// from a default tag and typed like the field.
	Default interface{} `json:"default,omitempty"`
	// Deprecated marks a field clients should stop using, read from a
	// deprecated tag.
	Deprecated bool `json:"deprecated,omitempty"`
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
	// tagErrors holds the constraint tags left out because their value does
	// not parse, for CheckSchemaTags.
	tagErrors []error
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
		walker.applyConstraintTags(fieldSchema, typ, field)
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

// constraintTag reads one constraint tag into a schema. apply returns an
// error, and leaves the schema unchanged, when the tag value does not parse
// for the field type.
type constraintTag struct {
	name  string
	apply func(schema *Schema, fieldType reflect.Type, raw string) error
}

func constraintTags() []constraintTag {
	return []constraintTag{
		{"enum", applyEnumTag},
		{"default", applyDefaultTag},
		{"minimum", applyMinimumTag},
		{"maximum", applyMaximumTag},
		{"minLength", applyMinLengthTag},
		{"maxLength", applyMaxLengthTag},
		{"pattern", applyPatternTag},
		{"deprecated", applyDeprecatedTag},
	}
}

// applyConstraintTags reads the constraint tags of field, a field of owner.
// A value that does not parse is left out rather than guessed, and recorded
// for CheckSchemaTags.
func (walker *schemaWalker) applyConstraintTags(schema *Schema, owner reflect.Type, field reflect.StructField) {
	for _, tag := range constraintTags() {
		raw := field.Tag.Get(tag.name)
		if raw == "" {
			continue
		}
		if err := tag.apply(schema, field.Type, raw); err != nil {
			walker.tagErrors = append(walker.tagErrors, errors.NewC(codes.CfgInvalid,
				"doc: field %s.%s has an invalid %s tag %q: %w", owner, field.Name, tag.name, raw, err))
		}
	}
}

func applyEnumTag(schema *Schema, fieldType reflect.Type, raw string) error {
	typ := dereferenceType(fieldType)
	if !isScalarType(typ) {
		return fmt.Errorf("enum needs a scalar field, not %s", typ)
	}
	parts := strings.Split(raw, ",")
	values := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		value, err := parseScalar(typ, strings.TrimSpace(part))
		if err != nil {
			return err
		}
		values = append(values, value)
	}
	if schema.Nullable {
		// OpenAPI 3.0.3 checks enum after nullable, so a pointer field's
		// null must be listed or validators reject it.
		values = append(values, nil)
	}
	schema.Enum = values
	return nil
}

// applyDefaultTag types a scalar default like the field and reads any other
// default as JSON.
func applyDefaultTag(schema *Schema, fieldType reflect.Type, raw string) error {
	typ := dereferenceType(fieldType)
	if isScalarType(typ) {
		value, err := parseScalar(typ, raw)
		if err != nil {
			return err
		}
		schema.Default = value
		return nil
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return err
	}
	schema.Default = value
	return nil
}

func applyMinimumTag(schema *Schema, _ reflect.Type, raw string) error {
	value, err := parseFiniteFloat(raw, 64)
	if err != nil {
		return err
	}
	schema.Minimum = &value
	return nil
}

func applyMaximumTag(schema *Schema, _ reflect.Type, raw string) error {
	value, err := parseFiniteFloat(raw, 64)
	if err != nil {
		return err
	}
	schema.Maximum = &value
	return nil
}

func applyMinLengthTag(schema *Schema, _ reflect.Type, raw string) error {
	length, err := parseLength(raw)
	if err != nil {
		return err
	}
	schema.MinLength = &length
	return nil
}

func applyMaxLengthTag(schema *Schema, _ reflect.Type, raw string) error {
	length, err := parseLength(raw)
	if err != nil {
		return err
	}
	schema.MaxLength = &length
	return nil
}

func applyPatternTag(schema *Schema, _ reflect.Type, raw string) error {
	if _, err := regexp.Compile(raw); err != nil {
		return err
	}
	schema.Pattern = raw
	return nil
}

func applyDeprecatedTag(schema *Schema, _ reflect.Type, raw string) error {
	deprecated, err := strconv.ParseBool(raw)
	if err != nil {
		return err
	}
	schema.Deprecated = deprecated
	return nil
}

func dereferenceType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ
}

// isScalarType reports whether typ is documented as a boolean, number or
// string, the types whose tag values parseScalar can type.
func isScalarType(typ reflect.Type) bool {
	if typ == reflect.TypeOf(time.Time{}) {
		return true
	}
	switch typ.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// parseScalar types raw like a value of typ, so an int field writes JSON
// numbers and a value out of the field's range is rejected.
func parseScalar(typ reflect.Type, raw string) (interface{}, error) {
	if typ == reflect.TypeOf(time.Time{}) {
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			return nil, err
		}
		return raw, nil
	}
	switch typ.Kind() {
	case reflect.Bool:
		return boxResult(strconv.ParseBool(raw))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return boxResult(strconv.ParseInt(raw, 10, typ.Bits()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return boxResult(strconv.ParseUint(raw, 10, typ.Bits()))
	case reflect.Float32, reflect.Float64:
		return boxResult(parseFiniteFloat(raw, typ.Bits()))
	default:
		return raw, nil
	}
}

func boxResult[T any](value T, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}
	return value, nil
}

// parseFiniteFloat rejects NaN and infinities, which JSON cannot encode. It
// keeps the 64-bit value of a float32 so 0.1 is written as 0.1.
func parseFiniteFloat(raw string, bitSize int) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%q is not a finite number", raw)
	}
	if bitSize == 32 && math.Abs(value) > math.MaxFloat32 {
		return 0, fmt.Errorf("%q is out of range for float32", raw)
	}
	return value, nil
}

func parseLength(raw string) (int, error) {
	length, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if length < 0 {
		return 0, fmt.Errorf("length %d is negative", length)
	}
	return length, nil
}
