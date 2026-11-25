package api_test

import (
	"encoding/json"
	"fmt"
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

// newLibraryRejectionsRoutes returns docs where POST /tasks documents the
// library's own rejections and GET /health does not, so the contracttest
// module validates them and the spec shows they stay on their route.
func newLibraryRejectionsRoutes(options ...api.Option) *doc.Docs {
	docs := doc.New(doc.API{Title: "Library rejections API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	noop := func(http.ResponseWriter, *http.Request) {}
	instance := api.New(options...)

	api.Post(router, "/tasks", noop,
		doc.OperationID("createTask"),
		doc.Status(http.StatusCreated, nil),
		instance.DocRejections(),
	)
	api.Get(router, "/health", noop,
		doc.OperationID("health"),
		doc.Status(http.StatusOK, nil),
	)
	return docs
}

func TestLibraryRejectionsSpecMatchesGolden(t *testing.T) {
	for _, variant := range completeRouteVariants {
		t.Run(variant.name, func(t *testing.T) {
			docs := newLibraryRejectionsRoutes(variant.options...)

			got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
			if err != nil {
				t.Fatalf("marshal spec: %v", err)
			}

			assertMatchesGolden(t, fmt.Sprintf("testdata/openapi/library-rejections.%s.json", variant.name), got)
		})
	}
}
