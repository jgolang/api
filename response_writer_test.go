package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jgolang/api/core"
)

func TestResponseWriterAnswers500WhenBodyCannotBeEncoded(t *testing.T) {
	buf := captureDefaultLogger(t)
	failOnPackageLoggers(t)

	req := httptest.NewRequest(http.MethodGet, "/ratio", nil)
	res := httptest.NewRecorder()
	SuccessWithContent(map[string]float64{"ratio": math.NaN()}).Write(res, req)

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
	wantCode := string(ResponseCodes.InternalServerEerror)
	if header["code"] != wantCode {
		t.Fatalf("expected code %q, got %#v", wantCode, body)
	}
	record := recordNamed(t, decodeRecords(t, buf), "response cannot be encoded")
	if !strings.Contains(fmt.Sprint(record["error"]), "NaN") {
		t.Fatalf("expected the encoding error to be logged, got %v", record)
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
// which must neither exit the process nor be logged as an error.
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
			// The server goroutine logs into buf; it is read after server.Close.
			buf := captureDefaultLogger(t)
			failOnPackageLoggers(t)
			server := httptest.NewServer(tt.handler)
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

			if res.StatusCode != http.StatusOK {
				t.Fatalf("expected the client to get status 200, got %d", res.StatusCode)
			}
			if !strings.Contains(string(body), `"type":"`+tt.wantType+`"`) {
				t.Fatalf("expected a %s body, got %q", tt.wantType, body)
			}
			for _, record := range decodeRecords(t, buf) {
				if record["level"] != "INFO" {
					t.Fatalf("expected only the request and response records, got %v", record)
				}
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
	captureDefaultLogger(t)
	RegisterNewAPIResponseFormatter(errorMarkerFormatter{})
	defer RegisterNewAPIResponseFormatter(ResponseFormatter{})

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
	captureDefaultLogger(t)

	res := httptest.NewRecorder()
	res.WriteHeader(http.StatusAccepted)
	SuccessWithContent(map[string]float64{"ratio": math.NaN()}).Write(res, httptest.NewRequest(http.MethodGet, "/ratio", nil))

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected the earlier status 202 to stay, got %d", res.Code)
	}
	wantCode := string(ResponseCodes.InternalServerEerror)
	if !strings.Contains(res.Body.String(), `"code":"`+wantCode+`"`) {
		t.Fatalf("expected the internal server error body, got %q", res.Body.String())
	}
}

// formatterFunc adapts a function to core.APIResponseFormatter.
type formatterFunc func(core.ResponseData) *core.ResponseFormatted

func (f formatterFunc) Format(data core.ResponseData) *core.ResponseFormatted { return f(data) }

// TestDefaultEncodeFailureHeaders checks the headers of the default writer's
// encode-failure 500: the failed body's headers are dropped, the error body's
// Content-Type applies, and the other headers are kept, as on an instance.
func TestDefaultEncodeFailureHeaders(t *testing.T) {
	failedHeaders := map[string]string{
		"content-type":     "text/csv",
		"Content-Encoding": "gzip",
		"Content-Length":   "12",
		"ETag":             `"v1"`,
		"X-Trace":          "t",
	}
	tests := []struct {
		name            string
		formatter       core.APIResponseFormatter
		wantStatus      int
		wantContentType string
		wantBody        bool
	}{
		{name: "default formatter", formatter: ResponseFormatter{}, wantStatus: 500, wantContentType: "application/json", wantBody: true},
		{
			name: "formatter sets the error body's Content-Type",
			formatter: formatterFunc(func(data core.ResponseData) *core.ResponseFormatted {
				formatted := ResponseFormatter{}.Format(data)
				formatted.Headers = map[string]string{"Content-Type": "application/problem+json"}
				return formatted
			}),
			wantStatus: 500, wantContentType: "application/problem+json", wantBody: true,
		},
		{
			name:       "formatter returns no error body",
			formatter:  formatterFunc(func(core.ResponseData) *core.ResponseFormatted { return nil }),
			wantStatus: 500,
		},
		{
			name: "error body cannot be encoded either",
			formatter: formatterFunc(func(data core.ResponseData) *core.ResponseFormatted {
				return &core.ResponseFormatted{Body: math.NaN(), HTTPStatusCode: data.HTTPStatusCode}
			}),
			wantStatus: 500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureDefaultLogger(t)
			RegisterNewAPIResponseFormatter(tt.formatter)
			t.Cleanup(func() { RegisterNewAPIResponseFormatter(ResponseFormatter{}) })
			res := httptest.NewRecorder()
			res.Header().Set("Content-Type", "text/html")

			ResponseWriter{}.Write(&core.ResponseFormatted{Body: math.NaN(), HTTPStatusCode: http.StatusOK, Headers: failedHeaders}, res)

			if res.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.Code, tt.wantStatus)
			}
			if got := res.Header().Get("Content-Type"); got != tt.wantContentType {
				t.Fatalf("Content-Type = %q, want %q", got, tt.wantContentType)
			}
			for _, key := range []string{"Content-Encoding", "Content-Length", "ETag"} {
				if got := res.Header().Get(key); got != "" {
					t.Fatalf("%s = %q, want it dropped", key, got)
				}
			}
			if got := res.Header().Get("X-Trace"); got != "t" {
				t.Fatalf("X-Trace = %q, want the other headers kept", got)
			}
			if gotBody := res.Body.Len() != 0; gotBody != tt.wantBody {
				t.Fatalf("body = %q, want a body: %v", res.Body.String(), tt.wantBody)
			}
		})
	}
}
