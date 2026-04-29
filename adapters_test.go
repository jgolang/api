package api

import (
	"net/http"
	"testing"

	"github.com/jgolang/api/doc"
)

type testAdapterRouter struct{}

func (router testAdapterRouter) Handle(method string, path string, handler http.HandlerFunc, opts ...doc.RouteOption) {
}

func TestRegisteredAdapterCreatesRouter(t *testing.T) {
	const name = "test-adapter"
	expectedTarget := struct{}{}
	expectedDocs := doc.New(doc.API{Title: "Test API", Version: "1.0.0"})

	RegisterAdapter(name, func(target any, docs *doc.Docs) (Router, error) {
		if target != expectedTarget {
			t.Fatalf("unexpected target: %#v", target)
		}
		if docs != expectedDocs {
			t.Fatalf("unexpected docs: %#v", docs)
		}
		return testAdapterRouter{}, nil
	})

	router, err := NewRouter(name, expectedTarget, expectedDocs)
	if err != nil {
		t.Fatalf("expected registered adapter, got error: %v", err)
	}
	if router == nil {
		t.Fatalf("expected router")
	}
}

func TestNewRouterReturnsErrorForMissingAdapter(t *testing.T) {
	router, err := NewRouter("missing-adapter", nil, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if router != nil {
		t.Fatalf("expected nil router, got %#v", router)
	}
}

func TestRegisterAdapterIgnoresInvalidInput(t *testing.T) {
	RegisterAdapter("", func(target any, docs *doc.Docs) (Router, error) {
		return testAdapterRouter{}, nil
	})
	RegisterAdapter("nil-factory", nil)

	if _, err := NewRouter("", nil, nil); err == nil {
		t.Fatalf("empty adapter name should not be registered")
	}
	if _, err := NewRouter("nil-factory", nil, nil); err == nil {
		t.Fatalf("nil adapter factory should not be registered")
	}
}
