package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jgolang/api"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

func ExampleStatusForKind() {
	fmt.Println(api.StatusForKind(codes.KindNotFound))
	fmt.Println(api.StatusForKind(""))
	// Output:
	// 404
	// 500
}

func ExampleWriteError() {
	errTaskNotFound := errors.Define(codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound), "task not found")
	recorder := httptest.NewRecorder()

	api.WriteError(recorder, httptest.NewRequest(http.MethodGet, "/tasks/42", nil), errors.Wrap(errTaskNotFound))

	fmt.Println(recorder.Code)
	// Output:
	// 404
}
