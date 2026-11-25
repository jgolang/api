package doc

import (
	"encoding/json"
	stderrors "errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

type constraintFixture struct {
	TargetType string         `json:"target_type" enum:"goal, project,task ,note"`
	Priority   int            `json:"priority" enum:"1,2,3" default:"2"`
	Level      *int8          `json:"level,omitempty" enum:"1,2,3"`
	Unsigned   uint16         `json:"unsigned" enum:"1,2"`
	Ratio      float64        `json:"ratio" enum:"0.5,1.5"`
	Flag       bool           `json:"flag" enum:"true"`
	At         time.Time      `json:"at" enum:"2026-01-01T00:00:00Z"`
	Limit      int            `json:"limit" default:"10"`
	Zero       int            `json:"zero" default:"0"`
	Enabled    bool           `json:"enabled" default:"false"`
	Status     string         `json:"status" default:"open"`
	Meta       map[string]int `json:"meta,omitempty" default:"{\"a\":1}"`
	Score      float64        `json:"score" minimum:"0" maximum:"99.5"`
	Name       string         `json:"name" minLength:"1" maxLength:"80"`
	Slug       string         `json:"slug" pattern:"^[a-z]+$"`
	Old        string         `json:"old" deprecated:"true"`
	Address    schemaAddress  `json:"address" deprecated:"true"`
}

type badConstraintFixture struct {
	Priority   int               `json:"priority" enum:"1,two"`
	Small      int8              `json:"small" enum:"300"`
	Tags       []string          `json:"tags" enum:"a,b"`
	Nested     schemaAddress     `json:"nested" enum:"a"`
	Labels     map[string]string `json:"labels" enum:"a"`
	Limit      int               `json:"limit" default:"ten"`
	Meta       map[string]int    `json:"meta" default:"{bad"`
	Low        float64           `json:"low" minimum:"low"`
	High       float64           `json:"high" maximum:"NaN"`
	Short      string            `json:"short" minLength:"-1"`
	Long       string            `json:"long" maxLength:"1.5"`
	Slug       string            `json:"slug" pattern:"[a-"`
	Deprecated string            `json:"deprecated" deprecated:"yes"`
	Due        time.Time         `json:"due" default:"x"`
}

type badEnumFixture struct {
	Priority int `json:"priority" enum:"1,two"`
}

type plainFixture struct {
	Title string `json:"title" description:"Task title"`
}

// componentProperties returns the properties of the component that docs
// generates for body.
func componentProperties(t *testing.T, body interface{}) map[string]*Schema {
	t.Helper()
	docs := New(API{Title: "constraints", Version: "1"})
	docs.Register("POST", "/items", Body(body))

	spec := GenerateOpenAPI(docs)
	name := schemaTypeName(reflect.TypeOf(body))
	component := spec.Components.Schemas[name]
	if component == nil {
		t.Fatalf("component %s missing: %#v", name, spec.Components.Schemas)
	}
	return component.Properties
}

func marshalSchema(t *testing.T, schema *Schema) string {
	t.Helper()
	got, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	return string(got)
}

var constraintTagCases = []struct {
	property string
	want     string
}{
	{"target_type", `{"type":"string","enum":["goal","project","task","note"]}`},
	{"priority", `{"type":"integer","format":"int32","enum":[1,2,3],"default":2}`},
	{"level", `{"type":"integer","format":"int32","enum":[1,2,3,null],"nullable":true}`},
	{"unsigned", `{"type":"integer","format":"int32","enum":[1,2]}`},
	{"ratio", `{"type":"number","format":"double","enum":[0.5,1.5]}`},
	{"flag", `{"type":"boolean","enum":[true]}`},
	{"at", `{"type":"string","format":"date-time","enum":["2026-01-01T00:00:00Z"]}`},
	{"limit", `{"type":"integer","format":"int32","default":10}`},
	{"zero", `{"type":"integer","format":"int32","default":0}`},
	{"enabled", `{"type":"boolean","default":false}`},
	{"status", `{"type":"string","default":"open"}`},
	{"meta", `{"type":"object","additionalProperties":{"type":"integer","format":"int32"},"default":{"a":1}}`},
	{"score", `{"type":"number","format":"double","minimum":0,"maximum":99.5}`},
	{"name", `{"type":"string","minLength":1,"maxLength":80}`},
	{"slug", `{"type":"string","pattern":"^[a-z]+$"}`},
	{"old", `{"type":"string","deprecated":true}`},
}

func TestGenerateOpenAPIWritesConstraintTags(t *testing.T) {
	properties := componentProperties(t, constraintFixture{})

	for _, tc := range constraintTagCases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}

func TestGenerateOpenAPIMovesDeprecatedRefIntoAllOf(t *testing.T) {
	properties := componentProperties(t, constraintFixture{})

	got := marshalSchema(t, properties["address"])

	want := `{"deprecated":true,"allOf":[{"$ref":"#/components/schemas/SchemaAddress"}]}`
	if got != want {
		t.Fatalf("address = %s, want %s", got, want)
	}
}

func TestSchemaFromTypeWritesConstraintTags(t *testing.T) {
	properties := SchemaFromType(constraintFixture{}).Properties

	for _, tc := range constraintTagCases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
	if !properties["address"].Deprecated {
		t.Fatalf("inline address is not deprecated: %#v", properties["address"])
	}
}

func TestConstraintTagValuesAreTypedByFieldKind(t *testing.T) {
	properties := SchemaFromType(constraintFixture{}).Properties
	cases := []struct {
		property string
		got      interface{}
		want     interface{}
	}{
		{"priority enum", properties["priority"].Enum, []interface{}{int64(1), int64(2), int64(3)}},
		{"priority default", properties["priority"].Default, int64(2)},
		{"level enum", properties["level"].Enum, []interface{}{int64(1), int64(2), int64(3), nil}},
		{"unsigned enum", properties["unsigned"].Enum, []interface{}{uint64(1), uint64(2)}},
		{"ratio enum", properties["ratio"].Enum, []interface{}{0.5, 1.5}},
		{"flag enum", properties["flag"].Enum, []interface{}{true}},
		{"enabled default", properties["enabled"].Default, false},
		{"meta default", properties["meta"].Default, map[string]interface{}{"a": float64(1)}},
	}

	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			if !reflect.DeepEqual(tc.got, tc.want) {
				t.Fatalf("%s = %#v, want %#v", tc.property, tc.got, tc.want)
			}
		})
	}
}

func TestGenerateOpenAPILeavesOutMalformedConstraintTags(t *testing.T) {
	properties := componentProperties(t, badConstraintFixture{})
	cases := []struct {
		property string
		want     string
	}{
		{"priority", `{"type":"integer","format":"int32"}`},
		{"small", `{"type":"integer","format":"int32"}`},
		{"tags", `{"type":"array","items":{"type":"string"}}`},
		{"nested", `{"$ref":"#/components/schemas/SchemaAddress"}`},
		{"labels", `{"type":"object","additionalProperties":{"type":"string"}}`},
		{"limit", `{"type":"integer","format":"int32"}`},
		{"meta", `{"type":"object","additionalProperties":{"type":"integer","format":"int32"}}`},
		{"low", `{"type":"number","format":"double"}`},
		{"high", `{"type":"number","format":"double"}`},
		{"short", `{"type":"string"}`},
		{"long", `{"type":"string"}`},
		{"slug", `{"type":"string"}`},
		{"deprecated", `{"type":"string"}`},
		{"due", `{"type":"string","format":"date-time"}`},
	}

	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}

func TestSchemaFromTypeLeavesOutMalformedConstraintTags(t *testing.T) {
	schema := SchemaFromType(badEnumFixture{})

	if got := schema.Properties["priority"].Enum; got != nil {
		t.Fatalf("enum = %#v, want none", got)
	}
}

func TestCheckSchemaTagsReturnsNilForValidOrMissingDocs(t *testing.T) {
	clean := New(API{Title: "clean", Version: "1"})
	clean.Register("POST", "/items", Body(constraintFixture{}), Status(201, plainFixture{}))
	cases := []struct {
		name string
		docs *Docs
	}{
		{"nil docs", nil},
		{"no routes", New(API{Title: "empty", Version: "1"})},
		{"valid tags", clean},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckSchemaTags(tc.docs); err != nil {
				t.Fatalf("CheckSchemaTags() = %v, want nil", err)
			}
		})
	}
}

func TestCheckSchemaTagsReportsMalformedEnum(t *testing.T) {
	docs := New(API{Title: "bad", Version: "1"})
	docs.Register("POST", "/items", Body(badEnumFixture{}))

	err := CheckSchemaTags(docs)

	if err == nil {
		t.Fatal("CheckSchemaTags() = nil, want an error")
	}
	for _, part := range []string{"badEnumFixture", "Priority", "enum", `"two"`} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("error %q does not mention %s", err, part)
		}
	}
	var appErr *errors.Error
	if !stderrors.As(err, &appErr) {
		t.Fatalf("error %T is not a *errors.Error", err)
	}
	if appErr.Code == nil || appErr.Code.Str() != codes.CfgInvalid.Str() {
		t.Fatalf("code = %v, want %v", appErr.Code, codes.CfgInvalid)
	}
}

func TestCheckSchemaTagsJoinsEveryMalformedTag(t *testing.T) {
	docs := New(API{Title: "bad", Version: "1"})
	docs.Register("POST", "/items", Body(badConstraintFixture{}))

	err := CheckSchemaTags(docs)

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("error %T does not join several errors", err)
	}
	if got := len(joined.Unwrap()); got != 14 {
		t.Fatalf("reported %d errors, want 14: %v", got, err)
	}
}

func TestRoutesDoNotShareConstraintValues(t *testing.T) {
	minimum := 1.0
	schema := &Schema{Type: "integer", Minimum: &minimum, Default: map[string]interface{}{"a": 1.0}}
	docs := New(API{Title: "clone", Version: "1"})
	docs.Register("POST", "/a", BodySchema(schema))
	docs.Register("POST", "/b", BodySchema(schema))

	first := docs.Routes()[0].BodySchema
	*first.Minimum = 5
	first.Default.(map[string]interface{})["a"] = 9.0

	second := docs.Routes()[1].BodySchema
	if *second.Minimum != 1 || second.Default.(map[string]interface{})["a"] != 1.0 {
		t.Fatalf("second route changed: minimum %v, default %v", *second.Minimum, second.Default)
	}
	if *schema.Minimum != 1 {
		t.Fatalf("caller schema changed: minimum %v", *schema.Minimum)
	}
}
