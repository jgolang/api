package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

// newComponentResponsesRoutes returns docs where two routes reference one
// component response built by an instance, so the contracttest module
// validates component responses and their $refs.
func newComponentResponsesRoutes() *doc.Docs {
	docs := doc.New(doc.API{Title: "Component responses API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	noop := func(http.ResponseWriter, *http.Request) {}
	instance := api.New()

	docs.AddResponse("TaskNotFound", instance.DocError(http.StatusNotFound, "Task not found"))
	api.Get(router, "/tasks/{id}", noop,
		doc.OperationID("getTask"),
		doc.Path("id", doc.Int, true),
		doc.Status(http.StatusOK, nil),
		doc.ResponseRef(http.StatusNotFound, "TaskNotFound"),
	)
	api.Delete(router, "/tasks/{id}", noop,
		doc.OperationID("deleteTask"),
		doc.Path("id", doc.Int, true),
		doc.Status(http.StatusNoContent, nil),
		doc.ResponseRef(http.StatusNotFound, "TaskNotFound"),
	)
	return docs
}

func TestComponentResponsesSpecMatchesGolden(t *testing.T) {
	docs := newComponentResponsesRoutes()

	got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	assertMatchesGolden(t, "testdata/openapi/component-responses.json", got)
}
