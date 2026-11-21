package envelope_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
)

type docTask struct {
	ID int `json:"id"`
}

func generateResponseSpec(body any) doc.OpenAPI {
	docs := doc.New(doc.API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks", doc.Status(http.StatusOK, body))
	return doc.GenerateOpenAPI(docs)
}

func TestDocTypesRequireHeader(t *testing.T) {
	tests := []struct {
		name string
		body any
	}{
		{name: "success", body: envelope.Success[docTask]()},
		{name: "error", body: envelope.Error()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := generateResponseSpec(tt.body)

			ref := spec.Paths["/tasks"]["get"].Responses["200"].Content["application/json"].Schema.Ref
			body := spec.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
			if body == nil {
				t.Fatalf("expected a component schema for %q", ref)
			}
			if !containsString(body.Required, "header") {
				t.Fatalf("expected header required, got %v", body.Required)
			}
		})
	}
}

func TestDocTypesCodeExampleIsWireSuccessCode(t *testing.T) {
	spec := generateResponseSpec(envelope.Success[docTask]())

	code := spec.Components.Schemas["ResponseInfo"].Properties["code"]

	if code.Example != "0000" || code.Description == "" {
		t.Fatalf("expected code example 0000 with a description, got %#v", code)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
