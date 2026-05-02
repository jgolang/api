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
