package doc

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

// schemaNode refers to itself, so every walk must stop at the second visit.
type schemaNode struct {
	Value string      `json:"value"`
	Next  *schemaNode `json:"next,omitempty"`
}

func TestSchemaFromTypeMatchesGolden(t *testing.T) {
	schemas := map[string]*Schema{
		"common_shapes":     SchemaFromType(schemaExample{}),
		"pointer_body":      SchemaFromType(&schemaExample{}),
		"response_wrapper":  SchemaFromType(testResponseOf[taskResponse]{}),
		"response_of_slice": SchemaFromType(testResponseOf[[]taskResponse]{}),
		"request_wrapper":   SchemaFromType(testRequestOf[createTaskRequest]{}),
		"error_response":    SchemaFromType(testErrorResponse{}),
		"recursive":         SchemaFromType(schemaNode{}),
		"ref_fixture":       SchemaFromType(refTaskRequest{}),
	}

	assertMatchesGolden(t, "schema-from-type.golden.json", schemas)
}

func TestGenerateOpenAPIFixturesMatchGolden(t *testing.T) {
	docs := New(API{Title: "Fixtures", Version: "1.0.0"})
	docs.Register(http.MethodPost, "/examples", Body(schemaExample{}), Status(http.StatusOK, &schemaExample{}))
	docs.Register(http.MethodGet, "/tasks", Status(http.StatusOK, testResponseOf[[]taskResponse]{}))
	docs.Register(http.MethodPost, "/tasks",
		Body(testRequestOf[createTaskRequest]{}),
		Status(http.StatusCreated, testResponseOf[taskResponse]{}),
		Status(http.StatusBadRequest, testErrorResponse{}),
	)
	docs.Register(http.MethodPost, "/ref-tasks", Body(&refTaskRequest{}), Status(http.StatusCreated, &refTaskRequest{}))
	docs.Register(http.MethodPost, "/nodes", Body(schemaNode{}), Status(http.StatusOK, &schemaNode{}))

	spec := GenerateOpenAPI(docs)

	assertMatchesGolden(t, "openapi-fixtures.golden.json", spec)
}

func assertMatchesGolden(t *testing.T, name string, value interface{}) {
	t.Helper()
	got, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from golden; diff the output below with testdata; use -update only for an intended change\ngot:\n%s", name, got)
	}
}

// componentNode reaches itself through an anonymous struct, which the
// component placeholder, not a recursion guard, must stop.
type componentNode struct {
	X struct {
		N *componentNode `json:"n,omitempty"`
	} `json:"x"`
}

func TestGenerateOpenAPIExpandsAnonymousStructInsideComponent(t *testing.T) {
	docs := New(API{Title: "Nodes", Version: "1.0.0"})
	docs.Register(http.MethodPost, "/nodes", Body(struct {
		N *componentNode `json:"n,omitempty"`
	}{}))

	node := GenerateOpenAPI(docs).Components.Schemas["ComponentNode"]

	if node == nil || node.Properties["x"] == nil || node.Properties["x"].Properties["n"] == nil {
		t.Fatalf("expected ComponentNode.x expanded with its n property, got %#v", node)
	}
}
