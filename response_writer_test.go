package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jgolang/api/core"
)

func TestResponseWriterAnswers500WhenBodyCannotBeEncoded(t *testing.T) {
	previousFatal, previousPrintError := Fatal, PrintError
	var fatalCalled bool
	var logged string
	Fatal = func(...interface{}) { fatalCalled = true }
	PrintError = func(args ...interface{}) { logged += fmt.Sprintln(args...) }
	defer func() { Fatal, PrintError = previousFatal, previousPrintError }()

	req := httptest.NewRequest(http.MethodGet, "/ratio", nil)
	res := httptest.NewRecorder()
	SuccessWithContent(map[string]float64{"ratio": math.NaN()}).Write(res, req)

	if fatalCalled {
		t.Fatal("expected Fatal not to be called")
	}
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", res.Code)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected a single JSON body, got %q: %v", res.Body.String(), err)
	}
	header, _ := body["header"].(map[string]any)
	wantCode := strings.ToLower(string(ResponseCodes.InternalServerEerror))
	if header["code"] != wantCode {
		t.Fatalf("expected code %q, got %#v", wantCode, body)
	}
	if !strings.Contains(logged, "NaN") {
		t.Fatalf("expected the encoding error to be logged, got %q", logged)
	}
}

func TestResponseWriterKeepsStatusAndHeadersOnSuccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ratio", nil)
	res := httptest.NewRecorder()
	Success{
		Content: map[string]float64{"ratio": 0.5},
		Headers: map[string]string{"X-Custom": "value"},
	}.Write(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if got := res.Header().Get("X-Custom"); got != "value" {
		t.Fatalf("expected custom header, got %q", got)
	}
	if !strings.Contains(res.Body.String(), `"ratio":0.5`) {
		t.Fatalf("expected encoded content, got %q", res.Body.String())
	}
}

// Warning and Informative default to status 100. A real server sends it as an
// informational status and delivers the body with 200; the recorder that
// ProcessRequest writes into rejects the body with http.ErrBodyNotAllowed,
// which must neither exit the process nor be logged.
func TestDefaultInformationalStatusReachesClientWithoutFatal(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.HandlerFunc
		wantType string
	}{
		{name: "Warning", handler: func(w http.ResponseWriter, r *http.Request) { Warning{}.Write(w, r) }, wantType: "warning"},
		{name: "Informative", handler: func(w http.ResponseWriter, r *http.Request) { Informative{}.Write(w, r) }, wantType: "info"},
		{name: "Warning under ProcessRequest", handler: ProcessRequest(func(w http.ResponseWriter, r *http.Request) { Warning{}.Write(w, r) }), wantType: "warning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousFatal, previousPrintError := Fatal, PrintError
			var mu sync.Mutex
			var fatalCalled bool
			var logged string
			Fatal = func(...interface{}) { mu.Lock(); fatalCalled = true; mu.Unlock() }
			PrintError = func(args ...interface{}) { mu.Lock(); logged += fmt.Sprintln(args...); mu.Unlock() }
			server := httptest.NewServer(tt.handler)
			defer func() { Fatal, PrintError = previousFatal, previousPrintError }()
			defer server.Close()

			res, err := http.Get(server.URL + "/tasks")
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			body, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Fatalf("reading body failed: %v", err)
			}
			server.Close()

			mu.Lock()
			defer mu.Unlock()
			if fatalCalled {
				t.Fatal("expected Fatal not to be called")
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("expected the client to get status 200, got %d", res.StatusCode)
			}
			if !strings.Contains(string(body), `"type":"`+tt.wantType+`"`) {
				t.Fatalf("expected a %s body, got %q", tt.wantType, body)
			}
			if logged != "" {
				t.Fatalf("expected nothing logged, got %q", logged)
			}
		})
	}
}

// errorMarkerFormatter formats like the default formatter but replaces error
// bodies, to show which formatter built the 500 body.
type errorMarkerFormatter struct{}

func (errorMarkerFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	formatted := ResponseFormatter{}.Format(data)
	if data.ResponseType == ErrorType {
		formatted.Body = map[string]string{"marker": "registered formatter", "code": string(data.ResponseCode)}
	}
	return formatted
}

func TestResponseWriterBuildsFailureWithRegisteredFormatterAndHeaders(t *testing.T) {
	previousPrintError := PrintError
	PrintError = func(...interface{}) {}
	RegisterNewAPIResponseFormatter(errorMarkerFormatter{})
	defer func() {
		PrintError = previousPrintError
		RegisterNewAPIResponseFormatter(ResponseFormatter{})
	}()

	res := httptest.NewRecorder()
	Success{
		Content: map[string]float64{"ratio": math.NaN()},
		Headers: map[string]string{"X-Custom": "value"},
	}.Write(res, httptest.NewRequest(http.MethodGet, "/ratio", nil))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "registered formatter") {
		t.Fatalf("expected the registered formatter to build the body, got %q", res.Body.String())
	}
	if got := res.Header().Get("X-Custom"); got != "value" {
		t.Fatalf("expected the response headers to be kept, got %q", got)
	}
}

func TestResponseWriterFailureAfterStatusWrittenKeepsEarlierStatus(t *testing.T) {
	previousPrintError := PrintError
	PrintError = func(...interface{}) {}
	defer func() { PrintError = previousPrintError }()

	res := httptest.NewRecorder()
	res.WriteHeader(http.StatusAccepted)
	SuccessWithContent(map[string]float64{"ratio": math.NaN()}).Write(res, httptest.NewRequest(http.MethodGet, "/ratio", nil))

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected the earlier status 202 to stay, got %d", res.Code)
	}
	wantCode := strings.ToLower(string(ResponseCodes.InternalServerEerror))
	if !strings.Contains(res.Body.String(), `"code":"`+wantCode+`"`) {
		t.Fatalf("expected the internal server error body, got %q", res.Body.String())
	}
}

// TestDefaultEncodeFailureDoesNotKeepTheFailedContentType checks that the
// default writer labels its 500 error body with the error body's own
// Content-Type, not with the one set for the body that failed to encode.
func TestDefaultEncodeFailureDoesNotKeepTheFailedContentType(t *testing.T) {
	previousPrintError := PrintError
	PrintError = func(...interface{}) {}
	defer func() { PrintError = previousPrintError }()
	res := httptest.NewRecorder()

	ResponseWriter{}.Write(&core.ResponseFormatted{
		Body:           math.NaN(),
		HTTPStatusCode: http.StatusOK,
		Headers:        map[string]string{"content-type": "text/csv", "X-Trace": "t"},
	}, res)

	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := res.Header().Get("X-Trace"); got != "t" {
		t.Fatalf("X-Trace = %q, want the other headers kept", got)
	}
}
