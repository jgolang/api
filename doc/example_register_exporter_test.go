package doc_test

import (
	"fmt"
	"net/http"

	"github.com/jgolang/api/doc"
)

// ordersExporter serves Swagger UI with the service's own title and icon.
type ordersExporter struct{}

func (ordersExporter) Handler(openAPIURL string) http.HandlerFunc {
	return doc.SwaggerUIHandler(openAPIURL,
		doc.WithTitle("Orders API"),
		doc.WithFavicon("/static/orders-icon.png"),
	)
}

func ExampleRegisterExporter() {
	// A distinct name keeps the default "swagger" exporter untouched.
	doc.RegisterExporter("orders-swagger", ordersExporter{})

	exporter, err := doc.ExporterByName("orders-swagger")
	if err != nil {
		fmt.Println(err)
		return
	}
	printTitleLine(exporter.Handler("/openapi.json"))
	// Output:
	// <title>Orders API</title>
}
