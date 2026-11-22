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

func componentBody(t *testing.T, body any) (*doc.Schema, doc.OpenAPI) {
	t.Helper()
	spec := generateResponseSpec(body)
	ref := spec.Paths["/tasks"]["get"].Responses["200"].Content["application/json"].Schema.Ref
	schema := spec.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
	if schema == nil {
		t.Fatalf("expected a component schema for %q", ref)
	}
	return schema, spec
}

func TestProblemRequiresOnlyType(t *testing.T) {
	body, _ := componentBody(t, envelope.Problem())

	if len(body.Required) != 1 || body.Required[0] != "type" {
		t.Fatalf("expected only type required, got %v", body.Required)
	}
}

func TestProblemExampleIsNotFound(t *testing.T) {
	body, _ := componentBody(t, envelope.Problem())

	if body.Properties["status"].Example != float64(404) || body.Properties["title"].Example != "Not Found" {
		t.Fatalf("expected a 404 Not Found example, got status %v title %v",
			body.Properties["status"].Example, body.Properties["title"].Example)
	}
}

func TestPlainDocTypesRequireEveryWireKey(t *testing.T) {
	wireKeys := []string{"title", "message", "type", "code", "token", "action", "event_id", "info", "content"}
	tests := []struct {
		name string
		body any
	}{
		{name: "success", body: envelope.PlainSuccess[docTask]()},
		{name: "error", body: envelope.PlainError()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := componentBody(t, tt.body)

			if len(body.Required) != len(wireKeys) {
				t.Fatalf("expected %d required keys, got %v", len(wireKeys), body.Required)
			}
			for _, key := range wireKeys {
				if !containsString(body.Required, key) {
					t.Fatalf("expected %s required, got %v", key, body.Required)
				}
			}
		})
	}
}

func TestPlainDocTypesInfoAndContentAreNullable(t *testing.T) {
	tests := []struct {
		name string
		body any
	}{
		{name: "success", body: envelope.PlainSuccess[docTask]()},
		{name: "error", body: envelope.PlainError()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := componentBody(t, tt.body)

			info := body.Properties["info"]
			if !info.Nullable || info.Type != "object" || info.AdditionalProperties == nil || info.AdditionalProperties.Type != "string" {
				t.Fatalf("expected info as a nullable object of strings, got %#v", info)
			}
			if !body.Properties["content"].Nullable {
				t.Fatalf("expected content nullable, got %#v", body.Properties["content"])
			}
		})
	}
}

func TestPlainSuccessContentReferencesPayload(t *testing.T) {
	body, spec := componentBody(t, envelope.PlainSuccess[docTask]())

	content := body.Properties["content"]
	if len(content.AllOf) != 1 {
		t.Fatalf("expected content as allOf [$ref], got %#v", content)
	}
	ref := content.AllOf[0].Ref
	if spec.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")] == nil {
		t.Fatalf("expected content to reference a component, got %q", ref)
	}
}

func TestPlainSuccessCodeExampleIsWireSuccessCode(t *testing.T) {
	body, _ := componentBody(t, envelope.PlainSuccess[docTask]())

	if code := body.Properties["code"]; code.Example != "0000" {
		t.Fatalf("expected code example 0000, got %#v", code)
	}
}
