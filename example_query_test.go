package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jgolang/api"
)

func ExampleQueryInt() {
	r := httptest.NewRequest(http.MethodGet, "/tasks?limit=abc", nil)

	_, ok, err := api.QueryInt(r, "limit")
	fmt.Println(ok, err)

	_, ok, err = api.QueryInt(httptest.NewRequest(http.MethodGet, "/tasks", nil), "limit")
	fmt.Println(ok, err)
	// Output:
	// true [b002](Invalid format.): query parameter "limit" must be an integer
	// false <nil>
}
