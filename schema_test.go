package api

import (
	"testing"
	"time"
)

type schemaAddress struct {
	City string `json:"city"`
}

type schemaExample struct {
	ID        int               `json:"id"`
	Name      string            `json:"name,omitempty"`
	Email     string            `json:"email,omitempty" format:"email" example:"user@example.com" validate:"required"`
	Count     int               `json:"count,omitempty" example:"3"`
	CreatedAt time.Time         `json:"created_at"`
	Address   *schemaAddress    `json:"address,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Ignored   string            `json:"-"`
}

func TestSchemaFromTypeSupportsCommonGoShapes(t *testing.T) {
	schema := SchemaFromType(schemaExample{})

	if schema.Type != "object" {
		t.Fatalf("expected object schema, got %s", schema.Type)
	}
	if schema.Properties["id"].Type != "integer" {
		t.Fatalf("expected integer id schema: %#v", schema.Properties["id"])
	}
	if schema.Properties["created_at"].Format != "date-time" {
		t.Fatalf("expected time.Time date-time schema: %#v", schema.Properties["created_at"])
	}
	if schema.Properties["email"].Format != "email" || schema.Properties["email"].Example != "user@example.com" {
		t.Fatalf("expected format and example tags in email schema: %#v", schema.Properties["email"])
	}
	if schema.Properties["count"].Example != float64(3) {
		t.Fatalf("expected numeric example tag in count schema: %#v", schema.Properties["count"])
	}
	if schema.Properties["address"].Nullable != true {
		t.Fatalf("expected pointer schema to be nullable: %#v", schema.Properties["address"])
	}
	if schema.Properties["address"].Properties["city"].Type != "string" {
		t.Fatalf("expected nested struct schema: %#v", schema.Properties["address"])
	}
	if schema.Properties["tags"].Items.Type != "string" {
		t.Fatalf("expected slice item schema: %#v", schema.Properties["tags"])
	}
	if schema.Properties["metadata"].AdditionalProperties.Type != "string" {
		t.Fatalf("expected map value schema: %#v", schema.Properties["metadata"])
	}
	if _, ok := schema.Properties["Ignored"]; ok {
		t.Fatalf("json:- field should not be included")
	}
	if !containsString(schema.Required, "email") {
		t.Fatalf("expected validate required field in required list, got %#v", schema.Required)
	}
	if len(schema.Required) != 3 {
		t.Fatalf("expected non-omitempty fields plus validate required, got %#v", schema.Required)
	}
}

func TestSchemaFromTypeAllowsManualSchema(t *testing.T) {
	manual := &Schema{Type: "string", Format: "uuid"}
	if got := SchemaFromType(manual); got != manual {
		t.Fatalf("manual schema should be returned as-is")
	}
}

func TestSchemaFromTypeSupportsTypedJSONResponseContent(t *testing.T) {
	schema := SchemaFromType(JSONResponseOf[taskResponse]{})

	if schema.Properties["header"].Properties["type"].Type != "string" {
		t.Fatalf("expected header schema, got %#v", schema.Properties["header"])
	}
	content := schema.Properties["content"]
	if content.Type != "object" {
		t.Fatalf("expected content object schema, got %#v", content)
	}
	if content.Properties["id"].Type != "integer" {
		t.Fatalf("expected content id schema, got %#v", content.Properties["id"])
	}
	if content.Properties["title"].Type != "string" {
		t.Fatalf("expected content title schema, got %#v", content.Properties["title"])
	}
}

func TestSchemaFromTypeSupportsTypedJSONResponseSliceContent(t *testing.T) {
	schema := SchemaFromType(JSONResponseOf[[]taskResponse]{})

	content := schema.Properties["content"]
	if content.Type != "array" {
		t.Fatalf("expected content array schema, got %#v", content)
	}
	if content.Items.Properties["id"].Type != "integer" {
		t.Fatalf("expected array item DTO schema, got %#v", content.Items)
	}
}

func TestSchemaFromTypeSupportsJSONErrorResponse(t *testing.T) {
	schema := SchemaFromType(JSONErrorResponse{})

	if _, ok := schema.Properties["header"]; !ok {
		t.Fatalf("expected header property in error response schema: %#v", schema)
	}
	if _, ok := schema.Properties["content"]; ok {
		t.Fatalf("error response schema should not include content: %#v", schema)
	}
}

func TestSchemaFromTypeSupportsTypedJSONRequestContent(t *testing.T) {
	schema := SchemaFromType(JSONRequestOf[createTaskRequest]{})

	if schema.Properties["header"].Properties["uuid"].Type != "string" {
		t.Fatalf("expected request header schema, got %#v", schema.Properties["header"])
	}
	content := schema.Properties["content"]
	if content.Type != "object" {
		t.Fatalf("expected request content object schema, got %#v", content)
	}
	if content.Properties["title"].Type != "string" {
		t.Fatalf("expected request content title schema, got %#v", content.Properties["title"])
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
