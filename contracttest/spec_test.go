package contracttest

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// specFiles is where the root module's tests write every generated spec.
// Each one is validated, so a story that adds a spec there is covered
// without changing this file.
const specFiles = "../testdata/openapi/*.json"

// validateSpec validates spec as OpenAPI with its examples checked against
// their schemas.
func validateSpec(spec *openapi3.T) error {
	return spec.Validate(context.Background(), openapi3.EnableExamplesValidation())
}

func loadSpec(t *testing.T, path string) *openapi3.T {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	return spec
}

func TestSpecsAreValid(t *testing.T) {
	paths, err := filepath.Glob(specFiles)
	if err != nil {
		t.Fatalf("list specs: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no spec matches %s", specFiles)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			spec := loadSpec(t, path)

			if err := validateSpec(spec); err != nil {
				t.Fatalf("%s is not valid OpenAPI with examples validated: %v", path, err)
			}
		})
	}
}

// TestSpecValidationRejectsWrongExample proves examples are validated: a
// 201 example whose id is no longer an integer must fail.
func TestSpecValidationRejectsWrongExample(t *testing.T) {
	spec := loadSpec(t, "../testdata/openapi/complete-route.envelope.json")
	created := spec.Paths.Find("/projects/{projectID}/tasks").Post.Responses.Status(http.StatusCreated).Value
	media := created.Content.Get("application/json")
	example, ok := media.Example.(map[string]any)
	if !ok {
		t.Fatalf("201 example is %T, want an object", media.Example)
	}
	content, ok := example["content"].(map[string]any)
	if !ok {
		t.Fatalf("201 example content is %T, want an object", example["content"])
	}
	content["id"] = "not a number"

	err := validateSpec(spec)

	if err == nil || !strings.Contains(err.Error(), "invalid example") {
		t.Fatalf("validation error = %v, want it to reject the edited 201 example", err)
	}
}
