package doc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type componentProblem struct {
	Code string `json:"code"`
}

const notFoundRef = "#/components/responses/NotFound"

func componentResponse(t *testing.T, spec OpenAPI, name string) ResponseObject {
	t.Helper()
	if spec.Components == nil {
		t.Fatal("expected components")
	}
	response, ok := spec.Components.Responses[name]
	if !ok {
		t.Fatalf("expected components.responses.%s, got %+v", name, spec.Components.Responses)
	}
	return response
}

func hasComponentResponses(spec OpenAPI) bool {
	return spec.Components != nil && len(spec.Components.Responses) > 0
}

func TestAddResponseIsReferencedByRoutes(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("NotFound", ResponseSchema(404, "Task not found", &Schema{Type: "object"}))
	docs.Register("GET", "/tasks/{id}", ResponseRef(404, "NotFound"))
	docs.Register("DELETE", "/tasks/{id}", ResponseRef(404, "NotFound"))

	spec := GenerateOpenAPI(docs)
	raw := marshalString(t, spec)

	component := componentResponse(t, spec, "NotFound")
	if component.Description != "Task not found" {
		t.Fatalf("description = %q, want %q", component.Description, "Task not found")
	}
	if got := component.Content["application/json"].Schema; got == nil || got.Type != "object" {
		t.Fatalf("schema = %+v, want object", got)
	}
	if got := strings.Count(raw, `"$ref":"`+notFoundRef+`"`); got != 2 {
		t.Fatalf("$ref uses = %d, want 2 in %s", got, raw)
	}
	if got := strings.Count(raw, "Task not found"); got != 1 {
		t.Fatalf("description written %d times, want 1 in %s", got, raw)
	}
	for _, method := range []string{"get", "delete"} {
		response := spec.Paths["/tasks/{id}"][method].Responses["404"]
		if got := marshalString(t, response); got != `{"$ref":"`+notFoundRef+`"}` {
			t.Fatalf("%s 404 = %s, want only the $ref", method, got)
		}
	}
}

func TestAddResponseWritesBodyTypeToSchemas(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("Problem", Status(400, componentProblem{}))

	spec := GenerateOpenAPI(docs)

	schema := componentResponse(t, spec, "Problem").Content["application/json"].Schema
	if schema == nil || schema.Ref != "#/components/schemas/ComponentProblem" {
		t.Fatalf("schema = %+v, want $ref to ComponentProblem", schema)
	}
	if _, ok := spec.Components.Schemas["ComponentProblem"]; !ok {
		t.Fatalf("expected components.schemas.ComponentProblem, got %+v", spec.Components.Schemas)
	}
}

func TestAddResponseWritesNamedExamples(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("Conflict", ResponseExamples(409, "Conflict", componentProblem{}, map[string]interface{}{
		"locked": ExampleObject{Summary: "Locked", Value: componentProblem{Code: "TSK-409"}},
	}))

	media := componentResponse(t, GenerateOpenAPI(docs), "Conflict").Content["application/json"]

	want := map[string]ExampleObject{"locked": {Summary: "Locked", Value: componentProblem{Code: "TSK-409"}}}
	if !reflect.DeepEqual(media.Examples, want) {
		t.Fatalf("examples = %+v, want %+v", media.Examples, want)
	}
}

func TestResponseRefUsesTheRouteStatus(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("NotFound", ResponseSchema(404, "Gone", &Schema{Type: "object"}))
	docs.Register("GET", "/tasks/{id}", ResponseRef(410, "NotFound"))

	responses := GenerateOpenAPI(docs).Paths["/tasks/{id}"]["get"].Responses

	if got := responses["410"]; got.Ref != notFoundRef {
		t.Fatalf("410 = %+v, want $ref %s", got, notFoundRef)
	}
	if _, ok := responses["404"]; ok {
		t.Fatal("the component's own status must not be written on the route")
	}
}

func TestAddResponseOptionStatusSetsDefaultDescription(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("Gone", Status(404, componentProblem{}))
	docs.Register("GET", "/tasks/{id}", ResponseRef(410, "Gone"))

	spec := GenerateOpenAPI(docs)

	if got := componentResponse(t, spec, "Gone").Description; got != "Not Found" {
		t.Fatalf("description = %q, want %q", got, "Not Found")
	}
	if got := spec.Paths["/tasks/{id}"]["get"].Responses["410"].Ref; got != "#/components/responses/Gone" {
		t.Fatalf("410 ref = %q, want the Gone component", got)
	}
}

func TestAddResponseSameNameReplaces(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("NotFound", ResponseSchema(404, "first", &Schema{Type: "object"}))
	docs.AddResponse("NotFound", ResponseSchema(404, "second", &Schema{Type: "object"}))

	if got := componentResponse(t, GenerateOpenAPI(docs), "NotFound").Description; got != "second" {
		t.Fatalf("description = %q, want %q", got, "second")
	}
}

func TestAddResponseIgnoresInvalidInput(t *testing.T) {
	tests := []struct {
		name         string
		responseName string
		option       RouteOption
	}{
		{name: "empty name", responseName: "", option: ResponseSchema(404, "x", &Schema{Type: "object"})},
		{name: "nil option", responseName: "NotFound", option: nil},
		{name: "no response", responseName: "NotFound", option: Summary("x")},
		{name: "two responses", responseName: "NotFound", option: func(route *Route) {
			ResponseSchema(404, "a", &Schema{Type: "object"})(route)
			ResponseSchema(409, "b", &Schema{Type: "object"})(route)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docs := newTestDocs()

			docs.AddResponse(test.responseName, test.option)

			if spec := GenerateOpenAPI(docs); hasComponentResponses(spec) {
				t.Fatalf("expected no component responses, got %+v", spec.Components.Responses)
			}
		})
	}
}

func TestResponseRefToMissingComponentIsWrittenAsIs(t *testing.T) {
	docs := newTestDocs()
	docs.Register("GET", "/tasks/{id}", ResponseRef(404, "Missing"))

	spec := GenerateOpenAPI(docs)

	if got := spec.Paths["/tasks/{id}"]["get"].Responses["404"].Ref; got != "#/components/responses/Missing" {
		t.Fatalf("ref = %q, want the dangling ref", got)
	}
	if hasComponentResponses(spec) {
		t.Fatalf("expected no component responses, got %+v", spec.Components.Responses)
	}
}

func TestResponseRefWithEmptyNameAddsNothing(t *testing.T) {
	route := newTestDocs().Register("GET", "/tasks", ResponseRef(404, ""))

	if len(route.Responses) != 0 {
		t.Fatalf("responses = %+v, want none", route.Responses)
	}
}

func TestLaterResponseOptionReplacesResponseRef(t *testing.T) {
	docs := newTestDocs()
	docs.AddResponse("NotFound", ResponseSchema(404, "shared", &Schema{Type: "object"}))
	docs.Register("GET", "/tasks/{id}",
		ResponseRef(404, "NotFound"),
		ResponseSchema(404, "own", &Schema{Type: "object"}),
	)

	got := GenerateOpenAPI(docs).Paths["/tasks/{id}"]["get"].Responses["404"]

	if got.Ref != "" || got.Description != "own" {
		t.Fatalf("404 = %+v, want the later inline response", got)
	}
}

func TestAddResponseOnNilDocsIsNoOp(t *testing.T) {
	var docs *Docs

	docs.AddResponse("NotFound", ResponseSchema(404, "x", &Schema{Type: "object"}))
}

func TestAddResponseStoresACopy(t *testing.T) {
	docs := newTestDocs()
	schema := &Schema{Type: "object"}
	examples := map[string]interface{}{"a": map[string]interface{}{"code": "A"}}
	var added *Route
	docs.AddResponse("NotFound", func(route *Route) {
		route.Responses = append(route.Responses, RouteResponse{Status: 404, Schema: schema, Examples: examples})
		added = route
	})

	schema.Type = "string"
	examples["a"].(map[string]interface{})["code"] = "B"
	added.Responses[0].Description = "changed"

	component := componentResponse(t, GenerateOpenAPI(docs), "NotFound")
	media := component.Content["application/json"]
	if media.Schema.Type != "object" {
		t.Fatalf("schema type = %q, want object", media.Schema.Type)
	}
	if got := media.Examples["a"].Value; !reflect.DeepEqual(got, map[string]interface{}{"code": "A"}) {
		t.Fatalf("example = %+v, want code A", got)
	}
	if component.Description != "Not Found" {
		t.Fatalf("description = %q, want the status text", component.Description)
	}
}

func TestResponseObjectWithoutRefMarshalsAsBefore(t *testing.T) {
	object := ResponseObject{Description: "<ok>", Content: map[string]MediaType{"application/json": {Schema: &Schema{Type: "object"}}}}

	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"description":"\u003cok\u003e","content":{"application/json":{"schema":{"type":"object"}}}}`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}
}
