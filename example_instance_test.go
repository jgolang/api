package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jgolang/api"
	"github.com/jgolang/api/core"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
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

func ExampleWithLowercaseCodes() {
	titleRequired := codes.New("PLN-001", "title is required").WithKind(codes.KindValidation)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, errors.NewC(titleRequired, "empty title"))
	})

	for _, a := range []*api.API{api.New(), api.New(api.WithLowercaseCodes())} {
		rec := httptest.NewRecorder()
		a.Handler(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tasks", nil))

		var body struct {
			Header struct {
				Code string `json:"code"`
			} `json:"header"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(body.Header.Code)
	}
	// Output:
	// PLN-001
	// pln-001
}
