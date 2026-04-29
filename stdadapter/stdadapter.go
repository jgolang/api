package stdadapter

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
)

func init() {
	api.RegisterAdapter("std", func(target any, docs *doc.Docs) (api.Router, error) {
		if target == nil {
			return New(nil, docs), nil
		}
		mux, ok := target.(*http.ServeMux)
		if !ok {
			return nil, fmt.Errorf("std adapter expects *http.ServeMux")
		}
		return New(mux, docs), nil
	})
}

type route struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// Router adapts http.ServeMux to api.Router.
type Router struct {
	mux     *http.ServeMux
	docs    *doc.Docs
	mu      sync.Mutex
	routes  map[string][]route
	handled map[string]bool
}

// New creates a net/http adapter.
func New(mux *http.ServeMux, docs *doc.Docs) *Router {
	if mux == nil {
		mux = http.NewServeMux()
	}
	return &Router{
		mux:     mux,
		docs:    docs,
		routes:  make(map[string][]route),
		handled: make(map[string]bool),
	}
}

// ServeHTTP delegates to the wrapped ServeMux.
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}

// Handle registers an HTTP handler and stores its metadata.
func (router *Router) Handle(method string, path string, handler http.HandlerFunc, opts ...doc.RouteOption) {
	if router.docs != nil {
		router.docs.Register(method, path, opts...)
	}
	pattern := serveMuxPattern(path)
	router.mu.Lock()
	defer router.mu.Unlock()
	router.routes[pattern] = append(router.routes[pattern], route{
		method:  method,
		path:    normalizePath(path),
		handler: handler,
	})
	sort.SliceStable(router.routes[pattern], func(i, j int) bool {
		return routePriority(router.routes[pattern][i].path) > routePriority(router.routes[pattern][j].path)
	})
	if router.handled[pattern] {
		return
	}
	router.handled[pattern] = true
	router.mux.HandleFunc(pattern, router.dispatch(pattern))
}

func (router *Router) dispatch(pattern string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		router.mu.Lock()
		routes := append([]route(nil), router.routes[pattern]...)
		router.mu.Unlock()

		methodAllowed := false
		for _, registered := range routes {
			vars, match := matchPath(registered.path, r.URL.Path)
			if !match {
				continue
			}
			if registered.method != r.Method {
				methodAllowed = true
				continue
			}
			if len(vars) > 0 {
				r = api.SetRouteVars(vars, r)
			}
			registered.handler.ServeHTTP(w, r)
			return
		}
		if methodAllowed {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, r)
	}
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func serveMuxPattern(path string) string {
	path = normalizePath(path)
	index := strings.Index(path, "{")
	if index < 0 {
		return path
	}
	prefix := path[:index]
	slash := strings.LastIndex(prefix, "/")
	if slash <= 0 {
		return "/"
	}
	return prefix[:slash+1]
}

func matchPath(pattern string, path string) (map[string]string, bool) {
	pattern = normalizePath(pattern)
	path = normalizePath(path)
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	if len(patternParts) != len(pathParts) {
		return nil, false
	}
	vars := make(map[string]string)
	for i := range patternParts {
		part := patternParts[i]
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			if pathParts[i] == "" {
				return nil, false
			}
			vars[strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")] = pathParts[i]
			continue
		}
		if part != pathParts[i] {
			return nil, false
		}
	}
	return vars, true
}

func splitPath(path string) []string {
	path = strings.Trim(normalizePath(path), "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func routePriority(path string) int {
	priority := 0
	for _, part := range splitPath(path) {
		priority += 10
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		priority += 100
	}
	return priority
}
