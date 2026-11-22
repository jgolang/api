package doc_test

import (
	"fmt"
	"net/http"

	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

// Example_mediaTypes documents a multipart upload and a binary download. A
// route sets Route.BodyContentType or RouteResponse.ContentType with a
// RouteOption closure; left empty, both stay application/json.
func Example_mediaTypes() {
	docs := doc.New(doc.API{Title: "Files API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	binary := &doc.Schema{Type: "string", Format: "binary"}

	router.Handle("POST", "/files", func(http.ResponseWriter, *http.Request) {},
		doc.BodySchema(&doc.Schema{
			Type:       "object",
			Properties: map[string]*doc.Schema{"file": binary},
			Required:   []string{"file"},
		}),
		func(route *doc.Route) { route.BodyContentType = "multipart/form-data" },
		doc.Status(http.StatusCreated, nil),
	)
	router.Handle("GET", "/files/{id}", func(http.ResponseWriter, *http.Request) {},
		doc.Path("id", doc.String, true),
		func(route *doc.Route) {
			route.Responses = append(route.Responses, doc.RouteResponse{
				Status:      http.StatusOK,
				ContentType: "application/octet-stream",
				Schema:      binary,
			})
		},
	)

	spec := doc.GenerateOpenAPI(docs)
	for key := range spec.Paths["/files"]["post"].RequestBody.Content {
		fmt.Println("upload:", key)
	}
	for key := range spec.Paths["/files/{id}"]["get"].Responses["200"].Content {
		fmt.Println("download:", key)
	}
	// Output:
	// upload: multipart/form-data
	// download: application/octet-stream
}
