package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
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

func TestRedactQuery(t *testing.T) {
	keys := instanceSensitiveLogKeys(nil)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "nothing sensitive", raw: "page=2&sort=a%20b", want: "page=2&sort=a%20b"},
		{name: "sensitive value", raw: "page=2&token=T&q=x", want: "page=2&token=[REDACTED]&q=x"},
		{name: "escaped key", raw: "access%5Ftoken=T", want: "access%5Ftoken=[REDACTED]"},
		{name: "key without value", raw: "token&page=2", want: "token&page=2"},
		{name: "empty value", raw: "token=", want: "token=[REDACTED]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactQuery(tt.raw, keys); got != tt.want {
				t.Fatalf("redactQuery(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestRedactURI(t *testing.T) {
	keys := instanceSensitiveLogKeys(nil)
	tests := []struct{ uri, want string }{
		{uri: "/tasks", want: "/tasks"},
		{uri: "/tasks?", want: "/tasks?"},
		{uri: "/login?password=p&next=%2F", want: "/login?password=[REDACTED]&next=%2F"},
	}
	for _, tt := range tests {
		if got := redactURI(tt.uri, keys); got != tt.want {
			t.Errorf("redactURI(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

// TestInstanceRedactsFormParsedBeforeProcessRequest masks the form that a
// middleware parsed before ProcessRequest.
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
	req := httptest.NewRequest(http.MethodGet, "/tasks?token=T&page=2", nil)

	a.Handler(parseForm(ProcessRequest(okHandler))).ServeHTTP(httptest.NewRecorder(), req)

	if got := recordNamed(t, decodeRecords(t, buf), "request")["form"]; got != "page=2&token=[REDACTED]" {
		t.Fatalf("form = %v, want the token masked", got)
	}
}

func TestWithSensitiveLogKeysOnlyAddsKeys(t *testing.T) {
	a := New(WithSensitiveLogKeys("PIN", "", "  "), WithSensitiveLogKeys(" Otp "))

	want := append(instanceSensitiveLogKeys(nil), "pin", "otp")
	if !slices.Equal(a.sensitiveLogKeys, want) {
		t.Fatalf("keys = %q, want %q", a.sensitiveLogKeys, want)
	}
	for _, key := range append(slices.Clone(SensitiveLogKeys), "cookie", "session") {
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
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger))
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
}

// nanFormatter formats every response into a body that cannot be encoded.
type nanFormatter struct{}

func (nanFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{Body: math.NaN(), HTTPStatusCode: data.HTTPStatusCode}
}

func TestInstanceAnswersBare500WhenTheErrorBodyCannotBeEncoded(t *testing.T) {
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	a := New(WithLogger(logger), WithResponseFormatter(nanFormatter{}))

	rec := serve(a.Handler(http.HandlerFunc(okHandler)), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || rec.Body.Len() != 0 {
		t.Fatalf("response = %d %q, want a bare 500", rec.Code, rec.Body)
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
	if record["level"] != "ERROR" || record["panic"] != "boom" {
		t.Fatalf("record = %v, want an error record with panic=boom", record)
	}
	if record["event_id"] != recordNamed(t, records, "request")["event_id"] {
		t.Fatalf("event_id = %v, want the request's", record["event_id"])
	}
	if strings.Contains(buf.String(), "Not response") || strings.Contains(buf.String(), "&{") {
		t.Fatalf("records dump the request: %s", buf)
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
	failOnPackageLoggers(t)
	logger, buf := newRecordingLogger()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, response := GetHeaderValueInt("X-Count", r); response != nil {
			response.Write(w, r)
		}
	})

	serve(New(WithLogger(logger)).Handler(handler), http.MethodGet, "", map[string]string{"X-Count": "many"})

	records := decodeRecords(t, buf)
	if len(records) != 1 || records[0]["level"] != "ERROR" || records[0]["error"] == nil {
		t.Fatalf("records = %v, want one error record with the error", records)
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

func TestDefaultInstanceCallsFatalOnEncodeFailure(t *testing.T) {
	previousFatal := Fatal
	var fatalCalls int
	Fatal = func(...interface{}) { fatalCalls++ }
	t.Cleanup(func() { Fatal = previousFatal })

	SuccessWithContent(math.NaN()).Write(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tasks", nil))

	if fatalCalls != 1 {
		t.Fatalf("Fatal calls = %d, want 1", fatalCalls)
	}
}
