package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

// newFileRoutes returns docs for a multipart upload and a binary download,
// so the contracttest module validates media types other than JSON.
func newFileRoutes() *doc.Docs {
	docs := doc.New(doc.API{Title: "Files API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	binary := &doc.Schema{Type: "string", Format: "binary"}
	noop := func(http.ResponseWriter, *http.Request) {}

	api.Post(router, "/files", noop,
		doc.OperationID("uploadFile"),
		doc.BodySchema(&doc.Schema{
			Type:       "object",
			Properties: map[string]*doc.Schema{"file": binary},
			Required:   []string{"file"},
		}),
		func(route *doc.Route) { route.BodyContentType = "multipart/form-data" },
		doc.Status(http.StatusCreated, nil),
	)
	api.Get(router, "/files/{id}", noop,
		doc.OperationID("downloadFile"),
		doc.Path("id", doc.String, true),
		func(route *doc.Route) {
			route.Responses = append(route.Responses, doc.RouteResponse{
				Status:      http.StatusOK,
				ContentType: "application/octet-stream",
				Schema:      binary,
			})
		},
	)
	return docs
}

func TestFileRoutesSpecMatchesGolden(t *testing.T) {
	docs := newFileRoutes()

	got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	assertMatchesGolden(t, "testdata/openapi/file-routes.json", got)
}
