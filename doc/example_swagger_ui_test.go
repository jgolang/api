package doc_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/jgolang/api/doc"
)

// printTitleLine prints the <title> line of the page served by handler.
func printTitleLine(handler http.HandlerFunc) {
	res := httptest.NewRecorder()
	handler(res, httptest.NewRequest(http.MethodGet, "/docs", nil))
	for _, line := range strings.Split(res.Body.String(), "\n") {
		if strings.Contains(line, "<title>") {
			fmt.Println(strings.TrimSpace(line))
		}
	}
}

func ExampleSwaggerUIHandler() {
	handler := doc.SwaggerUIHandler("/openapi.json",
		doc.WithTitle("Orders API"),
		doc.WithFavicon("/static/orders-icon.png"),
	)

	printTitleLine(handler)
	// Output:
	// <title>Orders API</title>
}
