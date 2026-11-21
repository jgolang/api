package doc

import (
	"encoding/json"
	"fmt"
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
// The page loads a pinned swagger-ui-dist release from unpkg, checked with
// Subresource Integrity hashes.
func SwaggerUIHandler(openAPIURL string) http.HandlerFunc {
	// json.Marshal quotes the URL as a JavaScript string literal and escapes
	// <, > and &, so it cannot end the script element. A string always encodes.
	encodedURL, _ := json.Marshal(openAPIURL)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, swaggerUIHTML, encodedURL)
	}
}

// swaggerUIHTML pins swagger-ui-dist 5.34.1. When updating the version,
// recompute each integrity hash from the exact file:
//
//	curl -sL <url> | openssl dgst -sha384 -binary | openssl base64 -A
const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.34.1/swagger-ui.css" integrity="sha384-4c3vsE5f+trhFyHrFrLM1fFGUn531NIaC7dcqEWIJdoRQJ5CkJbSuSCwXLpaS1KF" crossorigin="anonymous">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.34.1/swagger-ui-bundle.js" integrity="sha384-XFYN47VzPiDY6qPtxYKg0nqUeW/upo20xf1j2SUE7gT/i35FJB4cuS/71Iegzk8Z" crossorigin="anonymous"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: %s,
        dom_id: "#swagger-ui"
      });
    };
  </script>
</body>
</html>`
