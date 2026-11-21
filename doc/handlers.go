package doc

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
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

// SwaggerUIOption customizes the page served by SwaggerUIHandler.
type SwaggerUIOption func(*swaggerUIConfig)

type swaggerUIConfig struct {
	title   string
	favicon string
}

// WithTitle sets the page title shown in the browser tab. The title is
// HTML-escaped. An empty title keeps the default, "API Docs".
func WithTitle(title string) SwaggerUIOption {
	return func(config *swaggerUIConfig) {
		if title != "" {
			config.title = title
		}
	}
}

// WithFavicon sets the icon shown in the browser tab, replacing the generic
// default icon. href is a URL ("/static/icon.png") or a data: URI with a PNG,
// SVG or ICO image, and it is HTML-escaped. An empty href keeps the default.
// Percent-encode (url.PathEscape) or base64-encode an SVG data: URI, since a
// raw "#" starts a URL fragment and cuts the image.
func WithFavicon(href string) SwaggerUIOption {
	return func(config *swaggerUIConfig) {
		if href != "" {
			config.favicon = href
		}
	}
}

// SwaggerUIHandler returns a handler that serves Swagger UI for an OpenAPI URL.
// The page loads a pinned swagger-ui-dist release from unpkg, checked with
// Subresource Integrity hashes. Without options the page is titled "API Docs"
// and shows a generic inline icon; WithTitle and WithFavicon change them.
// Options are applied once, when the handler is built; nil options are skipped.
func SwaggerUIHandler(openAPIURL string, opts ...SwaggerUIOption) http.HandlerFunc {
	// Percent-encoding keeps raw <, >, ", # and spaces out of the data: URI.
	defaultFavicon := "data:image/svg+xml," + url.PathEscape(defaultFaviconSVG)
	config := swaggerUIConfig{title: "API Docs", favicon: defaultFavicon}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	// html.EscapeString, not html/template: the template URL filter rewrites
	// data: URIs to "#ZgotmplZ", and a data: URI is a supported favicon.
	title := html.EscapeString(config.title)
	favicon := html.EscapeString(config.favicon)
	// json.Marshal quotes the URL as a JavaScript string literal and escapes
	// <, > and &, so it cannot end the script element. A string always encodes.
	encodedURL, _ := json.Marshal(openAPIURL)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, swaggerUIHTML, title, favicon, encodedURL)
	}
}

// defaultFaviconSVG is an original "{ }" glyph in a mid gray that stays
// readable on light and dark browser tabs. It is inlined so the page needs no
// extra request.
const defaultFaviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" fill="none" stroke="#808080" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4C9 4 8 5.5 8 8v4c0 2-1 3-3 4 2 1 3 2 3 4v4c0 2.5 1 4 4 4"/><path d="M20 4c3 0 4 1.5 4 4v4c0 2 1 3 3 4-2 1-3 2-3 4v4c0 2.5-1 4-4 4"/></svg>`

// swaggerUIHTML pins swagger-ui-dist 5.34.1. When updating the version,
// recompute each integrity hash from the exact file:
//
//	curl -sL <url> | openssl dgst -sha384 -binary | openssl base64 -A
const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <link rel="icon" href="%s">
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
