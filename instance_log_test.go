package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jgolang/api/core"
)

// logRecord is one decoded slog JSON record.
type logRecord map[string]any

// newRecordingLogger returns a logger that writes JSON records into the
// returned buffer.
func newRecordingLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func decodeRecords(t *testing.T, buf *bytes.Buffer) []logRecord {
	t.Helper()
	var records []logRecord
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record logRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decoding record %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

// recordNamed returns the only record with message msg.
func recordNamed(t *testing.T, records []logRecord, msg string) logRecord {
	t.Helper()
	var found []logRecord
	for _, record := range records {
		if record["msg"] == msg {
			found = append(found, record)
		}
	}
	if len(found) != 1 {
		t.Fatalf("records named %q = %d, want 1 (%v)", msg, len(found), records)
	}
	return found[0]
}

// failOnPackageLoggers makes the test fail if Print, PrintError or Fatal is
// called.
func failOnPackageLoggers(t *testing.T) {
	t.Helper()
	previousPrint, previousPrintError, previousFatal := Print, PrintError, Fatal
	Print = func(format string, args ...interface{}) { t.Errorf("Print called: "+format, args...) }
	PrintError = func(args ...interface{}) { t.Errorf("PrintError called: %v", args) }
	Fatal = func(args ...interface{}) { t.Errorf("Fatal called: %v", args) }
	t.Cleanup(func() {
		Print, PrintError, Fatal = previousPrint, previousPrintError, previousFatal
	})
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	Success200().Write(w, r)
}

func TestInstanceLogsRequestAndResponseRecords(t *testing.T) {
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger))
	req := httptest.NewRequest(http.MethodPost, "/tasks?page=2", strings.NewReader(`{"title":"task"}`))
	req.Header.Set("Authorization", "Bearer s3cr3t")

	a.Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	records := decodeRecords(t, buf)
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2 (%v)", len(records), records)
	}
	request, response := records[0], records[1]
	if request["msg"] != "request" || response["msg"] != "response" {
		t.Fatalf("messages = %v, %v, want request, response", request["msg"], response["msg"])
	}
	for _, record := range records {
		if record["level"] != "INFO" {
			t.Errorf("level = %v, want INFO", record["level"])
		}
	}
	for _, key := range []string{"event_id", "method", "uri", "headers"} {
		if _, ok := request[key]; !ok {
			t.Errorf("request record has no %q: %v", key, request)
		}
	}
	for _, key := range []string{"event_id", "method", "uri", "status", "status_text", "duration_ms"} {
		if _, ok := response[key]; !ok {
			t.Errorf("response record has no %q: %v", key, response)
		}
	}
	if request["event_id"] == "" || request["event_id"] != response["event_id"] {
		t.Errorf("event_id = %v and %v, want the same non-empty ID", request["event_id"], response["event_id"])
	}
	if response["status"] != float64(http.StatusOK) || response["status_text"] != "OK" {
		t.Errorf("status = %v %v, want 200 OK", response["status"], response["status_text"])
	}
	if strings.Contains(buf.String(), "s3cr3t") || !strings.Contains(buf.String(), redactedLogValue) {
		t.Errorf("authorization not redacted: %s", buf)
	}
}

func TestInstanceRedactsAddedSensitiveKeys(t *testing.T) {
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger), WithSensitiveLogKeys("PIN"))
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Pin", "1234")
	req.Header.Set("Authorization", "Bearer s3cr3t")

	a.Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	headers := recordNamed(t, decodeRecords(t, buf), "request")["headers"].(map[string]any)
	for _, key := range []string{"Pin", "Authorization"} {
		if got := headers[key].([]any)[0]; got != redactedLogValue {
			t.Errorf("header %s = %v, want %s", key, got, redactedLogValue)
		}
	}
}

func TestInstanceRedactsQueryCookieAndSession(t *testing.T) {
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger))
	req := httptest.NewRequest(http.MethodGet, "/login?access_token=Q&page=2", nil)
	req.Header.Set("Cookie", "sid=abc")
	req.Header.Set("X-Session-Id", "s1")

	a.Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	records := decodeRecords(t, buf)
	for _, msg := range []string{"request", "response"} {
		if got := recordNamed(t, records, msg)["uri"]; got != "/login?access_token=[REDACTED]&page=2" {
			t.Errorf("%s uri = %v, want the token masked", msg, got)
		}
	}
	for _, secret := range []string{"sid=abc", `"s1"`, "=Q"} {
		if strings.Contains(buf.String(), secret) {
			t.Errorf("records contain %s: %s", secret, buf)
		}
	}
}

// TestInstanceURIMaskingAgreesWithTheDefaultInstance checks that an instance
// masks the logged uri exactly as the default instance does, since both share
// one code path, and pins the masked uri.
func TestInstanceURIMaskingAgreesWithTheDefaultInstance(t *testing.T) {
	tests := []struct{ name, uri, want string }{
		{name: "no query", uri: "/tasks", want: "/tasks"},
		{name: "empty query", uri: "/tasks?", want: "/tasks?"},
		{name: "sensitive value", uri: "/login?password=p&next=%2F", want: "/login?password=[REDACTED]&next=%2F"},
		{name: "at sign in the query", uri: "/login?email=a@b.com&token=T", want: "/login?email=a@b.com&token=[REDACTED]"},
		{name: "at sign in an absolute-form query", uri: "http://host/login?email=a@b.com&token=T", want: "http://[REDACTED]@b.com&token=[REDACTED]"},
		{name: "semicolon in a sensitive value", uri: "/login?token=abc;def&page=2", want: "/login?token=[REDACTED]&page=2"},
		{name: "semicolon separator", uri: "/tasks?page=2;token=T&q=x", want: "/tasks?page=2;token=[REDACTED]&q=x"},
		{name: "escaped key", uri: "/tasks?access%5Ftoken=T", want: "/tasks?access%5Ftoken=[REDACTED]"},
		{name: "key without value", uri: "/tasks?token&page=2", want: "/tasks?token&page=2"},
		{name: "empty value", uri: "/tasks?token=", want: "/tasks?token=[REDACTED]"},
		{name: "key that cannot be unescaped", uri: "/tasks?bad%zz=V&page=2", want: "/tasks?bad%zz=[REDACTED]&page=2"},
		{name: "user information", uri: "http://user:pass@example.com/tasks?token=T", want: "http://[REDACTED]@example.com/tasks?token=[REDACTED]"},
		{name: "user without password", uri: "https://user@example.com", want: "https://[REDACTED]@example.com"},
		{name: "url in a non-sensitive value", uri: "/redirect?to=http://a@b", want: "/redirect?to=http://a@b"},
		{name: "slash in the password", uri: "http://u:S4c/x@host/p", want: "http://[REDACTED]@host/p"},
		{name: "question mark in the password", uri: "http://u:S4d?x@host/p", want: "http://[REDACTED]@host/p"},
		{name: "network-path", uri: "//u:p@host/p", want: "//[REDACTED]@host/p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultBuf := captureDefaultLogger(t)
			logger, buf := newRecordingLogger()
			instance := New(WithLogger(logger))

			ProcessRequest(okHandler).ServeHTTP(httptest.NewRecorder(), newRawURIRequest(tt.uri))
			instance.Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), newRawURIRequest(tt.uri))

			if got := recordNamed(t, decodeRecords(t, defaultBuf), "request")["uri"]; got != tt.want {
				t.Fatalf("default instance uri = %v, want %q", got, tt.want)
			}
			if got := recordNamed(t, decodeRecords(t, buf), "request")["uri"]; got != tt.want {
				t.Fatalf("instance uri = %v, want %q", got, tt.want)
			}
		})
	}
}

// TestHeaderMaskingAgreesWithTheDefaultInstance checks that the default
// instance and an instance mask URL-valued header values as they mask uri,
// and keep the other values.
func TestHeaderMaskingAgreesWithTheDefaultInstance(t *testing.T) {
	tests := []struct {
		name   string
		header string
		values []string
		want   []string
		secret string
	}{
		{name: "referer with token", header: "Referer", values: []string{"https://app.example/cb?token=referer-secret&page=2"}, want: []string{"https://app.example/cb?token=[REDACTED]&page=2"}, secret: "referer-secret"},
		{name: "origin with user info", header: "Origin", values: []string{"https://u:origin-secret@app.example"}, want: []string{"https://[REDACTED]@app.example"}, secret: "origin-secret"},
		{name: "origin-form", header: "X-Original-Url", values: []string{"/login?password=original-secret"}, want: []string{"/login?password=[REDACTED]"}, secret: "original-secret"},
		{name: "network-path", header: "X-Forwarded-Uri", values: []string{"//u:forwarded-secret@host/p?code=1"}, want: []string{"//[REDACTED]@host/p?code=1"}, secret: "forwarded-secret"},
		{name: "link", header: "Link", values: []string{`<https://api.example/items?page=2&api_key=link-secret>; rel="next"`}, want: []string{"<https://api.example/items?page=2&api_key=[REDACTED]"}, secret: "link-secret"},
		{name: "query-only value", header: "X-Search", values: []string{"q?token=search-secret"}, want: []string{"q?token=[REDACTED]"}, secret: "search-secret"},
		{name: "several values", header: "Referer", values: []string{"https://a.example/?token=first-secret", "https://b.example/?token=first-secret"}, want: []string{"https://a.example/?token=[REDACTED]", "https://b.example/?token=[REDACTED]"}, secret: "first-secret"},
		{name: "link list", header: "Link", values: []string{`<https://api.example/items?page=2>; rel="next", <https://u:link-pass@api.example/items?api_key=list-secret>; rel="last"`}, want: []string{`<https://[REDACTED]@api.example/items?api_key=[REDACTED]`}, secret: "list-secret"},
		{name: "comma in a query value", header: "Referer", values: []string{"https://app.example/?ids=1,2&token=comma-secret"}, want: []string{"https://app.example/?ids=1,2&token=[REDACTED]"}, secret: "comma-secret"},
		{name: "url in a query value", header: "Referer", values: []string{"https://app.example/?next=/cb?token=nested-secret"}, want: []string{"https://app.example/?next=/cb?token=[REDACTED]"}, secret: "nested-secret"},
		{name: "user agent", header: "User-Agent", values: []string{"Mozilla/5.0 (X11; Linux) Firefox/120.0"}, want: []string{"Mozilla/5.0 (X11; Linux) Firefox/120.0"}},
		{name: "accept", header: "Accept", values: []string{"text/html;q=0.9"}, want: []string{"text/html;q=0.9"}},
		{name: "from", header: "From", values: []string{"ops@example.com"}, want: []string{"ops@example.com"}},
		{name: "sensitive name", header: "Authorization", values: []string{"Bearer x"}, want: []string{redactedLogValue}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultBuf := captureDefaultLogger(t)
			logger, buf := newRecordingLogger()
			instance := New(WithLogger(logger))
			newRequest := func() *http.Request {
				req := newRawURIRequest("/tasks")
				req.Header[tt.header] = tt.values
				return req
			}

			ProcessRequest(okHandler).ServeHTTP(httptest.NewRecorder(), newRequest())
			instance.Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), newRequest())

			for name, records := range map[string]*bytes.Buffer{"default instance": defaultBuf, "instance": buf} {
				headers := recordNamed(t, decodeRecords(t, records), "request")["headers"].(map[string]any)
				var got []string
				for _, value := range headers[tt.header].([]any) {
					got = append(got, value.(string))
				}
				if !slices.Equal(got, tt.want) {
					t.Errorf("%s %s = %q, want %q", name, tt.header, got, tt.want)
				}
				if tt.secret != "" && strings.Contains(records.String(), tt.secret) {
					t.Errorf("%s records contain %q: %s", name, tt.secret, records)
				}
			}
		})
	}
}

// newRawURIRequest returns a GET request whose RequestURI is uri as sent,
// which httptest.NewRequest would parse and rebuild.
func newRawURIRequest(uri string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RequestURI = uri
	return req
}

// TestInstanceRedactsFormParsedBeforeProcessRequest checks that a form
// parsed before ProcessRequest is never logged: bodies are not logged, and
// query values appear only, masked, in uri.
func TestInstanceRedactsFormParsedBeforeProcessRequest(t *testing.T) {
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger))
	parseForm := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parsing form: %v", err)
			}
			next.ServeHTTP(w, r)
		})
	}
	req := httptest.NewRequest(http.MethodPost, "/tasks?token=T&page=2", strings.NewReader("note=postedvalue&secret=postedsecret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	a.Handler(parseForm(ProcessRequest(okHandler))).ServeHTTP(httptest.NewRecorder(), req)

	records := decodeRecords(t, buf)
	request := recordNamed(t, records, "request")
	if _, ok := request["form"]; ok {
		t.Fatalf("request record has a form attribute: %v", request)
	}
	if request["uri"] != "/tasks?token=[REDACTED]&page=2" {
		t.Fatalf("uri = %v, want the token masked", request["uri"])
	}
	for _, value := range []string{"postedvalue", "postedsecret", "=T"} {
		if strings.Contains(buf.String(), value) {
			t.Fatalf("records contain %q: %s", value, buf)
		}
	}
}

func TestWithSensitiveLogKeysOnlyAddsKeys(t *testing.T) {
	a := New(WithSensitiveLogKeys("PIN", "", "  "), WithSensitiveLogKeys(" Otp "))

	want := append(instanceSensitiveLogKeys(nil), "pin", "otp")
	if !slices.Equal(a.sensitiveLogKeys, want) {
		t.Fatalf("keys = %q, want %q", a.sensitiveLogKeys, want)
	}
	for _, key := range defaultSensitiveLogKeys() {
		if !slices.Contains(a.sensitiveLogKeys, key) {
			t.Errorf("default key %q missing", key)
		}
	}
}

// captureProcessOutput redirects os.Stdout, os.Stderr and the standard
// logger for one test and returns a function that reports what was written.
func captureProcessOutput(t *testing.T) func() string {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previousStdout, previousStderr, previousLogOutput := os.Stdout, os.Stderr, log.Writer()
	var logOutput bytes.Buffer
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	log.SetOutput(&logOutput)
	restore := func() {
		os.Stdout, os.Stderr = previousStdout, previousStderr
		log.SetOutput(previousLogOutput)
	}
	t.Cleanup(restore)
	return func() string {
		restore()
		stdoutWriter.Close()
		stderrWriter.Close()
		stdout, _ := io.ReadAll(stdoutReader)
		stderr, _ := io.ReadAll(stderrReader)
		return string(stdout) + string(stderr) + logOutput.String()
	}
}

func TestInstanceWithoutLoggerIsSilent(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		body    string
	}{
		{name: "success", handler: ProcessRequest(okHandler), body: `{"title":"task"}`},
		{name: "bad json", handler: ProcessRequest(okHandler), body: `{"title":`},
		{name: "panic", handler: ProcessRequest(func(http.ResponseWriter, *http.Request) { panic("boom") })},
		{name: "nan content", handler: ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
			SuccessWithContent(math.NaN()).Write(w, r)
		})},
		{name: "header helper failure", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, response := GetHeaderValueInt("X-Count", r); response != nil {
				response.Write(w, r)
			}
		})},
		{name: "request body failure", handler: RequestBody(okHandler), body: `{"title":`},
		{name: "query helper failure", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, response := GetQueryParamValueInt("page", r); response != nil {
				response.Write(w, r)
			}
		})},
		{name: "route var helper failure", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, response := GetRouteVarValueInt("id", r); response != nil {
				response.Write(w, r)
			}
		})},
		{name: "unmarshal body failure", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload struct{}
			if response := UnmarshalBody(&payload, r); response != nil {
				response.Write(w, r)
			}
		}), body: `{"title":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			output := captureProcessOutput(t)
			req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(tt.body))

			New().Handler(tt.handler).ServeHTTP(httptest.NewRecorder(), req)

			if got := output(); got != "" {
				t.Fatalf("instance wrote %q", got)
			}
		})
	}
}

func TestInstanceAnswers500WhenResponseCannotBeEncoded(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{name: "no writer option"},
		{name: "nil writer", opts: []Option{WithResponseWriter(nil)}},
		{name: "ResponseWriter value", opts: []Option{WithResponseWriter(ResponseWriter{})}},
		{name: "ResponseWriter pointer", opts: []Option{WithResponseWriter(&ResponseWriter{})}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			logger, buf := newRecordingLogger()
			a := New(append(tt.opts, WithLogger(logger))...)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SuccessWithContent(math.NaN()).Write(w, r)
			})

			rec := serve(a.Handler(handler), http.MethodGet, "", nil)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
			failure := Error500()
			instanceFormatter := options{responseMode: ResponseModeEnvelope}.responseFormatter()
			want := instanceFormatter.Format(core.ResponseData{
				Title:          failure.Title,
				Message:        failure.Message,
				ResponseCode:   failure.ResponseCode,
				HTTPStatusCode: failure.HTTPStatusCode,
				ResponseType:   ErrorType,
			})
			wantBody, err := json.Marshal(want.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != string(wantBody) {
				t.Fatalf("body = %s, want %s", got, wantBody)
			}
			record := recordNamed(t, decodeRecords(t, buf), "response cannot be encoded")
			if record["level"] != "ERROR" || record["error"] == nil || record["error"] == "" {
				t.Fatalf("record = %v, want an error record with the error", record)
			}
		})
	}
}

// nanFormatter formats every response into a body that cannot be encoded.
type nanFormatter struct{}

func (nanFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{Body: math.NaN(), HTTPStatusCode: data.HTTPStatusCode, Headers: map[string]string{"X-Trace": "t"}}
}

func TestInstanceAnswersBare500WhenTheErrorBodyCannotBeEncoded(t *testing.T) {
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger), WithResponseFormatter(nanFormatter{}))

	rec := serve(a.Handler(http.HandlerFunc(okHandler)), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || rec.Body.Len() != 0 {
		t.Fatalf("response = %d %q, want a bare 500", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Trace") != "t" {
		t.Fatalf("headers = %v, want the response headers kept", rec.Header())
	}
	if records := decodeRecords(t, buf); len(records) != 2 {
		t.Fatalf("records = %v, want both encode failures", records)
	}
}

func TestInstanceIgnoresBodyNotAllowedOnInformationalStatus(t *testing.T) {
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger))
	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		Warning{}.Write(w, r)
	})

	serve(a.Handler(handler), http.MethodGet, "", nil)

	for _, record := range decodeRecords(t, buf) {
		if record["level"] != "INFO" {
			t.Fatalf("unexpected record %v", record)
		}
	}
}

func TestInstanceLogsRecoveredPanic(t *testing.T) {
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger))
	handler := ProcessRequest(func(http.ResponseWriter, *http.Request) { panic("boom") })

	rec := serve(a.Handler(handler), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	records := decodeRecords(t, buf)
	record := recordNamed(t, records, "panic recovered")
	if record["level"] != "ERROR" || record["panic"] != redactedLogValue || record["panic_type"] != "string" {
		t.Fatalf("record = %v, want an error record with panic=%s and panic_type=string", record, redactedLogValue)
	}
	if record["event_id"] != recordNamed(t, records, "request")["event_id"] {
		t.Fatalf("event_id = %v, want the request's", record["event_id"])
	}
	keys := slices.Sorted(maps.Keys(record))
	if want := []string{"event_id", "level", "msg", "panic", "panic_type", "time"}; !slices.Equal(keys, want) {
		t.Fatalf("panic record keys = %q, want %q", keys, want)
	}
	if strings.Contains(buf.String(), "Not response") || strings.Contains(buf.String(), "&{") {
		t.Fatalf("records dump the request: %s", buf)
	}
}

// TestInstanceLogsPanicValueOnlyForRuntimeErrors checks that a recovered
// panic logs its value only when the runtime built it: any other value may
// carry request data, so it is masked and only its type is logged.
func TestInstanceLogsPanicValueOnlyForRuntimeErrors(t *testing.T) {
	tests := []struct {
		name      string
		panicWith func()
		wantPanic string
		wantType  string
		secret    string
	}{
		{
			name:      "string",
			panicWith: func() { panic("boom") },
			wantPanic: redactedLogValue,
			wantType:  "string",
			secret:    "boom",
		},
		{
			name:      "error with a secret",
			panicWith: func() { panic(fmt.Errorf("bad token %q", "S3")) },
			wantPanic: redactedLogValue,
			wantType:  "*errors.errorString",
			secret:    "S3",
		},
		{
			name: "nil map write",
			panicWith: func() {
				var counts map[string]int
				counts["tasks"]++
			},
			wantPanic: "assignment to entry in nil map",
			wantType:  "runtime.plainError",
		},
		{
			name: "index out of range",
			panicWith: func() {
				items, index := []int{1}, 3
				_ = items[index]
			},
			wantPanic: "runtime error: index out of range [3] with length 1",
			wantType:  "runtime.boundsError",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newRecordingLogger()
			handler := ProcessRequest(func(http.ResponseWriter, *http.Request) { tt.panicWith() })

			rec := serve(New(WithLogger(logger)).Handler(handler), http.MethodGet, "", nil)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
			record := recordNamed(t, decodeRecords(t, buf), "panic recovered")
			if record["panic"] != tt.wantPanic || record["panic_type"] != tt.wantType {
				t.Fatalf("panic = %v, panic_type = %v, want %q, %q", record["panic"], record["panic_type"], tt.wantPanic, tt.wantType)
			}
			if tt.secret != "" && strings.Contains(buf.String(), tt.secret) {
				t.Fatalf("records contain the panic value %q: %s", tt.secret, buf)
			}
		})
	}
}

func TestInstanceLogsRejectedBody(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{name: "ProcessRequest", handler: ProcessRequest(okHandler)},
		{name: "RequestBody", handler: RequestBody(okHandler)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			logger, buf := newRecordingLogger()
			silent := serve(New().Handler(tt.handler), http.MethodPost, `{"title":`, nil)

			rec := serve(New(WithLogger(logger)).Handler(tt.handler), http.MethodPost, `{"title":`, nil)

			if rec.Code != http.StatusBadRequest || rec.Body.String() != silent.Body.String() {
				t.Fatalf("response = %d %s, want 400 %s", rec.Code, rec.Body, silent.Body)
			}
			records := decodeRecords(t, buf)
			if len(records) != 1 || records[0]["level"] != "ERROR" || records[0]["error"] == nil {
				t.Fatalf("records = %v, want one error record with the error", records)
			}
		})
	}
}

func TestInstanceLogsHelperFailure(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		body    string
		headers map[string]string
		read    func(r *http.Request) Response
		msg     string
		key     string
	}{
		{
			name:    "header",
			target:  "/tasks",
			headers: map[string]string{"X-Count": "many"},
			read: func(r *http.Request) Response {
				_, response := GetHeaderValueInt("X-Count", r)
				return response
			},
			msg: "header value not obtained",
			key: "X-Count",
		},
		{
			name:   "query param",
			target: "/tasks?page=many",
			read: func(r *http.Request) Response {
				_, response := GetQueryParamValueInt("page", r)
				return response
			},
			msg: "query param not obtained",
			key: "page",
		},
		{
			name:   "route var",
			target: "/tasks",
			read: func(r *http.Request) Response {
				_, response := GetRouteVarValueInt("id", r)
				return response
			},
			msg: "route var not obtained",
			key: "id",
		},
		{
			name:   "unmarshal body",
			target: "/tasks",
			body:   `{"title":`,
			read: func(r *http.Request) Response {
				var payload struct{}
				return UnmarshalBody(&payload, r)
			},
			msg: "request body not unmarshaled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			logger, buf := newRecordingLogger()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response := tt.read(r); response != nil {
					response.Write(w, r)
				}
			})
			req := httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader(tt.body))
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			New(WithLogger(logger)).Handler(handler).ServeHTTP(httptest.NewRecorder(), req)

			records := decodeRecords(t, buf)
			if len(records) != 1 || records[0]["msg"] != tt.msg || records[0]["level"] != "ERROR" || records[0]["error"] == nil {
				t.Fatalf("records = %v, want one error record %q with the error", records, tt.msg)
			}
			if tt.key != "" && records[0]["key"] != tt.key {
				t.Fatalf("key = %v, want %q", records[0]["key"], tt.key)
			}
		})
	}
}

func TestInstanceHelperErrorsHideSensitiveValues(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		headers map[string]string
		read    func(r *http.Request) Response
		key     string
	}{
		{
			name:    "header",
			target:  "/tasks",
			headers: map[string]string{"X-Api-Key": "supersecret"},
			read: func(r *http.Request) Response {
				_, response := GetHeaderValueInt("X-Api-Key", r)
				return response
			},
			key: "X-Api-Key",
		},
		{
			name:   "query param",
			target: "/tasks?access_token=supersecret",
			read: func(r *http.Request) Response {
				_, response := GetQueryParamValueInt64("access_token", r)
				return response
			},
			key: "access_token",
		},
		{
			name:   "string query param",
			target: "/tasks",
			read: func(r *http.Request) Response {
				_, response := GetQueryParamValueString("password", r)
				return response
			},
			key: "password",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newRecordingLogger()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response := tt.read(r); response != nil {
					response.Write(w, r)
				}
			})

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			New(WithLogger(logger)).Handler(handler).ServeHTTP(httptest.NewRecorder(), req)

			if strings.Contains(buf.String(), "supersecret") {
				t.Fatalf("records contain the value: %s", buf)
			}
			records := decodeRecords(t, buf)
			if len(records) != 1 || records[0]["key"] != tt.key || records[0]["error"] == nil || records[0]["error"] == "" {
				t.Fatalf("records = %v, want one record with key %q and an error", records, tt.key)
			}
		})
	}
}

func TestLogSafeHelperError(t *testing.T) {
	_, numErr := strconv.Atoi("supersecret")
	_, rangeErr := strconv.ParseInt("99999999999999999999", 10, 64)
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "syntax", err: numErr, want: "invalid syntax"},
		{name: "range", err: rangeErr, want: "value out of range"},
		{name: "other", err: fmt.Errorf("The password key header has not been obtained"), want: "invalid value"},
	}
	for _, tt := range tests {
		if got := sensitiveValueError(tt.err); got != tt.want {
			t.Errorf("%s: sensitiveValueError = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestInstancesLogOnlyToTheirOwnLogger(t *testing.T) {
	firstLogger, firstBuf := newRecordingLogger()
	secondLogger, secondBuf := newRecordingLogger()
	first := New(WithLogger(firstLogger)).Handler(ProcessRequest(okHandler))
	second := New(WithLogger(secondLogger), WithSensitiveLogKeys("pin")).Handler(ProcessRequest(okHandler))

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/first", nil)
			req.Header.Set("Pin", "1111")
			first.ServeHTTP(httptest.NewRecorder(), req)
		})
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/second", nil)
			req.Header.Set("Pin", "2222")
			second.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	for _, tt := range []struct {
		name, own, other, ownPin string
		buf                      *bytes.Buffer
	}{
		{name: "first", own: "/first", other: "/second", ownPin: "1111", buf: firstBuf},
		{name: "second", own: "/second", other: "/first", ownPin: redactedLogValue, buf: secondBuf},
	} {
		records := decodeRecords(t, tt.buf)
		if len(records) != 40 {
			t.Errorf("%s records = %d, want 40", tt.name, len(records))
		}
		for _, record := range records {
			if record["uri"] != tt.own {
				t.Fatalf("%s logger got %v", tt.name, record)
			}
			if headers, ok := record["headers"].(map[string]any); ok && headers["Pin"].([]any)[0] != tt.ownPin {
				t.Fatalf("%s Pin = %v, want %s", tt.name, headers["Pin"], tt.ownPin)
			}
		}
	}
}

func TestInstanceDefaultSensitiveLogKeys(t *testing.T) {
	want := []string{"authorization", "token", "password", "secret", "credential", "apikey", "api-key", "key", "cookie", "session"}

	if got := New().sensitiveLogKeys; !slices.Equal(got, want) {
		t.Fatalf("instance keys = %q, want %q", got, want)
	}
}

func TestInstanceIgnoresPackageSensitiveLogKeys(t *testing.T) {
	previous := SensitiveLogKeys
	SensitiveLogKeys = nil
	t.Cleanup(func() { SensitiveLogKeys = previous })
	logger, buf := newRecordingLogger()
	req := httptest.NewRequest(http.MethodGet, "/tasks?token=T", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")

	New(WithLogger(logger)).Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	if strings.Contains(buf.String(), "s3cr3t") || strings.Contains(buf.String(), "token=T") {
		t.Fatalf("records not masked with SensitiveLogKeys = nil: %s", buf)
	}
}

func TestWithSensitiveLogKeysRepeatedKey(t *testing.T) {
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger), WithSensitiveLogKeys("pin", "PIN"), WithSensitiveLogKeys("pin"))
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Pin", "1234")

	a.Handler(ProcessRequest(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	if strings.Contains(buf.String(), "1234") {
		t.Fatalf("Pin not masked: %s", buf)
	}
	if !slices.Equal(a.sensitiveLogKeys[:10], instanceSensitiveLogKeys(nil)) {
		t.Fatalf("keys = %q, want the defaults first", a.sensitiveLogKeys)
	}
}

func TestInstanceNeverLogsBodiesWhateverThePackageFlags(t *testing.T) {
	previousRequest, previousResponse, previousFull := LogRequestBody, LogResponseBody, PrintFullEvent
	LogRequestBody, LogResponseBody, PrintFullEvent = true, true, true
	t.Cleanup(func() {
		LogRequestBody, LogResponseBody, PrintFullEvent = previousRequest, previousResponse, previousFull
	})
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		SuccessWithContent(map[string]string{"answer": "responsevalue"}).Write(w, r)
	})

	serve(New(WithLogger(logger)).Handler(handler), http.MethodPost, `{"title":"requestvalue"}`, nil)

	for _, value := range []string{"requestvalue", "responsevalue"} {
		if strings.Contains(buf.String(), value) {
			t.Fatalf("records contain body value %q: %s", value, buf)
		}
	}
}

func TestInstanceProcessRequestCopiesResponseHeaders(t *testing.T) {
	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "kept")
		Success201().Write(w, r)
	})

	rec := serve(New().Handler(handler), http.MethodGet, "", nil)

	if rec.Code != http.StatusCreated || rec.Header().Get("X-Custom") != "kept" {
		t.Fatalf("response = %d %v, want 201 with X-Custom", rec.Code, rec.Header())
	}
}

// headerFormatter formats like the instance's built-in formatter and adds
// headers to every response; a nil error body makes it return nil for errors.
type headerFormatter struct {
	headers      map[string]string
	errorHeaders map[string]string
	nilErrorBody bool
}

func (f headerFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	if f.nilErrorBody && data.ResponseType == ErrorType {
		return nil
	}
	formatted := options{responseMode: ResponseModeEnvelope}.responseFormatter().Format(data)
	formatted.Headers = f.headers
	if data.ResponseType == ErrorType {
		formatted.Headers = f.errorHeaders
	}
	return formatted
}

func TestInstanceAnswersBare500WhenTheFormatterReturnsNoErrorBody(t *testing.T) {
	a := New(WithResponseFormatter(headerFormatter{headers: map[string]string{"X-Trace": "t"}, nilErrorBody: true}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SuccessWithContent(math.NaN()).Write(w, r)
	})

	rec := serve(a.Handler(handler), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || rec.Body.Len() != 0 || rec.Header().Get("X-Trace") != "t" {
		t.Fatalf("response = %d %q %v, want a bare 500 with the headers", rec.Code, rec.Body, rec.Header())
	}
}

func TestInstanceKeepsResponseHeadersOnEncodeFailure(t *testing.T) {
	a := New(WithResponseFormatter(headerFormatter{headers: map[string]string{"X-Trace": "t"}}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SuccessWithContent(math.NaN()).Write(w, r)
	})

	rec := serve(a.Handler(handler), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || rec.Body.Len() == 0 || rec.Header().Get("X-Trace") != "t" {
		t.Fatalf("response = %d %q %v, want a 500 body with the headers", rec.Code, rec.Body, rec.Header())
	}
}

func TestInstanceWriterKeepsFormattedContentType(t *testing.T) {
	formatter := headerFormatter{headers: map[string]string{"content-type": "application/problem+json"}}
	tests := []struct {
		name   string
		writer core.APIResponseWriter
		want   string
	}{
		{name: "instance", writer: options{}.responseWriter(formatter), want: "application/problem+json"},
		// Story 5.6: the default writer keeps a Content-Type set in Headers too.
		{name: "default", writer: ResponseWriter{}, want: "application/problem+json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			tt.writer.Write(formatter.Format(core.ResponseData{HTTPStatusCode: http.StatusOK}), rec)

			if got := rec.Header().Get("Content-Type"); got != tt.want {
				t.Fatalf("Content-Type = %q, want %q", got, tt.want)
			}
		})
	}
}

// nilFormatter formats every response into nil.
type nilFormatter struct{}

func (nilFormatter) Format(core.ResponseData) *core.ResponseFormatted { return nil }

func TestInstanceAnswers500WhenTheFormattedResponseIsNil(t *testing.T) {
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger), WithResponseFormatter(nilFormatter{}))

	rec := serve(a.Handler(http.HandlerFunc(okHandler)), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || rec.Body.Len() != 0 {
		t.Fatalf("response = %d %q, want a bare 500", rec.Code, rec.Body)
	}
	record := recordNamed(t, decodeRecords(t, buf), "response cannot be written")
	if record["level"] != "ERROR" || record["error"] != "nil formatted response" {
		t.Fatalf("record = %v, want an error record for the nil response", record)
	}
}

func TestDefaultWriterAnswers500WhenTheFormattedResponseIsNil(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	buf := captureDefaultLogger(t)
	failOnPackageLoggers(t)
	rec := httptest.NewRecorder()

	ResponseWriter{}.Write(nil, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if title, code := envelopeHeader(t, rec.Body.Bytes()); title != InternalServerTitle || code != string(internalServerErrorCode()) {
		t.Fatalf("title, code = %q, %q, want the Error500 body", title, code)
	}
	record := recordNamed(t, decodeRecords(t, buf), "response cannot be written")
	if record["error"] != "nil formatted response" {
		t.Fatalf("record = %v, want the nil response error", record)
	}
}

// embeddedWriter is a custom writer that only embeds ResponseWriter.
type embeddedWriter struct{ ResponseWriter }

// TestInstanceEmbeddedWriterLogsThroughSlogDefault checks that a type that
// embeds ResponseWriter keeps the default instance's behavior: it logs
// through slog.Default and builds the 500 with the registered (envelope)
// formatter, not the instance's (problem details) one.
func TestInstanceEmbeddedWriterLogsThroughSlogDefault(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	defaultBuf := captureDefaultLogger(t)
	failOnPackageLoggers(t)
	logger, instanceBuf := newRecordingLogger()
	a := New(WithLogger(logger), WithResponseMode(ResponseModeProblem), WithResponseWriter(embeddedWriter{}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SuccessWithContent(math.NaN()).Write(w, r)
	})

	rec := serve(a.Handler(handler), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want the registered formatter's application/json", got)
	}
	if _, code := envelopeHeader(t, rec.Body.Bytes()); code == "" {
		t.Fatalf("body = %s, want the registered formatter's envelope with a code", rec.Body)
	}
	recordNamed(t, decodeRecords(t, defaultBuf), "response cannot be encoded")
	if instanceBuf.Len() != 0 {
		t.Fatalf("instance logger got records: %s", instanceBuf)
	}
}

// TestInstanceWriterContentTypeComesFromTheFormattedHeaders checks that a
// Content-Type the handler put on w.Header() never labels the JSON body:
// only the formatted headers decide it.
func TestInstanceWriterContentTypeComesFromTheFormattedHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "no formatted Content-Type", want: "application/json"},
		{name: "formatted Content-Type", headers: map[string]string{"content-type": "application/problem+json"}, want: "application/problem+json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := options{}.responseWriter(headerFormatter{})
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", "text/html")

			writer.Write(&core.ResponseFormatted{Body: map[string]string{"a": "<b>"}, HTTPStatusCode: http.StatusOK, Headers: tt.headers}, rec)

			if got := rec.Header().Get("Content-Type"); got != tt.want {
				t.Fatalf("Content-Type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstanceEncodeFailureUsesTheErrorBodyContentType(t *testing.T) {
	tests := []struct {
		name      string
		formatter headerFormatter
		want      string
	}{
		{name: "error body without Content-Type", formatter: headerFormatter{headers: map[string]string{"Content-Type": "text/html", "X-Trace": "t"}}, want: "application/json"},
		{name: "error body with Content-Type", formatter: headerFormatter{headers: map[string]string{"Content-Type": "text/html", "X-Trace": "t"}, errorHeaders: map[string]string{"Content-Type": "application/problem+json"}}, want: "application/problem+json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := options{}.responseWriter(tt.formatter)
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", "text/html")

			writer.Write(&core.ResponseFormatted{Body: math.NaN(), HTTPStatusCode: http.StatusOK, Headers: tt.formatter.headers}, rec)

			if rec.Code != http.StatusInternalServerError || rec.Header().Get("X-Trace") != "t" {
				t.Fatalf("response = %d %v, want 500 with the other headers kept", rec.Code, rec.Header())
			}
			if got := rec.Header().Get("Content-Type"); got != tt.want {
				t.Fatalf("Content-Type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstanceBare500DropsTheFailedContentType(t *testing.T) {
	writer := options{}.responseWriter(headerFormatter{nilErrorBody: true})
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "text/html")

	writer.Write(&core.ResponseFormatted{Body: math.NaN(), HTTPStatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html", "X-Trace": "t"}}, rec)

	if rec.Code != http.StatusInternalServerError || rec.Header().Get("X-Trace") != "t" || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("response = %d %v, want a bare 500 with X-Trace and no Content-Type", rec.Code, rec.Header())
	}
}

// getValueHelpers calls every Get*Value* helper with a name that has no
// value, so each one fails.
var getValueHelpers = []struct {
	name string
	msg  string
	call func(r *http.Request) Response
}{
	{"GetHeaderValueString", "header value not obtained", func(r *http.Request) Response { _, resp := GetHeaderValueString("X-Missing", r); return resp }},
	{"GetHeaderValueInt", "header value not obtained", func(r *http.Request) Response { _, resp := GetHeaderValueInt("X-Missing", r); return resp }},
	{"GetHeaderValueInt64", "header value not obtained", func(r *http.Request) Response { _, resp := GetHeaderValueInt64("X-Missing", r); return resp }},
	{"GetHeaderValueFloat64", "header value not obtained", func(r *http.Request) Response { _, resp := GetHeaderValueFloat64("X-Missing", r); return resp }},
	{"GetHeaderValueBool", "header value not obtained", func(r *http.Request) Response { _, resp := GetHeaderValueBool("X-Missing", r); return resp }},
	{"GetRouteVarValueString", "route var not obtained", func(r *http.Request) Response { _, resp := GetRouteVarValueString("X-Missing", r); return resp }},
	{"GetRouteVarValueInt", "route var not obtained", func(r *http.Request) Response { _, resp := GetRouteVarValueInt("X-Missing", r); return resp }},
	{"GetRouteVarValueInt64", "route var not obtained", func(r *http.Request) Response { _, resp := GetRouteVarValueInt64("X-Missing", r); return resp }},
	{"GetRouteVarValueFloat64", "route var not obtained", func(r *http.Request) Response { _, resp := GetRouteVarValueFloat64("X-Missing", r); return resp }},
	{"GetRouteVarValueBool", "route var not obtained", func(r *http.Request) Response { _, resp := GetRouteVarValueBool("X-Missing", r); return resp }},
	{"GetQueryParamValueString", "query param not obtained", func(r *http.Request) Response { _, resp := GetQueryParamValueString("X-Missing", r); return resp }},
	{"GetQueryParamValueInt", "query param not obtained", func(r *http.Request) Response { _, resp := GetQueryParamValueInt("X-Missing", r); return resp }},
	{"GetQueryParamValueInt64", "query param not obtained", func(r *http.Request) Response { _, resp := GetQueryParamValueInt64("X-Missing", r); return resp }},
	{"GetQueryParamValueFloat64", "query param not obtained", func(r *http.Request) Response { _, resp := GetQueryParamValueFloat64("X-Missing", r); return resp }},
	{"GetQueryParamValueBool", "query param not obtained", func(r *http.Request) Response { _, resp := GetQueryParamValueBool("X-Missing", r); return resp }},
}

func TestEveryGetValueHelperFailureIsSilentWithoutALogger(t *testing.T) {
	for _, tt := range getValueHelpers {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			output := captureProcessOutput(t)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response := tt.call(r); response != nil {
					response.Write(w, r)
				}
			})

			serve(New().Handler(handler), http.MethodGet, "", nil)

			if got := output(); got != "" {
				t.Fatalf("instance wrote %q", got)
			}
		})
	}
}

func TestEveryGetValueHelperFailureIsRecorded(t *testing.T) {
	for _, tt := range getValueHelpers {
		t.Run(tt.name, func(t *testing.T) {
			failOnPackageLoggers(t)
			logger, buf := newRecordingLogger()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response := tt.call(r); response == nil {
					t.Fatal("helper did not fail")
				}
			})

			serve(New(WithLogger(logger)).Handler(handler), http.MethodGet, "", nil)

			records := decodeRecords(t, buf)
			if len(records) != 1 || records[0]["msg"] != tt.msg || records[0]["key"] != "X-Missing" || records[0]["level"] != "ERROR" {
				t.Fatalf("records = %v, want one %q error record with key X-Missing", records, tt.msg)
			}
		})
	}
}

func TestInstanceEncodeFailureDropsTheFailedBodyHeaders(t *testing.T) {
	bodyHeaders := map[string]string{
		"Content-Encoding":    "gzip",
		"Content-Length":      "20",
		"Content-Disposition": "attachment",
		"content-range":       "bytes 0-19/20",
		"Content-Language":    "en",
		"Content-Location":    "/tasks/1",
		"ETag":                `"v1"`,
		"Last-Modified":       "Sat, 21 Nov 2026 00:00:00 GMT",
	}
	headers := maps.Clone(bodyHeaders)
	headers["X-Trace"] = "t"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Success{Content: math.NaN(), Headers: headers}.Write(w, r)
	})

	rec := serve(New().Handler(handler), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("response = %d %q, want 500 with a JSON body", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Trace") != "t" || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("headers = %v, want X-Trace kept and application/json", rec.Header())
	}
	for key := range bodyHeaders {
		if got := rec.Header().Get(key); got != "" {
			t.Errorf("header %s = %q, want it dropped", key, got)
		}
	}
}

func TestWithoutBodyHeaders(t *testing.T) {
	got := withoutBodyHeaders(map[string]string{"content-type": "text/html", "ETAG": "x", "X-Trace": "t"})

	if want := map[string]string{"X-Trace": "t"}; !maps.Equal(got, want) {
		t.Fatalf("withoutBodyHeaders = %v, want %v", got, want)
	}
}
