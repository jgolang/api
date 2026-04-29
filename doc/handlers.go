package doc

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
)

// OpenAPIHandler returns a handler that serves the docs as OpenAPI JSON.
func OpenAPIHandler(docs *Docs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(GenerateOpenAPI(docs)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// SwaggerUIHandler returns a handler that serves Swagger UI for an OpenAPI URL.
func SwaggerUIHandler(openAPIURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, swaggerUIHTML, html.EscapeString(openAPIURL))
	}
}

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "%s",
        dom_id: "#swagger-ui"
      });
    };
  </script>
</body>
</html>`
