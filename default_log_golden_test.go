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
// running this test on v0.11.0 (b917197), so the default instance stays
// byte-identical to the latest v0 release.
func TestDefaultInstanceLogLinesMatchV0(t *testing.T) {
	tests := []struct {
		name string
		body string
		// next is the handler behind ProcessRequest; nil answers Success200.
		next http.HandlerFunc
		// parseForm parses the form before ProcessRequest runs.
		parseForm bool
		// logRequestBody sets LogRequestBody for the test.
		logRequestBody bool
		want           []string
	}{
		{
			name: "success",
			body: `{"title":"task"}`,
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2&access_token=[REDACTED]" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"],"Cookie":["[REDACTED]"]}`,
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
		{
			name: "error panic",
			next: func(http.ResponseWriter, *http.Request) { panic(goldenPanicError) },
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2&access_token=[REDACTED]" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"],"Cookie":["[REDACTED]"]}`,
				`PrintError: boom`,
			},
		},
		{
			name: "non-error panic",
			next: func(http.ResponseWriter, *http.Request) { panic("boom") },
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2&access_token=[REDACTED]" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"],"Cookie":["[REDACTED]"]}`,
				`PrintError: Not response: boom`,
			},
		},
		{
			name:           "logged body",
			body:           `{"title":"task","password":"s3cr3t"}`,
			logRequestBody: true,
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2&access_token=[REDACTED]" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"],"Cookie":["[REDACTED]"]} body="{\"password\":\"[REDACTED]\",\"title\":\"task\"}"`,
				`Print: RESPONSE event_id="<id>" status=200 status_text="OK" duration_ms=<ms>`,
			},
		},
		{
			name:      "parsed form",
			parseForm: true,
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2&access_token=[REDACTED]" form="access_token=[REDACTED]&page=2" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"],"Cookie":["[REDACTED]"]}`,
				`Print: RESPONSE event_id="<id>" status=200 status_text="OK" duration_ms=<ms>`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := captureDefaultLogLines(t)
			previousLogRequestBody := LogRequestBody
			LogRequestBody = tt.logRequestBody
			t.Cleanup(func() { LogRequestBody = previousLogRequestBody })
			next := tt.next
			if next == nil {
				next = func(w http.ResponseWriter, r *http.Request) {
					Success200().Write(w, r)
				}
			}
			handler := ProcessRequest(next)
			if tt.parseForm {
				handler = parseFormFirst(t, handler)
			}
			req := httptest.NewRequest(http.MethodPost, "/tasks?page=2&access_token=s3cr3t", bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer s3cr3t")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Cookie", "sid=s3cr3t")

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if fmt.Sprintf("%q", *lines) != fmt.Sprintf("%q", tt.want) {
				t.Fatalf("log lines =\n%s\nwant\n%s", joinLines(*lines), joinLines(tt.want))
			}
		})
	}
}

var goldenPanicError = goldenError("boom")

// goldenError is an error type that exists on v0.11.0 too, so this file
// compiles there unchanged.
type goldenError string

func (e goldenError) Error() string { return string(e) }

// parseFormFirst parses the request form before next runs.
func parseFormFirst(t *testing.T, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		next(w, r)
	}
}

func joinLines(lines []string) string {
	var out bytes.Buffer
	for _, line := range lines {
		out.WriteString(line + "\n")
	}
	return out.String()
}
