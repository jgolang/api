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
	// pointerPattern and hexIDPattern normalize the request dump of v0's
	// non-error panic line: addresses and generated event IDs change per run.
	pointerPattern = regexp.MustCompile(`0x[0-9a-f]+`)
	hexIDPattern   = regexp.MustCompile(`\b[0-9a-f]{32}\b`)
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
		line = durationMsPattern.ReplaceAllString(line, "duration_ms=<ms>")
		line = pointerPattern.ReplaceAllString(line, "0x<ptr>")
		return hexIDPattern.ReplaceAllString(line, "<id>")
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
		// next is the handler behind ProcessRequest; nil answers Success200.
		next http.HandlerFunc
		// parseForm parses the form before ProcessRequest runs.
		parseForm bool
		want      []string
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
		{
			name: "error panic",
			next: func(http.ResponseWriter, *http.Request) { panic(goldenPanicError) },
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"]}`,
				`PrintError: boom`,
			},
		},
		{
			name: "non-error panic",
			next: func(http.ResponseWriter, *http.Request) { panic("boom") },
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"]}`,
				`PrintError: Not response: &{POST /tasks?page=2 HTTP/1.1 1 1 map[Appbuildversion:[] Applanguage:[] Appname:[] Appversion:[] Authorization:[Bearer s3cr3t] Content-Type:[application/json] Devicebrand:[] Devicemodel:[] Deviceos:[] Devicetype:[] Eventid:[<id>] Ostimezone:[] Osversion:[] Securitytoken:[s3cr3t] Uuid:[]] {} <nil> 0 [] false example.com map[] map[] <nil> map[] 192.0.2.1:1234 /tasks?page=2 <nil> <nil> <nil>  0x<ptr> <nil> [] map[]}`,
			},
		},
		{
			name:      "parsed form",
			parseForm: true,
			want: []string{
				`Print: REQUEST event_id="<id>" method="POST" uri="/tasks?page=2" form="page=2" headers={"Authorization":["[REDACTED]"],"Content-Type":["application/json"]}`,
				`Print: RESPONSE event_id="<id>" status=200 status_text="OK" duration_ms=<ms>`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := captureDefaultLogLines(t)
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

var goldenPanicError = goldenError("boom")

// goldenError is an error type that exists on v0.10.0 too, so this file
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
