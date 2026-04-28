package api

import "net/http"

// Router is the minimal contract implemented by router adapters.
type Router interface {
	Handle(method string, path string, handler http.HandlerFunc, opts ...RouteOption)
}

// Get registers a GET route in a router adapter.
func Get(router Router, path string, handler http.HandlerFunc, opts ...RouteOption) {
	router.Handle(http.MethodGet, path, handler, opts...)
}

// Post registers a POST route in a router adapter.
func Post(router Router, path string, handler http.HandlerFunc, opts ...RouteOption) {
	router.Handle(http.MethodPost, path, handler, opts...)
}

// Put registers a PUT route in a router adapter.
func Put(router Router, path string, handler http.HandlerFunc, opts ...RouteOption) {
	router.Handle(http.MethodPut, path, handler, opts...)
}

// Patch registers a PATCH route in a router adapter.
func Patch(router Router, path string, handler http.HandlerFunc, opts ...RouteOption) {
	router.Handle(http.MethodPatch, path, handler, opts...)
}

// Delete registers a DELETE route in a router adapter.
func Delete(router Router, path string, handler http.HandlerFunc, opts ...RouteOption) {
	router.Handle(http.MethodDelete, path, handler, opts...)
}
