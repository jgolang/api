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

func ExampleWithKindStatus() {
	a := api.New(api.WithKindStatus(codes.KindValidation, http.StatusUnprocessableEntity))
	handler := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, errors.NewC(codes.ValRequired, "title"))
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/tasks", nil))

	fmt.Println(recorder.Code)
	// Output:
	// 422
}

func ExampleWithErrorHook() {
	a := api.New(api.WithErrorHook(func(r *http.Request, err error, status int) {
		fmt.Println(status, err) // log or count the error here
	}))
	handler := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, fmt.Errorf("loading task: database unavailable"))
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tasks/42", nil))
	// Output:
	// 500 loading task: database unavailable
}
