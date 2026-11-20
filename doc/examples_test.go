package doc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type conflictBody struct {
	Code string `json:"code"`
}

func conflictMedia(t *testing.T, docs *Docs) MediaType {
	t.Helper()
	response, ok := GenerateOpenAPI(docs).Paths["/tasks/{id}/complete"]["post"].Responses["409"]
	if !ok {
		t.Fatal("expected a 409 response")
	}
	media, ok := response.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content")
	}
	return media
}

func newConflictDocs(opts ...RouteOption) *Docs {
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("POST", "/tasks/{id}/complete", opts...)
	return docs
}

func TestResponseExamplesWritesNamedExamples(t *testing.T) {
	media := conflictMedia(t, newConflictDocs(ResponseExamples(409, "Transition not allowed or system record", conflictBody{}, map[string]interface{}{
		"transition": conflictBody{Code: "PLN-016"},
		"system":     conflictBody{Code: "PLN-024"},
	})))

	want := map[string]ExampleObject{
		"transition": {Value: conflictBody{Code: "PLN-016"}},
		"system":     {Value: conflictBody{Code: "PLN-024"}},
	}
	if media.Example != nil || !reflect.DeepEqual(media.Examples, want) {
		t.Fatalf("expected two named examples and no single example, got %+v", media)
	}
}

func TestResponseExamplesJSONHasNoSingleExample(t *testing.T) {
	media := conflictMedia(t, newConflictDocs(ResponseExamples(409, "", conflictBody{}, map[string]interface{}{"a": conflictBody{}})))

	raw, err := json.Marshal(media)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"example":`) || !strings.Contains(string(raw), `"examples":`) {
		t.Fatalf("expected only examples, got %s", raw)
	}
}

func TestLaterOptionForTheSameStatusStillReplaces(t *testing.T) {
	media := conflictMedia(t, newConflictDocs(
		ResponseExamples(409, "", conflictBody{}, map[string]interface{}{"a": conflictBody{Code: "A"}}),
		StatusWithExample(409, conflictBody{}, conflictBody{Code: "B"}),
	))

	if media.Examples != nil || !reflect.DeepEqual(media.Example, conflictBody{Code: "B"}) {
		t.Fatalf("expected the later option to replace the earlier one, got %+v", media)
	}
}

func TestResponseExamplesCopiesTheInputMap(t *testing.T) {
	input := map[string]interface{}{"a": conflictBody{Code: "A"}}
	option := ResponseExamples(409, "", conflictBody{}, input)
	input["b"] = conflictBody{Code: "B"}

	docs := New(API{Title: "test", Version: "1"})
	first := docs.Register("POST", "/tasks/{id}/complete", option)
	first.Responses[0].Examples["c"] = conflictBody{Code: "C"}
	docs.Register("POST", "/other", option)

	if got := docs.Routes()[1].Responses[0].Examples; len(got) != 1 {
		t.Fatalf("expected only the original example, got %v", got)
	}
}
