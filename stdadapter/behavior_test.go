package stdadapter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgolang/api"
)

func serve(router *Router, method, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

func writeBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }
}

func TestRouterBehavior(t *testing.T) {
	quiet := api.PrintError
	api.PrintError = func(...interface{}) {}
	t.Cleanup(func() { api.PrintError = quiet })

	router := New(nil, nil)
	router.Handle(http.MethodGet, "/", writeBody("root"))
	router.Handle(http.MethodGet, "/tasks/", writeBody("tasks-slash"))
	router.Handle(http.MethodGet, "/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "task "+api.GetRouteVar("id", r))
	})
	router.Handle(http.MethodGet, "/tasks/{name}", writeBody("second-wildcard"))
	router.Handle(http.MethodGet, "/dup", writeBody("first"))
	router.Handle(http.MethodGet, "/dup", writeBody("second"))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "root is exact", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "root"},
		{name: "root does not match other paths", method: http.MethodGet, path: "/nothing", wantStatus: http.StatusNotFound},
		{name: "trailing slash is exact", method: http.MethodGet, path: "/tasks/", wantStatus: http.StatusOK, wantBody: "tasks-slash"},
		{name: "wildcard", method: http.MethodGet, path: "/tasks/42", wantStatus: http.StatusOK, wantBody: "task 42"},
		{name: "deeper path is not matched", method: http.MethodGet, path: "/tasks/42/x", wantStatus: http.StatusNotFound},
		{name: "first conflicting wildcard wins", method: http.MethodGet, path: "/tasks/7", wantStatus: http.StatusOK, wantBody: "task 7"},
		{name: "first duplicate wins", method: http.MethodGet, path: "/dup", wantStatus: http.StatusOK, wantBody: "first"},
		{name: "wrong method", method: http.MethodPost, path: "/tasks/42", wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := serve(router, tt.method, tt.path)

			if recorder.Code != tt.wantStatus || (tt.wantBody != "" && recorder.Body.String() != tt.wantBody) {
				t.Fatalf("%s %s = %d %q, want %d %q", tt.method, tt.path, recorder.Code, recorder.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}

func BenchmarkRouterServe(b *testing.B) {
	router := New(nil, nil)
	for _, path := range []string{"/tasks", "/tasks/{id}", "/tasks/{id}/notes", "/projects/{id}", "/goals/{id}"} {
		router.Handle(http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) {})
	}
	request := httptest.NewRequest(http.MethodGet, "/tasks/42/notes", nil)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(httptest.NewRecorder(), request)
	}
}
