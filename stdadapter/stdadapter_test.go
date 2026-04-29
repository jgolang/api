package stdadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
)

func TestRouterRegistersServeMuxRouteAndMetadata(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := New(http.NewServeMux(), docs)

	api.Get(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}, doc.Summary("List tasks"))

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	routes := docs.Routes()
	if len(routes) != 1 || routes[0].Summary != "List tasks" {
		t.Fatalf("route metadata was not registered: %#v", routes)
	}
}

func TestRegisteredAdapterCreatesRouter(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router, err := api.NewRouter("std", http.NewServeMux(), docs)
	if err != nil {
		t.Fatalf("expected std adapter, got error: %v", err)
	}

	api.Get(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}, doc.Summary("List tasks"))

	routes := docs.Routes()
	if len(routes) != 1 || routes[0].Summary != "List tasks" {
		t.Fatalf("route metadata was not registered: %#v", routes)
	}
}

func TestRegisteredAdapterRejectsInvalidTarget(t *testing.T) {
	router, err := api.NewRouter("std", struct{}{}, nil)
	if err == nil {
		t.Fatalf("expected invalid target error")
	}
	if router != nil {
		t.Fatalf("expected nil router, got %#v", router)
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
