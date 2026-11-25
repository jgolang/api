package doc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func requestMedia(t *testing.T, docs *Docs) MediaType {
	t.Helper()
	body := GenerateOpenAPI(docs).Paths["/tasks/{id}/complete"]["post"].RequestBody
	if body == nil {
		t.Fatal("expected a request body")
	}
	media, ok := body.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content")
	}
	return media
}

func TestResponseExamplesWritesExampleObjectsAsIs(t *testing.T) {
	tests := []struct {
		name    string
		example interface{}
		want    ExampleObject
	}{
		{
			name:    "value",
			example: ExampleObject{Summary: "S", Value: conflictBody{Code: "A"}},
			want:    ExampleObject{Summary: "S", Value: conflictBody{Code: "A"}},
		},
		{
			name:    "pointer",
			example: &ExampleObject{Summary: "S", Value: conflictBody{Code: "A"}},
			want:    ExampleObject{Summary: "S", Value: conflictBody{Code: "A"}},
		},
		{
			name:    "plain value",
			example: conflictBody{Code: "A"},
			want:    ExampleObject{Value: conflictBody{Code: "A"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			media := conflictMedia(t, newConflictDocs(ResponseExamples(409, "", conflictBody{}, map[string]interface{}{"a": test.example})))

			if got := media.Examples["a"]; !reflect.DeepEqual(got, test.want) {
				t.Fatalf("examples.a = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestResponseExamplesNilExampleObjectPointerWritesNullValue(t *testing.T) {
	media := conflictMedia(t, newConflictDocs(ResponseExamples(409, "", conflictBody{}, map[string]interface{}{"a": (*ExampleObject)(nil)})))

	raw, err := json.Marshal(media.Examples)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"a":{"value":null}}` {
		t.Fatalf("examples = %s, want a null value", raw)
	}
}

func TestBodyExamplesWritesNamedExamples(t *testing.T) {
	media := requestMedia(t, newConflictDocs(BodyExamples(conflictBody{}, map[string]interface{}{
		"x": ExampleObject{Summary: "X", Value: conflictBody{Code: "X"}},
		"y": conflictBody{Code: "Y"},
	})))

	want := map[string]ExampleObject{
		"x": {Summary: "X", Value: conflictBody{Code: "X"}},
		"y": {Value: conflictBody{Code: "Y"}},
	}
	if media.Schema == nil || media.Example != nil || !reflect.DeepEqual(media.Examples, want) {
		t.Fatalf("expected a schema, two named examples and no single example, got %+v", media)
	}
}

func TestBodyExamplesReplaceBodyWithExample(t *testing.T) {
	media := requestMedia(t, newConflictDocs(
		BodyWithExample(conflictBody{}, conflictBody{Code: "single"}),
		BodyExamples(conflictBody{}, map[string]interface{}{"x": conflictBody{Code: "X"}}),
	))

	if media.Example != nil || len(media.Examples) != 1 {
		t.Fatalf("expected only the named examples, got %+v", media)
	}
}

func TestBodyExamplesWithEmptyMapWritesOnlySchema(t *testing.T) {
	media := requestMedia(t, newConflictDocs(BodyExamples(conflictBody{}, nil)))

	raw, err := json.Marshal(media)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"schema":{"$ref":"#/components/schemas/ConflictBody"}}` {
		t.Fatalf("media = %s, want the schema only", raw)
	}
}

func TestBodyExamplesAreClonedOnRegisterAndRoutes(t *testing.T) {
	value := map[string]interface{}{"code": "X"}
	input := map[string]interface{}{"x": ExampleObject{Summary: "X", Value: value}}
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("POST", "/tasks/{id}/complete", BodyExamples(conflictBody{}, input))

	input["y"] = "added"
	value["code"] = "changed"
	routes := docs.Routes()
	routes[0].BodyExamples["z"] = "added"

	want := map[string]interface{}{"x": ExampleObject{Summary: "X", Value: map[string]interface{}{"code": "X"}}}
	if got := docs.Routes()[0].BodyExamples; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored body examples = %v, want %v", got, want)
	}
}

func TestResponseExamplesDeepCloneExampleObjectPointerValue(t *testing.T) {
	value := map[string]interface{}{"code": "A"}
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("POST", "/tasks/{id}/complete", ResponseExamples(409, "", conflictBody{},
		map[string]interface{}{"a": &ExampleObject{Summary: "A", Value: value}}))

	value["code"] = "changed"

	got, ok := docs.Routes()[0].Responses[0].Examples["a"].(*ExampleObject)
	if !ok || !reflect.DeepEqual(got.Value, map[string]interface{}{"code": "A"}) {
		t.Fatalf("stored example = %#v, want the value at registration", docs.Routes()[0].Responses[0].Examples["a"])
	}
}
