package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

var (
	eventIDPattern    = regexp.MustCompile(`event_id="[^"]*"`)
	durationMsPattern = regexp.MustCompile(`duration_ms=\d+`)
)

// captureDefaultLogLines replaces Print and PrintError for one test and
// returns the lines they would have written, with the event ID and the
// duration normalized so that runs can be compared.
func captureDefaultLogLines(t *testing.T) *[]string {
	t.Helper()
	previousPrint, previousPrintError := Print, PrintError
	lines := []string{}
	normalize := func(line string) string {
		line = eventIDPattern.ReplaceAllString(line, `event_id="<id>"`)
		return durationMsPattern.ReplaceAllString(line, "duration_ms=<ms>")
	}
	Print = func(format string, args ...interface{}) {
		lines = append(lines, "Print: "+normalize(fmt.Sprintf(format, args...)))
	}
	PrintError = func(args ...interface{}) {
		lines = append(lines, "PrintError: "+normalize(fmt.Sprint(args...)))
	}
	t.Cleanup(func() {
		Print, PrintError = previousPrint, previousPrintError
	})
	return &lines
}

// TestDefaultInstanceLogLinesMatchV0 pins the lines the default instance
// writes through Print and PrintError. The wanted lines were captured by
// running this test on v0.10.0 (ad03d5f).
func TestDefaultInstanceLogLinesMatchV0(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "success",
			body: `{"title":"task"}`,
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"]}`,
				`Print: RESPONSE event_id="<id>" status=200 status_text="OK" duration_ms=<ms>`,
			},
		},
		{
			name: "bad json",
			body: `{"title":`,
			want: []string{
				`PrintError: unexpected EOF`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := captureDefaultLogLines(t)
			handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
				Success200().Write(w, r)
			})
			req := httptest.NewRequest(http.MethodPost, "/tasks?page=2", bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer s3cr3t")
			req.Header.Set("Content-Type", "application/json")

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if fmt.Sprintf("%q", *lines) != fmt.Sprintf("%q", tt.want) {
				t.Fatalf("log lines =\n%s\nwant\n%s", joinLines(*lines), joinLines(tt.want))
			}
		})
	}
}

func joinLines(lines []string) string {
	var out bytes.Buffer
	for _, line := range lines {
		out.WriteString(line + "\n")
	}
	return out.String()
}
