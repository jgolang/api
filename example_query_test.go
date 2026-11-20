package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jgolang/api"
	"github.com/jgolang/errors/codes"
)

func ExampleQueryInt() {
	_, ok, err := api.QueryInt(httptest.NewRequest(http.MethodGet, "/tasks?limit=abc", nil), "limit")
	fmt.Println(ok, err)

	_, ok, err = api.QueryInt(httptest.NewRequest(http.MethodGet, "/tasks", nil), "limit")
	fmt.Println(ok, err)
	// Output:
	// true [b002](Invalid format.): query parameter "limit" must be an integer
	// false <nil>
}

func ExampleQueryEnum() {
	inboxInvalid := codes.New("PLN-051", "in_inbox only accepts true").WithKind(codes.KindValidation)
	r := httptest.NewRequest(http.MethodGet, "/tasks?in_inbox=false", nil)

	_, _, err := api.QueryEnum(r, "in_inbox", []string{"true"}, api.WithQueryCode(inboxInvalid))
	fmt.Println(err)
	// Output: [PLN-051](in_inbox only accepts true): query parameter "in_inbox" must be one of true
}
