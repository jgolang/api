package doc_test

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

func ExampleHidden() {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)

	router.Handle("GET", "/tasks", func(http.ResponseWriter, *http.Request) {}, doc.Summary("List tasks"))
	router.Handle("GET", "/openapi.json", doc.OpenAPIHandler(docs), doc.Hidden())
	router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json"), doc.Hidden())

	var paths []string
	for path := range doc.GenerateOpenAPI(docs).Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	fmt.Println(paths)
	// Output: [/tasks]
}
