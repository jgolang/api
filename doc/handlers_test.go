package doc

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
)

func serveSwaggerUI(t *testing.T, openAPIURL string, opts ...SwaggerUIOption) string {
	t.Helper()
	res := httptest.NewRecorder()
	SwaggerUIHandler(openAPIURL, opts...)(res, httptest.NewRequest(http.MethodGet, "/docs", nil))
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

func TestSwaggerUIHandlerDefaultPageMatchesGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/swagger-ui-default.html")
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	page := serveSwaggerUI(t, "/openapi.json")

	if page != string(want) {
		t.Fatalf("default page differs from golden file, got:\n%s", page)
	}
}

func TestSwaggerUIHandlerSkipsNilOption(t *testing.T) {
	want := serveSwaggerUI(t, "/openapi.json")

	page := serveSwaggerUI(t, "/openapi.json", nil)

	if page != want {
		t.Fatalf("expected a nil option to keep the default page, got:\n%s", page)
	}
}

var faviconHrefPattern = regexp.MustCompile(`<link rel="icon" href="([^"]*)">`)

func TestSwaggerUIHandlerDefaultFaviconIsInlineSVG(t *testing.T) {
	page := serveSwaggerUI(t, "/openapi.json")

	match := faviconHrefPattern.FindStringSubmatch(page)
	if match == nil {
		t.Fatalf("expected a favicon link, got:\n%s", page)
	}
	href := match[1]
	const prefix = "data:image/svg+xml,"
	if !strings.HasPrefix(href, prefix) {
		t.Fatalf("expected href to start with %q, got %q", prefix, href)
	}
	if strings.ContainsAny(href, "<>\"# ") {
		t.Fatalf("expected href without raw <>\"# or spaces, got %q", href)
	}
	svg, err := url.PathUnescape(strings.TrimPrefix(href, prefix))
	if err != nil {
		t.Fatalf("unescape href: %v", err)
	}
	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal([]byte(svg), &root); err != nil {
		t.Fatalf("expected well-formed SVG, got %v for %q", err, svg)
	}
	if root.XMLName.Local != "svg" {
		t.Fatalf("expected root element svg, got %q", root.XMLName.Local)
	}
}

func TestSwaggerUIHandlerTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{name: "custom", title: "Orders API", want: "<title>Orders API</title>"},
		{name: "empty keeps default", title: "", want: "<title>API Docs</title>"},
		{name: "markup is escaped", title: "</title><script>x</script>", want: "<title>&lt;/title&gt;&lt;script&gt;x&lt;/script&gt;</title>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := serveSwaggerUI(t, "/openapi.json", WithTitle(tt.title))

			if !strings.Contains(page, tt.want) {
				t.Fatalf("expected page to contain %s, got:\n%s", tt.want, page)
			}
			if strings.Count(page, "</script>") != 2 {
				t.Fatalf("expected the title not to add a script element, got:\n%s", page)
			}
		})
	}
}

func TestSwaggerUIHandlerFavicon(t *testing.T) {
	defaultLink := faviconHrefPattern.FindString(serveSwaggerUI(t, "/openapi.json"))
	tests := []struct {
		name    string
		favicon string
		want    string
	}{
		{name: "url", favicon: "/static/icon.png", want: `<link rel="icon" href="/static/icon.png">`},
		{name: "data uri", favicon: `data:image/svg+xml,<svg a="b&c"></svg>`, want: `<link rel="icon" href="data:image/svg+xml,&lt;svg a=&#34;b&amp;c&#34;&gt;&lt;/svg&gt;">`},
		{name: "markup is escaped", favicon: `x"><script>alert(1)</script>`, want: `<link rel="icon" href="x&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;">`},
		{name: "empty keeps default", favicon: "", want: defaultLink},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := serveSwaggerUI(t, "/openapi.json", WithFavicon(tt.favicon))

			if !strings.Contains(page, tt.want) {
				t.Fatalf("expected page to contain %s, got:\n%s", tt.want, page)
			}
			if strings.Count(page, `rel="icon"`) != 1 {
				t.Fatalf("expected exactly one favicon link, got:\n%s", page)
			}
			if strings.Count(page, "</script>") != 2 {
				t.Fatalf("expected the favicon not to add a script element, got:\n%s", page)
			}
		})
	}
}
