package doc

import (
	"encoding/json"
	"net/http"
	"testing"
)

const refAssigneeRef = "#/components/schemas/RefAssignee"

type refAssignee struct {
	ID int `json:"id"`
}

type refBase struct {
	Kind string `json:"kind"`
}

type refTaskRequest struct {
	*refBase
	Owner    *refAssignee   `json:"owner,omitempty" description:"Optional owner"`
	Reviewer refAssignee    `json:"reviewer" description:"Reviewer"`
	Sample   refAssignee    `json:"sample" example:"{\"id\":7}"`
	Creator  refAssignee    `json:"creator"`
	Watchers []*refAssignee `json:"watchers,omitempty"`
}

func generateRefTaskSpec() OpenAPI {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodPost, "/tasks", Body(&refTaskRequest{}), Status(http.StatusCreated, &refTaskRequest{}))
	return GenerateOpenAPI(docs)
}

func TestGenerateOpenAPIWritesRefSiblingsAsAllOf(t *testing.T) {
	tests := []struct {
		name        string
		property    string
		nullable    bool
		description string
		example     interface{}
	}{
		{name: "pointer field with description", property: "owner", nullable: true, description: "Optional owner"},
		{name: "value field with description", property: "reviewer", description: "Reviewer"},
		{name: "value field with example", property: "sample", example: map[string]interface{}{"id": float64(7)}},
	}
	properties := generateRefTaskSpec().Components.Schemas["RefTaskRequest"].Properties
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := properties[test.property]

			if schema.Ref != "" || len(schema.AllOf) != 1 || schema.AllOf[0].Ref != refAssigneeRef {
				t.Fatalf("expected allOf [$ref], got %#v", schema)
			}
			if schema.Nullable != test.nullable || schema.Description != test.description {
				t.Fatalf("expected nullable %v and description %q, got %#v", test.nullable, test.description, schema)
			}
			if test.example != nil && schema.Example == nil {
				t.Fatalf("expected example beside allOf, got %#v", schema)
			}
		})
	}
}

func TestGenerateOpenAPIKeepsPlainRefForUntaggedStructField(t *testing.T) {
	properties := generateRefTaskSpec().Components.Schemas["RefTaskRequest"].Properties

	creator := properties["creator"]

	if creator.Ref != refAssigneeRef || creator.AllOf != nil {
		t.Fatalf("expected plain $ref, got %#v", creator)
	}
}

func TestGenerateOpenAPIWritesPointerSliceItemsAsNullableAllOf(t *testing.T) {
	properties := generateRefTaskSpec().Components.Schemas["RefTaskRequest"].Properties

	items := properties["watchers"].Items

	if len(items.AllOf) != 1 || items.AllOf[0].Ref != refAssigneeRef || !items.Nullable {
		t.Fatalf("expected nullable allOf [$ref] items, got %#v", items)
	}
}

func TestGenerateOpenAPIMergesEmbeddedPointerStruct(t *testing.T) {
	properties := generateRefTaskSpec().Components.Schemas["RefTaskRequest"].Properties

	kind := properties["kind"]

	if kind == nil || kind.Type != "string" {
		t.Fatalf("expected embedded pointer struct properties to be merged, got %#v", properties)
	}
}

func TestGenerateOpenAPIWritesPointerBodyAsPlainRef(t *testing.T) {
	operation := generateRefTaskSpec().Paths["/tasks"]["post"]

	body := operation.RequestBody.Content["application/json"].Schema
	response := operation.Responses["201"].Content["application/json"].Schema

	for _, schema := range []*Schema{body, response} {
		if schema.Ref != "#/components/schemas/RefTaskRequest" || schema.AllOf != nil || schema.Nullable {
			t.Fatalf("expected a pointer body sample as a plain $ref, got %#v", schema)
		}
	}
}

func TestGenerateOpenAPIWritesNoKeyBesideRef(t *testing.T) {
	encoded, err := json.Marshal(generateRefTaskSpec())
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	var decoded interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}

	// OpenAPI 3.0.3 ignores every key written beside $ref.
	var walk func(node interface{}, path string)
	walk = func(node interface{}, path string) {
		switch typed := node.(type) {
		case map[string]interface{}:
			if _, ok := typed["$ref"]; ok && len(typed) > 1 {
				t.Errorf("schema at %s has keys beside $ref: %v", path, typed)
			}
			for key, child := range typed {
				walk(child, path+"/"+key)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child, path+"[]")
			}
		}
	}
	walk(decoded, "#")
}
