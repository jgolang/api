package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLogRequestDoesNotPrintBodyByDefault(t *testing.T) {
	output := captureLogOutput(t, func() {
		LogRequest(http.MethodPost, "/tasks", "event-id", "", nil, []byte(`{"token":"secret-token"}`))
	})

	if strings.Contains(output, "secret-token") {
		t.Fatalf("expected request body to be hidden by default, got %q", output)
	}
	if strings.Contains(output, "body=") {
		t.Fatalf("expected request body field to be omitted by default, got %q", output)
	}
}

func TestLogRequestRedactsSensitiveHeadersAndJSONBody(t *testing.T) {
	previousLogRequestBody := LogRequestBody
	LogRequestBody = true
	defer func() {
		LogRequestBody = previousLogRequestBody
	}()

	headers := http.Header{
		"Authorization": []string{"Bearer secret-token"},
		"X-Api-Key":     []string{"secret-api-key"},
		"Content-Type":  []string{"application/json"},
	}
	body := []byte(`{"header":{"token":"body-token"},"content":{"password":"secret-password","title":"task"}}`)

	output := captureLogOutput(t, func() {
		LogRequest(http.MethodPost, "/tasks", "event-id", "", headers, body)
	})

	for _, value := range []string{"Bearer secret-token", "secret-api-key", "body-token", "secret-password"} {
		if strings.Contains(output, value) {
			t.Fatalf("expected sensitive value %q to be redacted, got %q", value, output)
		}
	}
	if !strings.Contains(output, redactedLogValue) {
		t.Fatalf("expected redacted marker in output, got %q", output)
	}
	if !strings.Contains(output, "task") {
		t.Fatalf("expected non-sensitive body fields to remain, got %q", output)
	}
}

func TestFormatBodyForLogTruncatesSanitizedBody(t *testing.T) {
	previousMaxLoggedBodyBytes := MaxLoggedBodyBytes
	previousPrintFullEvent := PrintFullEvent
	MaxLoggedBodyBytes = 40
	PrintFullEvent = false
	defer func() {
		MaxLoggedBodyBytes = previousMaxLoggedBodyBytes
		PrintFullEvent = previousPrintFullEvent
	}()

	output := formatBodyForLog([]byte(`{"content":"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"}`))

	if len(output) >= len(`{"content":"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"}`) {
		t.Fatalf("expected body to be truncated, got %q", output)
	}
	if !strings.Contains(output, "SKIPPED") {
		t.Fatalf("expected skipped marker in truncated body, got %q", output)
	}
}

func TestLogResponseIncludesDurationAndHidesBodyByDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusCreated)
	rec.Write([]byte(`{"token":"response-token"}`))

	output := captureLogOutput(t, func() {
		LogResponseWithDuration("event-id", rec, 1500*time.Millisecond)
	})

	if strings.Contains(output, "response-token") {
		t.Fatalf("expected response body to be hidden by default, got %q", output)
	}
	if !strings.Contains(output, "duration_ms=1500") {
		t.Fatalf("expected response duration in output, got %q", output)
	}
	if !strings.Contains(output, "status=201") {
		t.Fatalf("expected response status in output, got %q", output)
	}
}

func captureLogOutput(t *testing.T, fn func()) string {
	t.Helper()

	previousPrint := Print
	var output string
	Print = func(format string, args ...interface{}) {
		output = fmt.Sprintf(format, args...)
	}
	defer func() {
		Print = previousPrint
	}()

	fn()

	return output
}

func TestLogRequestRedactsCookieAndSessionHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{name: "cookie", header: "Cookie", value: "sid=secret-cookie"},
		{name: "session header", header: "X-Session-Id", value: "secret-session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{tt.header: []string{tt.value}}

			output := captureLogOutput(t, func() {
				LogRequest(http.MethodGet, "/tasks", "event-id", "", headers, nil)
			})

			want := fmt.Sprintf(`%q:["[REDACTED]"]`, tt.header)
			if strings.Contains(output, tt.value) || !strings.Contains(output, want) {
				t.Fatalf("expected %s and no %q in output, got %q", want, tt.value, output)
			}
		})
	}
}

func TestLogRequestSummarizesFormBody(t *testing.T) {
	previousLogRequestBody := LogRequestBody
	LogRequestBody = true
	defer func() {
		LogRequestBody = previousLogRequestBody
	}()
	body := []byte("user=ana&password=secret-password")

	output := captureLogOutput(t, func() {
		LogRequest(http.MethodPost, "/login", "event-id", "", nil, body)
	})

	want := fmt.Sprintf(`body="[non-JSON body, %d bytes]"`, len(body))
	if strings.Contains(output, "secret-password") || !strings.Contains(output, want) {
		t.Fatalf("expected %s and no body values in output, got %q", want, output)
	}
}

func TestLogRequestPrintsNonJSONBodyWithPrintFullEvent(t *testing.T) {
	previousLogRequestBody := LogRequestBody
	previousPrintFullEvent := PrintFullEvent
	LogRequestBody = true
	PrintFullEvent = true
	defer func() {
		LogRequestBody = previousLogRequestBody
		PrintFullEvent = previousPrintFullEvent
	}()

	output := captureLogOutput(t, func() {
		LogRequest(http.MethodPost, "/notes", "event-id", "", nil, []byte("plain note"))
	})

	if !strings.Contains(output, `body="plain note"`) {
		t.Fatalf("expected the raw body with PrintFullEvent, got %q", output)
	}
}

func TestLogRequestKeepsJSONScalarBody(t *testing.T) {
	previousLogRequestBody := LogRequestBody
	LogRequestBody = true
	defer func() {
		LogRequestBody = previousLogRequestBody
	}()

	output := captureLogOutput(t, func() {
		LogRequest(http.MethodPost, "/count", "event-id", "", nil, []byte("42"))
	})

	if !strings.Contains(output, `body="42"`) {
		t.Fatalf("expected a JSON scalar body to be logged, got %q", output)
	}
}

func TestLogRequestRedactsSensitiveURIQueryValues(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "no query",
			uri:  "/tasks",
			want: "/tasks",
		},
		{
			name: "empty query",
			uri:  "/tasks?",
			want: "/tasks?",
		},
		{
			name: "sensitive and plain values",
			uri:  "/tasks?page=2&access_token=secret-token&sort=asc",
			want: "/tasks?page=2&access_token=[REDACTED]&sort=asc",
		},
		{
			name: "case-insensitive key and semicolon separator",
			uri:  "/tasks?q=go;Session=secret-session&API-KEY=secret-key",
			want: "/tasks?q=go;Session=[REDACTED]&API-KEY=[REDACTED]",
		},
		{
			name: "sensitive value with a raw semicolon is masked up to the next ampersand",
			uri:  "/tasks?token=abc;def&q=go",
			want: "/tasks?token=[REDACTED]&q=go",
		},
		{
			name: "sensitive value with a raw semicolon at the end",
			uri:  "/tasks?q=go;token=abc;def;page=2",
			want: "/tasks?q=go;token=[REDACTED]",
		},
		{
			name: "escaped sensitive key",
			uri:  "/tasks?pass%77ord=secret-password",
			want: "/tasks?pass%77ord=[REDACTED]",
		},
		{
			name: "key that cannot be unescaped",
			uri:  "/tasks?bad%zz=secret-value&q=go",
			want: "/tasks?bad%zz=[REDACTED]&q=go",
		},
		{
			name: "part without value",
			uri:  "/tasks?token&q=go",
			want: "/tasks?token&q=go",
		},
		{
			name: "absolute-form userinfo",
			uri:  "http://user:secret-pass@example.com/tasks?q=go",
			want: "http://[REDACTED]@example.com/tasks?q=go",
		},
		{
			name: "absolute-form with @ in the query",
			uri:  "http://example.com/cb?email=a@b.com&token=secret-token",
			want: "http://[REDACTED]@b.com&token=[REDACTED]",
		},
		{
			name: "sensitive name holding a raw semicolon",
			uri:  "/tasks?token;x=secret-token&q=go",
			want: "/tasks?token;x=[REDACTED]&q=go",
		},
		{
			name: "authority-form userinfo",
			uri:  "user:secret-pass@example.com:443",
			want: "[REDACTED]@example.com:443",
		},
		{
			name: "network-path userinfo",
			uri:  "//user:secret-pass@example.com/tasks",
			want: "//[REDACTED]@example.com/tasks",
		},
		{
			name: "absolute-form userinfo without a query",
			uri:  "http://user:secret-pass@[::1]:8080/tasks",
			want: "http://[REDACTED]@[::1]:8080/tasks",
		},
		{
			name: "absolute-form userinfo with a raw # in the password",
			uri:  "http://user:se#cret@example.com/tasks",
			want: "http://[REDACTED]@example.com/tasks",
		},
		{
			name: "raw ? in the password before a sensitive value holding the @",
			uri:  "http://user:secret?key=1@example.com/tasks",
			want: "http://[REDACTED]",
		},
		{
			name: "absolute-form userinfo with a raw ? in the password",
			uri:  "http://user:se?cret@example.com/tasks?token=secret-token",
			want: "http://[REDACTED]@example.com/tasks?token=[REDACTED]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureLogOutput(t, func() {
				LogRequest(http.MethodGet, tt.uri, "event-id", "", nil, nil)
			})

			want := fmt.Sprintf("uri=%q", tt.want)
			if !strings.Contains(output, want) {
				t.Fatalf("expected %s in output, got %q", want, output)
			}
		})
	}
}

func TestLogRequestRedactsSensitiveFormValues(t *testing.T) {
	output := captureLogOutput(t, func() {
		LogRequest(http.MethodPost, "/login", "event-id", "password=secret-password&user=ana", nil, nil)
	})

	if strings.Contains(output, "secret-password") {
		t.Fatalf("expected the form password to be redacted, got %q", output)
	}
	if !strings.Contains(output, `form="password=[REDACTED]&user=ana"`) {
		t.Fatalf("expected masked form in output, got %q", output)
	}
}

// TestProcessRequestLogRedactsCookieQueryAndBody checks the default
// ProcessRequest records: the cookie and query token are masked and the body
// is never logged, even with LogRequestBody set.
func TestProcessRequestLogRedactsCookieQueryAndBody(t *testing.T) {
	buf := captureDefaultLogger(t)
	previousLogRequestBody := LogRequestBody
	LogRequestBody = true
	t.Cleanup(func() { LogRequestBody = previousLogRequestBody })

	body := `{"password":"secret-password","title":"task"}`
	req := httptest.NewRequest(http.MethodPost, "/login?token=secret-token&page=1", strings.NewReader(body))
	req.Header.Set("Cookie", "sid=secret-cookie")
	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		Success200().Write(w, r)
	})

	handler.ServeHTTP(httptest.NewRecorder(), req)

	for _, value := range []string{"secret-password", "secret-token", "secret-cookie", "title"} {
		if strings.Contains(buf.String(), value) {
			t.Fatalf("expected %q to be absent from the log, got %s", value, buf)
		}
	}
	request := recordNamed(t, decodeRecords(t, buf), "request")
	if request["uri"] != "/login?token=[REDACTED]&page=1" {
		t.Fatalf("uri = %v, want the token masked", request["uri"])
	}
	if cookie := request["headers"].(map[string]any)["Cookie"].([]any)[0]; cookie != redactedLogValue {
		t.Fatalf("Cookie = %v, want %s", cookie, redactedLogValue)
	}
}
