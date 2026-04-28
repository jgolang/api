package stdadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgolang/api"
)

func TestRouterRegistersServeMuxRouteAndMetadata(t *testing.T) {
	registry := api.NewRegistry(api.Info{Title: "Tasks API", Version: "1.0.0"})
	router := New(http.NewServeMux(), registry)

	api.Get(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}, api.Summary("List tasks"))

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	routes := registry.Routes()
	if len(routes) != 1 || routes[0].Summary != "List tasks" {
		t.Fatalf("route metadata was not registered: %#v", routes)
	}
}

func TestRouterAllowsMultipleMethodsForSamePath(t *testing.T) {
	router := New(http.NewServeMux(), nil)
	api.Get(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	api.Post(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", res.Code)
	}
}

func TestRouterMatchesBracePathParameters(t *testing.T) {
	router := New(http.NewServeMux(), nil)
	api.Get(router, "/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, response := api.GetRouteVarValueString("id", r)
		if response != nil {
			t.Fatalf("expected route var id, got response %#v", response)
		}
		if id != "42" {
			t.Fatalf("expected route var 42, got %s", id)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/tasks/42", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", res.Code)
	}
}

func TestRouterPrefersMoreSpecificRoutes(t *testing.T) {
	router := New(http.NewServeMux(), nil)
	api.Get(router, "/files/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	api.Get(router, "/files/static", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/files/static", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200 from static route, got %d", res.Code)
	}
}
