package stdadapter

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
)

func serve(router *Router, method, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

func writeBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }
}

func writeRouteVar(prefix, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, prefix+api.GetRouteVar(name, r))
	}
}

// TestRouterBehavior pins the ServeMux behavior changes listed in
// docs/adr/0002-stdadapter-servemux.md.
func TestRouterBehavior(t *testing.T) {
	router := New(nil, nil)
	router.Handle(http.MethodGet, "/", writeBody("root"))
	router.Handle(http.MethodGet, "/tasks/{id}", writeRouteVar("task ", "id"))
	router.Handle(http.MethodPut, "/tasks/{id}", writeBody("put"))
	router.Handle(http.MethodGet, "/tasks/new", writeBody("new"))
	router.Handle(http.MethodGet, "/lists/", writeBody("lists-slash"))
	router.Handle(http.MethodGet, "/files/{name}", writeRouteVar("file ", "name"))
	router.Handle(http.MethodGet, "/tasks", writeBody("tasks"))
	router.Handle(http.MethodGet, "/one/{name}", writeRouteVar("one ", "name"))
	router.Handle(http.MethodGet, "/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "path value "+r.PathValue("id"))
	})
	router.Handle(http.MethodGet, "/projects/{project}/tasks/{task}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, api.GetRouteVar("project", r)+"/"+api.GetRouteVar("task", r))
	})

	tests := []struct {
		name         string
		method       string
		path         string
		wantStatus   int
		wantBody     string
		wantAllow    string
		wantLocation string
	}{
		{name: "root is exact", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "root"},
		{name: "root does not match other paths", method: http.MethodGet, path: "/nothing", wantStatus: http.StatusNotFound},
		{name: "wildcard sets route var", method: http.MethodGet, path: "/tasks/42", wantStatus: http.StatusOK, wantBody: "task 42"},
		{name: "1 most specific route wins", method: http.MethodGet, path: "/tasks/new", wantStatus: http.StatusOK, wantBody: "new"},
		{name: "2 trailing slash does not match route without one", method: http.MethodGet, path: "/tasks/42/", wantStatus: http.StatusNotFound},
		{name: "2 unchanged: trailing slash route matches only itself", method: http.MethodGet, path: "/lists/42", wantStatus: http.StatusNotFound},
		{name: "2 trailing slash route", method: http.MethodGet, path: "/lists/", wantStatus: http.StatusOK, wantBody: "lists-slash"},
		{name: "3 escaped slash matches a wildcard", method: http.MethodGet, path: "/files/a%2Fb", wantStatus: http.StatusOK, wantBody: "file a/b"},
		{name: "3 escaped dot segments reach a wildcard", method: http.MethodGet, path: "/one/..%2F..%2Fetc", wantStatus: http.StatusOK, wantBody: "one ../../etc"},
		{name: "r.PathValue works", method: http.MethodGet, path: "/notes/7", wantStatus: http.StatusOK, wantBody: "path value 7"},
		{name: "several route vars", method: http.MethodGet, path: "/projects/p1/tasks/t2", wantStatus: http.StatusOK, wantBody: "p1/t2"},
		{name: "unchanged: path without trailing slash redirects to the slash route", method: http.MethodGet, path: "/lists", wantStatus: http.StatusTemporaryRedirect, wantLocation: "/lists/"},
		{name: "5 method not allowed lists allowed methods", method: http.MethodDelete, path: "/tasks/42", wantStatus: http.StatusMethodNotAllowed, wantAllow: "GET, HEAD, PUT"},
		{name: "6 GET route answers HEAD", method: http.MethodHead, path: "/tasks/42", wantStatus: http.StatusOK, wantBody: "task 42"},
		{name: "7 unchanged: double slash redirects", method: http.MethodGet, path: "//tasks", wantStatus: http.StatusTemporaryRedirect, wantLocation: "/tasks"},
		{name: "7 unchanged: dot segments redirect", method: http.MethodGet, path: "/a/../tasks", wantStatus: http.StatusTemporaryRedirect, wantLocation: "/tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := serve(router, tt.method, tt.path)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.path, recorder.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && recorder.Body.String() != tt.wantBody {
				t.Fatalf("%s %s body = %q, want %q", tt.method, tt.path, recorder.Body.String(), tt.wantBody)
			}
			if got := recorder.Header().Get("Allow"); got != tt.wantAllow {
				t.Fatalf("%s %s Allow = %q, want %q", tt.method, tt.path, got, tt.wantAllow)
			}
			if got := recorder.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("%s %s Location = %q, want %q", tt.method, tt.path, got, tt.wantLocation)
			}
		})
	}
}

// TestRouterHandlePanicsOnBadRoute pins changes 1 (conflicts) and 4 (names and
// partial segments) and the method and wildcard syntax checks: Handle panics,
// reports nothing, documents nothing for the rejected route, and keeps serving
// the first one.
func TestRouterHandlePanicsOnBadRoute(t *testing.T) {
	tests := []struct {
		name         string
		first        string
		firstRequest string
		method       string
		path         string
		wantPanic    string
	}{
		{name: "1 wildcards in the same place", first: "/tasks/{id}", firstRequest: "/tasks/1", method: http.MethodGet, path: "/tasks/{name}", wantPanic: "conflicts with"},
		{name: "1 same method and path", first: "/tasks/{id}", firstRequest: "/tasks/1", method: http.MethodGet, path: "/tasks/{id}", wantPanic: "conflicts with"},
		{name: "1 crossing routes, neither more specific", first: "/tasks/{id}/notes", firstRequest: "/tasks/1/notes", method: http.MethodGet, path: "/tasks/new/{kind}", wantPanic: "conflicts with"},
		{name: "4 wildcard name is not an identifier", first: "/users", firstRequest: "/users", method: http.MethodGet, path: "/users/{user-id}", wantPanic: "bad wildcard"},
		{name: "4 wildcard is not a whole segment", first: "/files", firstRequest: "/files", method: http.MethodGet, path: "/files/{name}.json", wantPanic: "bad wildcard"},
		{name: "rest wildcard is not supported", first: "/files", firstRequest: "/files", method: http.MethodGet, path: "/files/{rest...}", wantPanic: "unsupported wildcard"},
		{name: "end anchor is not supported", first: "/files", firstRequest: "/files", method: http.MethodGet, path: "/x/{$}", wantPanic: "unsupported wildcard"},
		{name: "empty method", first: "/any", firstRequest: "/any", method: "", path: "/other", wantPanic: "invalid method"},
		{name: "method with a space", first: "/any", firstRequest: "/any", method: "GE T", path: "/other", wantPanic: "invalid method"},
		{name: "lowercase method", first: "/any", firstRequest: "/any", method: "get", path: "/other", wantPanic: "invalid method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printed := 0
			previous := api.PrintError
			api.PrintError = func(...interface{}) { printed++ }
			t.Cleanup(func() { api.PrintError = previous })
			docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
			router := New(nil, docs)
			router.Handle(http.MethodGet, tt.first, writeBody("first"))

			recovered := handleRecovering(router, tt.method, tt.path)

			if recovered == nil {
				t.Fatalf("Handle(%q, %q) did not panic", tt.method, tt.path)
			}
			if message := fmt.Sprint(recovered); !strings.Contains(message, tt.wantPanic) {
				t.Fatalf("panic = %q, want it to contain %q", message, tt.wantPanic)
			}
			if printed != 0 {
				t.Fatalf("api.PrintError called %d times, want 0", printed)
			}
			if routes := docs.Routes(); len(routes) != 1 {
				t.Fatalf("documented routes = %d, want only the first one", len(routes))
			}
			if recorder := serve(router, http.MethodGet, tt.firstRequest); recorder.Code != http.StatusOK || recorder.Body.String() != "first" {
				t.Fatalf("first route = %d %q, want 200 \"first\"", recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestRouterOnCallerMuxWithCatchAll shows that a catch-all "/" already on the
// caller's mux answers what would otherwise be 404 or 405 (changes 1 and 5).
func TestRouterOnCallerMuxWithCatchAll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", writeBody("catch-all"))
	router := New(mux, nil)
	router.Handle(http.MethodGet, "/tasks/{id}", writeRouteVar("task ", "id"))

	recorder := serve(router, http.MethodDelete, "/tasks/42")

	if recorder.Code != http.StatusOK || recorder.Body.String() != "catch-all" {
		t.Fatalf("DELETE /tasks/42 = %d %q, want 200 \"catch-all\"", recorder.Code, recorder.Body.String())
	}
}

func handleRecovering(router *Router, method, path string) (recovered any) {
	defer func() { recovered = recover() }()
	router.Handle(method, path, writeBody("second"))
	return nil
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
