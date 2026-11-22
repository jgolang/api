package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

type namedExamplesTask struct {
	Title    string `json:"title"`
	Priority string `json:"priority,omitempty"`
}

type namedExamplesConflict struct {
	Code string `json:"code"`
}

// newNamedExamplesRoutes returns docs for a request body with two named,
// summarised examples and a response with a summarised example, so the
// contracttest module validates named examples against their schemas.
func newNamedExamplesRoutes() *doc.Docs {
	docs := doc.New(doc.API{Title: "Named examples API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	noop := func(http.ResponseWriter, *http.Request) {}

	api.Post(router, "/tasks", noop,
		doc.OperationID("createTask"),
		doc.BodyExamples(namedExamplesTask{}, map[string]interface{}{
			"minimal": doc.ExampleObject{
				Summary: "Only the required title",
				Value:   namedExamplesTask{Title: "Write docs"},
			},
			"urgent": doc.ExampleObject{
				Summary: "A high-priority task",
				Value:   namedExamplesTask{Title: "Fix outage", Priority: "high"},
			},
		}),
		doc.Status(http.StatusCreated, nil),
		doc.ResponseExamples(http.StatusConflict, "Task already exists", namedExamplesConflict{}, map[string]interface{}{
			"duplicate": doc.ExampleObject{
				Summary: "A task with the same title exists",
				Value:   namedExamplesConflict{Code: "TSK-001"},
			},
		}),
	)
	return docs
}

func TestNamedExamplesSpecMatchesGolden(t *testing.T) {
	docs := newNamedExamplesRoutes()

	got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	assertMatchesGolden(t, "testdata/openapi/named-examples.json", got)
}
