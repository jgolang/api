package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jgolang/api"
	"github.com/jgolang/api/core"
)

func ExampleNew() {
	a := api.New(api.WithResponseMode(api.ResponseModePlain))
	handler := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.SuccessWithContent(map[string]string{"status": "ok"}).Write(w, r)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	fmt.Println(rec.Code)
	fmt.Print(rec.Body.String())
	// Output:
	// 200
	// {"action":"","code":"0000","content":{"status":"ok"},"event_id":"","info":null,"message":"","title":"","token":"","type":"success"}
}

// statusOnlyWriter writes the status code and no body.
type statusOnlyWriter struct{}

func (statusOnlyWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	w.WriteHeader(response.HTTPStatusCode)
}

func ExampleWithResponseWriter() {
	a := api.New(api.WithResponseWriter(statusOnlyWriter{}))
	handler := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.Success200().Write(w, r)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	fmt.Println(rec.Code, rec.Body.Len())
	// Output: 200 0
}
