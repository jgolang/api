package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
