package doc

import (
	"net/http"
	"testing"
)

func TestGenerateOpenAPISkipsHiddenOperationOnSharedPath(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks")
	docs.Register(http.MethodPost, "/tasks", Hidden())

	operations := GenerateOpenAPI(docs).Paths["/tasks"]

	if _, ok := operations["get"]; !ok {
		t.Fatalf("expected visible GET /tasks, got %v", operations)
	}
	if _, ok := operations["post"]; ok {
		t.Fatalf("expected hidden POST /tasks absent, got %v", operations)
	}
}

func TestGenerateOpenAPISkipsPathWithOnlyHiddenRoutes(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/openapi.json", Hidden())

	paths := GenerateOpenAPI(docs).Paths

	if _, ok := paths["/openapi.json"]; ok {
		t.Fatalf("expected hidden-only path absent, got %v", paths)
	}
}

func TestGenerateOpenAPIAddsNoSchemeForHiddenRoute(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/docs", Hidden(), Security("hiddenAuth"))

	spec := GenerateOpenAPI(docs)

	if spec.Components == nil {
		return
	}
	if _, ok := spec.Components.SecuritySchemes["hiddenAuth"]; ok {
		t.Fatalf("expected no scheme for a hidden route, got %v", spec.Components.SecuritySchemes)
	}
}

func TestRoutesKeepHiddenRoutes(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/docs", Hidden())

	routes := docs.Routes()

	if len(routes) != 1 || !routes[0].Hidden {
		t.Fatalf("expected the hidden route kept with Hidden set, got %#v", routes)
	}
}
