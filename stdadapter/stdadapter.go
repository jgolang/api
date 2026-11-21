package stdadapter

import (
	"fmt"
	"net/http"
	"strings"

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

// Router adapts http.ServeMux to api.Router. Routes are registered as Go
// 1.22 ServeMux patterns ("GET /tasks/{id}"), so matching, precedence, 404 and
// 405 come from net/http.
type Router struct {
	mux  *http.ServeMux
	docs *doc.Docs
}

// New creates a net/http adapter.
func New(mux *http.ServeMux, docs *doc.Docs) *Router {
	if mux == nil {
		mux = http.NewServeMux()
	}
	return &Router{mux: mux, docs: docs}
}

// ServeHTTP delegates to the wrapped ServeMux.
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}

// Handle registers handler for method and path and stores its metadata.
//
// The route becomes the Go 1.22 ServeMux pattern "METHOD path" and is matched
// exactly: a trailing slash matches only itself, not the paths below it.
// {name} segments must be whole segments named by Go identifiers; their values
// are read with api.GetRouteVar or r.PathValue. {name} is the only wildcard
// syntax: ServeMux's {name...} and {$} are not supported. Precedence, 404, 405
// and HEAD for GET routes come from http.ServeMux.
//
// Handle panics, as http.ServeMux.Handle does, when method is not an
// upper-case RFC 9110 token ("GET", not "get" or ""), when path is not a valid
// pattern or uses {name...} or {$}, or when the route conflicts with one
// registered before: some request path matches both and neither is more
// specific (the same method and path, wildcards in the same place, or crossing
// routes such as /tasks/{id}/notes and /tasks/new/{kind}). A conflict is a
// programming error in the route table. A rejected route is neither served nor
// documented. See docs/adr/0002-stdadapter-servemux.md.
func (router *Router) Handle(method string, path string, handler http.HandlerFunc, opts ...doc.RouteOption) {
	if !validMethod(method) {
		panic(fmt.Sprintf("stdadapter: invalid method %q for %q: want an upper-case RFC 9110 token such as GET", method, path))
	}
	if strings.Contains(path, "...}") || strings.Contains(path, "{$}") {
		panic(fmt.Sprintf("stdadapter: unsupported wildcard in %q: only {name} is supported", path))
	}
	names := pathVarNames(path)
	router.mux.HandleFunc(method+" "+exactPattern(path), func(w http.ResponseWriter, r *http.Request) {
		if len(names) > 0 {
			vars := make(map[string]string, len(names))
			for _, name := range names {
				vars[name] = r.PathValue(name)
			}
			r = api.SetRouteVars(vars, r)
		}
		handler(w, r)
	})
	if router.docs != nil {
		router.docs.Register(method, path, opts...)
	}
}

// validMethod reports whether method is a non-empty RFC 9110 token with no
// lower-case letters, so it matches the methods clients send and the
// documented route.
func validMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, char := range method {
		if !isTokenChar(char) || (char >= 'a' && char <= 'z') {
			return false
		}
	}
	return true
}

// isTokenChar reports whether char is an RFC 9110 tchar.
func isTokenChar(char rune) bool {
	switch {
	case char >= 'A' && char <= 'Z', char >= 'a' && char <= 'z', char >= '0' && char <= '9':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", char)
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

// exactPattern returns the ServeMux path for path. A trailing slash gets {$}
// so it matches only itself, not every path below it.
func exactPattern(path string) string {
	path = normalizePath(path)
	if strings.HasSuffix(path, "/") {
		return path + "{$}"
	}
	return path
}

// pathVarNames returns the names of the {name} segments in path, computed once
// at registration.
func pathVarNames(path string) []string {
	var names []string
	for _, part := range strings.Split(normalizePath(path), "/") {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"))
		}
	}
	return names
}
