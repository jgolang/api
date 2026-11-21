package doc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveSwaggerUI(t *testing.T, openAPIURL string) string {
	t.Helper()
	res := httptest.NewRecorder()
	SwaggerUIHandler(openAPIURL)(res, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if got := res.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}
	return res.Body.String()
}

func TestSwaggerUIHandlerPinsAssetsWithIntegrity(t *testing.T) {
	page := serveSwaggerUI(t, "/openapi.json")

	for _, want := range []string{
		`<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.34.1/swagger-ui.css" integrity="sha384-4c3vsE5f+trhFyHrFrLM1fFGUn531NIaC7dcqEWIJdoRQJ5CkJbSuSCwXLpaS1KF" crossorigin="anonymous">`,
		`<script src="https://unpkg.com/swagger-ui-dist@5.34.1/swagger-ui-bundle.js" integrity="sha384-XFYN47VzPiDY6qPtxYKg0nqUeW/upo20xf1j2SUE7gT/i35FJB4cuS/71Iegzk8Z" crossorigin="anonymous"></script>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("expected page to contain %s, got:\n%s", want, page)
		}
	}
	if strings.Contains(page, "swagger-ui-dist@5/") {
		t.Fatalf("expected no unpinned asset URL, got:\n%s", page)
	}
}

func TestSwaggerUIHandlerEncodesURLForJavaScript(t *testing.T) {
	tests := []struct {
		name       string
		openAPIURL string
		want       string
	}{
		{name: "plain path", openAPIURL: "/openapi.json", want: `url: "/openapi.json",`},
		{name: "query with ampersand", openAPIURL: "/openapi.json?a=1&b=2", want: `url: "/openapi.json?a=1\u0026b=2",`},
		{name: "quote and script end", openAPIURL: `/x"</script><script>alert(1)//`, want: `url: "/x\"\u003c/script\u003e\u003cscript\u003ealert(1)//",`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := serveSwaggerUI(t, tt.openAPIURL)

			if !strings.Contains(page, tt.want) {
				t.Fatalf("expected page to contain %s, got:\n%s", tt.want, page)
			}
			if strings.Count(page, "</script>") != 2 {
				t.Fatalf("expected the URL not to close a script element, got:\n%s", page)
			}
		})
	}
}
