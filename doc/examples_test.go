package doc

import (
	"reflect"
	"testing"
)

type conflictBody struct {
	Code string `json:"code"`
}

func conflictMedia(t *testing.T, opts ...RouteOption) (ResponseObject, MediaType) {
	t.Helper()
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("POST", "/tasks/{id}/complete", opts...)

	response, ok := GenerateOpenAPI(docs).Paths["/tasks/{id}/complete"]["post"].Responses["409"]
	if !ok {
		t.Fatal("expected a 409 response")
	}
	media, ok := response.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content")
	}
	return response, media
}

func TestResponseExamplesWritesNamedExamples(t *testing.T) {
	examples := map[string]interface{}{
		"transition": conflictBody{Code: "PLN-016"},
		"system":     conflictBody{Code: "PLN-024"},
	}

	response, media := conflictMedia(t, ResponseExamples(409, "Transition not allowed or system record", conflictBody{}, examples))

	if response.Description != "Transition not allowed or system record" {
		t.Fatalf("unexpected description %q", response.Description)
	}
	if media.Example != nil || len(media.Examples) != 2 {
		t.Fatalf("expected two named examples and no single example, got %+v", media)
	}
	if !reflect.DeepEqual(media.Examples["system"].Value, conflictBody{Code: "PLN-024"}) {
		t.Fatalf("unexpected system example %+v", media.Examples["system"])
	}
}

func TestRepeatedStatusOptionsMergeIntoNamedExamples(t *testing.T) {
	response, media := conflictMedia(t,
		ResponseWithDescriptionAndExample(409, "Transition not allowed", conflictBody{}, conflictBody{Code: "PLN-016"}),
		ResponseWithDescriptionAndExample(409, "System record", conflictBody{}, conflictBody{Code: "PLN-024"}),
	)

	if response.Description != "Transition not allowed or System record" {
		t.Fatalf("unexpected merged description %q", response.Description)
	}
	want := map[string]ExampleObject{
		"Transition not allowed": {Summary: "Transition not allowed", Value: conflictBody{Code: "PLN-016"}},
		"System record":          {Summary: "System record", Value: conflictBody{Code: "PLN-024"}},
	}
	if media.Example != nil || !reflect.DeepEqual(media.Examples, want) {
		t.Fatalf("expected merged named examples, got %+v", media)
	}
}

func TestSingleExampleStaysAnExample(t *testing.T) {
	_, media := conflictMedia(t, StatusWithExample(409, conflictBody{}, conflictBody{Code: "PLN-016"}))

	if media.Examples != nil || !reflect.DeepEqual(media.Example, conflictBody{Code: "PLN-016"}) {
		t.Fatalf("expected a single example, got %+v", media)
	}
}
