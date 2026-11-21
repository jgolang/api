package api_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

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

func ExampleWithResponseMode_problem() {
	taskNotFound := codes.New("PLN-404", "task not found").WithKind(codes.KindNotFound)
	a := api.New(api.WithResponseMode(api.ResponseModeProblem))
	handler := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, errors.NewC(taskNotFound, "task 7"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/7", nil)
	req.Header.Set(api.EventIDHeaderKey, "evt-123")
	handler.ServeHTTP(rec, req)

	// A generic RFC 9457 client: the standard members plus the code extension.
	var problem struct {
		Type     string `json:"type"`
		Title    string `json:"title"`
		Status   int    `json:"status"`
		Detail   string `json:"detail"`
		Instance string `json:"instance"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(rec.Header().Get("Content-Type"))
	fmt.Printf("%s | %s | %d | %s | %q | %s\n", problem.Type, problem.Title, problem.Status, problem.Detail, problem.Instance, problem.Code)
	// Output:
	// application/problem+json
	// about:blank | Error response! | 404 | task not found | "" | PLN-404
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

func ExampleWithLogger() {
	// Drop the attributes that change on every run, so the output is stable.
	dropVolatile := func(groups []string, attr slog.Attr) slog.Attr {
		switch attr.Key {
		case slog.TimeKey, "event_id", "duration_ms":
			return slog.Attr{}
		}
		return attr
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: dropVolatile}))
	a := api.New(api.WithLogger(logger), api.WithSensitiveLogKeys("pin"))
	handler := a.Handler(api.ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		api.Success200().Write(w, r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/login?access_token=placeholder&page=2", nil)
	req.Header.Set("Pin", "0000")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	// Output:
	// level=INFO msg=request method=GET uri="/login?access_token=[REDACTED]&page=2" headers=map[Pin:[[REDACTED]]]
	// level=INFO msg=response method=GET uri="/login?access_token=[REDACTED]&page=2" status=200 status_text=OK
}
