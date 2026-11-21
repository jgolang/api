package api

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"log"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// captureDefaultLogger makes slog.Default write JSON records into the
// returned buffer for one test. It also restores the standard logger's
// output and flags, which slog.SetDefault redirects.
func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous, previousOutput, previousFlags := slog.Default(), log.Writer(), log.Flags()
	logger, buf := newRecordingLogger()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})
	return buf
}

// sameAsNewEventIDs matches what differs between two runs of one request:
// the event ID in bodies and the short ID in the trace suffix.
var sameAsNewEventIDs = regexp.MustCompile(`"event_id":"[0-9a-f]+"|\([0-9a-f]*-`)

// TestDefaultInstanceAnswersLikeNew checks that a v0 program without
// api.New answers exactly as the same handler under api.New().Handler,
// apart from event IDs.
func TestDefaultInstanceAnswersLikeNew(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	captureDefaultLogger(t)
	declared := codes.New("PLN-001", "plan not found").WithKind(codes.KindNotFound)
	tests := []struct {
		name    string
		handler http.Handler
		body    string
		headers map[string]string
	}{
		{name: "success", handler: ProcessRequest(okHandler), body: `{"title":"task"}`},
		{name: "declared code", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Error{ResponseCode: "PLN-001", Message: "plan not found"}.Write(w, r)
		})},
		{name: "write error", handler: ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, r, errors.NewC(declared, "no plan"))
		})},
		{name: "bearer token not echoed", handler: ProcessRequest(okHandler), headers: map[string]string{"Authorization": "Bearer s3cr3t"}},
		{name: "session header not copied", handler: RequestHeaderSession(okHandler), headers: map[string]string{SecurityTokenHeaderKey: "s3cr3t"}},
		{name: "session header missing", handler: RequestHeaderSession(okHandler)},
		{name: "bad json", handler: ProcessRequest(okHandler), body: `{"title":`},
		{name: "body too large", handler: ProcessRequest(okHandler), body: jsonBodyOfSize(defaultMaxBodyBytes + 1)},
		{name: "request body too large", handler: RequestBody(okHandler), body: jsonBodyOfSize(defaultMaxBodyBytes + 1)},
		{name: "unmarshal body too large", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if response := UnmarshalBody(&payload, r); response != nil {
				response.Write(w, r)
			}
		}), body: jsonBodyOfSize(defaultMaxBodyBytes + 1)},
		{name: "panic", handler: ProcessRequest(func(http.ResponseWriter, *http.Request) { panic("boom") })},
		{name: "nan content", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SuccessWithContent(math.NaN()).Write(w, r)
		})},
		{name: "custom token rejected", handler: CustomToken(okHandler), headers: map[string]string{"Authorization": "Bearer invalid"}},
		{name: "get value helper failure", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, response := GetHeaderValueInt("X-Count", r); response != nil {
				response.Write(w, r)
			}
		}), headers: map[string]string{"X-Count": "many"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withoutInstance := serve(tt.handler, http.MethodPost, tt.body, tt.headers)
			withNew := serve(New().Handler(tt.handler), http.MethodPost, tt.body, tt.headers)

			if withoutInstance.Code != withNew.Code {
				t.Errorf("status = %d without instance, %d with New()", withoutInstance.Code, withNew.Code)
			}
			if a, b := withoutInstance.Header(), withNew.Header(); !maps.EqualFunc(a, b, slices.Equal) {
				t.Errorf("headers = %v without instance, %v with New()", a, b)
			}
			a := sameAsNewEventIDs.ReplaceAllString(withoutInstance.Body.String(), "ID")
			if b := sameAsNewEventIDs.ReplaceAllString(withNew.Body.String(), "ID"); a != b {
				t.Errorf("body without instance: %s\nbody with New():        %s", a, b)
			}
		})
	}
}

// TestDefaultInstanceInEnvelopeRequestModeAnswersLikeNew checks that the
// envelope header.token read by ProcessRequest is not echoed, as on an
// instance in the same request mode.
func TestDefaultInstanceInEnvelopeRequestModeAnswersLikeNew(t *testing.T) {
	usePackageModes(t, RequestModeEnvelope, ResponseModeEnvelope)
	captureDefaultLogger(t)
	body := `{"header":{"token":"s3cr3t"},"content":{"title":"task"}}`
	handler := ProcessRequest(okHandler)

	withoutInstance := serve(handler, http.MethodPost, body, nil)
	withNew := serve(New(WithRequestMode(RequestModeEnvelope)).Handler(handler), http.MethodPost, body, nil)

	if withoutInstance.Code != withNew.Code {
		t.Errorf("status = %d without instance, %d with New()", withoutInstance.Code, withNew.Code)
	}
	if a, b := withoutInstance.Header(), withNew.Header(); !maps.EqualFunc(a, b, slices.Equal) {
		t.Errorf("headers = %v without instance, %v with New()", a, b)
	}
	a := sameAsNewEventIDs.ReplaceAllString(withoutInstance.Body.String(), "ID")
	if b := sameAsNewEventIDs.ReplaceAllString(withNew.Body.String(), "ID"); a != b {
		t.Errorf("body without instance: %s\nbody with New():        %s", a, b)
	}
	if strings.Contains(withoutInstance.Body.String(), "s3cr3t") {
		t.Errorf("body echoes the envelope token: %s", withoutInstance.Body)
	}
}

func TestDefaultInstanceSendsCodesAsDeclared(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModePlain)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(EventIDHeaderKey, "evtabc123")
	rec := httptest.NewRecorder()

	Error{ResponseCode: "PLN-001", Message: "plan not found"}.Write(rec, req)

	code, message := codeAndMessage(rec.Body.Bytes())
	if code != "PLN-001" || message != "plan not found (evtabc-PLN-001)" {
		t.Fatalf("code, message = %q, %q, want PLN-001 and the declared code in the suffix", code, message)
	}
}

func TestDefaultInstanceDoesNotEchoTheRequestToken(t *testing.T) {
	tests := []struct {
		mode ResponseMode
		want string
	}{
		{mode: ResponseModePlain, want: `"token":""`},
		{mode: ResponseModeEnvelope, want: ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			usePackageModes(t, RequestModePlain, tt.mode)

			rec := serve(http.HandlerFunc(okHandler), http.MethodGet, "", map[string]string{"Authorization": "Bearer s3cr3t"})

			if strings.Contains(rec.Body.String(), "s3cr3t") {
				t.Fatalf("body echoes the request token: %s", rec.Body)
			}
			if tt.want != "" && !strings.Contains(rec.Body.String(), tt.want) {
				t.Fatalf("body = %s, want %s", rec.Body, tt.want)
			}
			if tt.mode == ResponseModeEnvelope && strings.Contains(rec.Body.String(), `"token"`) {
				t.Fatalf("envelope has a token: %s", rec.Body)
			}
		})
	}
}

func TestDefaultRequestHeaderSessionDoesNotCopyTheToken(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "with session", token: "s3cr3t", wantStatus: http.StatusOK},
		{name: "without session", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(RequestHeaderSession(okHandler), http.MethodGet, "", map[string]string{SecurityTokenHeaderKey: tt.token})

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get(SecurityTokenHeaderKey); got != "" {
				t.Fatalf("%s response header = %q, want none", SecurityTokenHeaderKey, got)
			}
		})
	}
}

func TestDefaultInstanceBoundsBodiesAtTheLibraryReadPoints(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	buf := captureDefaultLogger(t)
	tests := []struct {
		name       string
		handler    http.Handler
		wantLogged string
	}{
		{name: "ProcessRequest", handler: ProcessRequest(okHandler), wantLogged: "request body rejected"},
		{name: "RequestBody", handler: RequestBody(okHandler), wantLogged: "request body rejected"},
		{name: "UnmarshalBody", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if response := UnmarshalBody(&payload, r); response != nil {
				response.Write(w, r)
			}
		}), wantLogged: "request body not unmarshaled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			rec := serve(tt.handler, http.MethodPost, jsonBodyOfSize(defaultMaxBodyBytes+1), nil)

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413", rec.Code)
			}
			if title, code := envelopeHeader(t, rec.Body.Bytes()); title != "Request content too large" || code != string(ResponseCodes.InvalidJSON) {
				t.Fatalf("title, code = %q, %q, want the too-large title and %s", title, code, ResponseCodes.InvalidJSON)
			}
			recordNamed(t, decodeRecords(t, buf), tt.wantLogged)
		})
	}
}

func TestDefaultProcessBodyIsBounded(t *testing.T) {
	tests := []struct {
		name string
		read func(r *http.Request) error
	}{
		{name: "ProcessBody", read: func(r *http.Request) error { _, err := ProcessBody(r); return err }},
		{name: "ProcessEncryptedBody", read: func(r *http.Request) error { _, err := ProcessEncryptedBody(r); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBodyOfSize(defaultMaxBodyBytes+1)))

			err := tt.read(req)

			var maxBytesErr *http.MaxBytesError
			if !stderrors.As(err, &maxBytesErr) {
				t.Fatalf("err = %v, want *http.MaxBytesError", err)
			}
		})
	}
}

func TestDefaultProcessBodyReadsABodyAtTheLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(jsonBodyOfSize(defaultMaxBodyBytes)))

	if _, err := ProcessBody(req); err != nil {
		t.Fatalf("ProcessBody = %v, want a body of exactly the limit accepted", err)
	}
}

func TestSetResponseModeProblemWritesProblemDetails(t *testing.T) {
	previous := CurrentResponseMode
	t.Cleanup(func() { CurrentResponseMode = previous })
	SetResponseMode(ResponseModeProblem)

	rec := serve(http.HandlerFunc(writeTaskNotFound), http.MethodGet, "", nil)

	if got := rec.Header().Get("Content-Type"); got != problemContentType {
		t.Fatalf("Content-Type = %q, want %s", got, problemContentType)
	}
	if members := decodeMembers(t, rec.Body.Bytes()); members["type"] != "about:blank" || members["status"] != float64(http.StatusNotFound) {
		t.Fatalf("members = %v, want a 404 problem", members)
	}
}

// TestDefaultInstanceLogsThroughSlogDefault pins, as golden records, what the
// default ProcessRequest writes: request and response through slog.Default, with
// sensitive headers and query values masked and no body or form even when
// the deprecated body-logging flags are on. Print and PrintError are never
// called.
func TestDefaultInstanceLogsThroughSlogDefault(t *testing.T) {
	failOnPackageLoggers(t)
	previousLogRequestBody, previousLogResponseBody := LogRequestBody, LogResponseBody
	LogRequestBody, LogResponseBody = true, true
	t.Cleanup(func() { LogRequestBody, LogResponseBody = previousLogRequestBody, previousLogResponseBody })
	tests := []struct {
		name      string
		body      string
		next      http.HandlerFunc
		parseForm bool
		want      []string
	}{
		{
			name: "success",
			body: `{"title":"task","password":"bodysecret"}`,
			want: []string{
				`{"event_id":"<id>","headers":{"Authorization":["[REDACTED]"],"Cookie":["[REDACTED]"]},"level":"INFO","method":"POST","msg":"request","uri":"/tasks?page=2&access_token=[REDACTED]"}`,
				`{"duration_ms":"<ms>","event_id":"<id>","level":"INFO","method":"POST","msg":"response","status":200,"status_text":"OK","uri":"/tasks?page=2&access_token=[REDACTED]"}`,
			},
		},
		{
			name: "parsed form",
			body: "note=formvalue&secret=formsecret",
			next: okHandler, parseForm: true,
			want: []string{
				`{"event_id":"<id>","headers":{"Authorization":["[REDACTED]"],"Content-Type":["application/x-www-form-urlencoded"],"Cookie":["[REDACTED]"]},"level":"INFO","method":"POST","msg":"request","uri":"/tasks?page=2&access_token=[REDACTED]"}`,
				`{"duration_ms":"<ms>","event_id":"<id>","level":"INFO","method":"POST","msg":"response","status":200,"status_text":"OK","uri":"/tasks?page=2&access_token=[REDACTED]"}`,
			},
		},
		{
			name: "bad json",
			body: `{"title":`,
			want: []string{`{"error":"unexpected EOF","level":"ERROR","msg":"request body rejected"}`},
		},
		{
			name: "panic",
			next: func(http.ResponseWriter, *http.Request) { panic("boom") },
			want: []string{
				`{"event_id":"<id>","headers":{"Authorization":["[REDACTED]"],"Cookie":["[REDACTED]"]},"level":"INFO","method":"POST","msg":"request","uri":"/tasks?page=2&access_token=[REDACTED]"}`,
				`{"event_id":"<id>","level":"ERROR","msg":"panic recovered","panic":"[REDACTED]","panic_type":"string"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureDefaultLogger(t)
			next := tt.next
			if next == nil {
				next = okHandler
			}
			handler := ProcessRequest(next)
			if tt.parseForm {
				handler = parseFormFirst(t, handler)
			}
			req := httptest.NewRequest(http.MethodPost, "/tasks?page=2&access_token=s3cr3t", strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer s3cr3t")
			req.Header.Set("Cookie", "sid=s3cr3t")
			if tt.parseForm {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}

			handler.ServeHTTP(httptest.NewRecorder(), req)

			records := decodeRecords(t, buf)
			var got []string
			for _, record := range records {
				got = append(got, goldenRecord(t, record))
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("records =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
			for _, secret := range []string{"s3cr3t", "bodysecret", "formvalue", "formsecret"} {
				if strings.Contains(buf.String(), secret) {
					t.Fatalf("records contain %s: %s", secret, buf)
				}
			}
			for _, record := range records {
				if record["msg"] == "request" && record["uri"] != "/tasks?page=2&access_token=[REDACTED]" {
					t.Fatalf("request uri = %v, want the token masked", record["uri"])
				}
			}
		})
	}
}

func TestDefaultGetValueHelperFailuresAreRecorded(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		headers   map[string]string
		read      func(r *http.Request) Response
		wantMsg   string
		wantKey   string
		wantError string
		secret    string
	}{
		{
			name:      "header",
			headers:   map[string]string{"X-Count": "many"},
			read:      func(r *http.Request) Response { _, response := GetHeaderValueInt("X-Count", r); return response },
			wantMsg:   "header value not obtained",
			wantKey:   "X-Count",
			wantError: `strconv.Atoi: parsing "many": invalid syntax`,
		},
		{
			name:      "sensitive header",
			headers:   map[string]string{"X-Api-Key": "supersecret"},
			read:      func(r *http.Request) Response { _, response := GetHeaderValueInt("X-Api-Key", r); return response },
			wantMsg:   "header value not obtained",
			wantKey:   "X-Api-Key",
			wantError: "invalid syntax",
			secret:    "supersecret",
		},
		{
			name:   "sensitive query param",
			target: "/?access_token=supersecret",
			read: func(r *http.Request) Response {
				_, response := GetQueryParamValueInt("access_token", r)
				return response
			},
			wantMsg:   "query param not obtained",
			wantKey:   "access_token",
			wantError: "invalid syntax",
			secret:    "supersecret",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureDefaultLogger(t)
			failOnPackageLoggers(t)
			target := tt.target
			if target == "" {
				target = "/"
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if response := tt.read(req); response == nil {
				t.Fatal("helper succeeded, want a failure")
			}

			records := decodeRecords(t, buf)
			if len(records) != 1 {
				t.Fatalf("records = %d, want 1 (%s)", len(records), buf)
			}
			record := records[0]
			if record["level"] != "ERROR" || record["msg"] != tt.wantMsg || record["key"] != tt.wantKey || record["error"] != tt.wantError {
				t.Fatalf("record = %v, want ERROR %q key %q error %q", record, tt.wantMsg, tt.wantKey, tt.wantError)
			}
			if tt.secret != "" && strings.Contains(buf.String(), tt.secret) {
				t.Fatalf("records contain the value: %s", buf)
			}
		})
	}
}

// goldenRecord returns record as JSON without its time, with the event ID
// and the duration normalized, so runs can be compared.
func goldenRecord(t *testing.T, record logRecord) string {
	t.Helper()
	delete(record, "time")
	if _, ok := record["event_id"]; ok {
		record["event_id"] = "<id>"
	}
	if _, ok := record["duration_ms"]; ok {
		record["duration_ms"] = "<ms>"
	}
	var golden bytes.Buffer
	encoder := json.NewEncoder(&golden)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(record); err != nil {
		t.Fatalf("encoding record %v: %v", record, err)
	}
	return strings.TrimSpace(golden.String())
}

func TestDefaultSensitiveLogKeysOnlyAdd(t *testing.T) {
	buf := captureDefaultLogger(t)
	previous := SensitiveLogKeys
	SensitiveLogKeys = []string{"pin"}
	t.Cleanup(func() { SensitiveLogKeys = previous })
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Pin", "1234")
	req.Header.Set("Authorization", "Bearer s3cr3t")

	ProcessRequest(okHandler).ServeHTTP(httptest.NewRecorder(), req)

	headers := recordNamed(t, decodeRecords(t, buf), "request")["headers"].(map[string]any)
	for _, key := range []string{"Pin", "Authorization"} {
		if got := headers[key].([]any)[0]; got != redactedLogValue {
			t.Errorf("header %s = %v, want %s", key, got, redactedLogValue)
		}
	}
}

func TestDefaultWriterAnswers500WhenResponseCannotBeEncoded(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	buf := captureDefaultLogger(t)
	failOnPackageLoggers(t)

	rec := serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SuccessWithContent(math.NaN()).Write(w, r)
	}), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if title, code := envelopeHeader(t, rec.Body.Bytes()); title != InternalServerTitle || code != string(internalServerErrorCode()) {
		t.Fatalf("title, code = %q, %q, want the Error500 body", title, code)
	}
	recordNamed(t, decodeRecords(t, buf), "response cannot be encoded")
}

func TestDefaultWriterAnswersBare500WhenTheFormatterReturnsNoBody(t *testing.T) {
	captureDefaultLogger(t)
	RegisterNewAPIResponseFormatter(nanFormatter{})
	t.Cleanup(func() { RegisterNewAPIResponseFormatter(ResponseFormatter{}) })

	rec := serve(http.HandlerFunc(okHandler), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError || rec.Body.Len() != 0 {
		t.Fatalf("status, body = %d, %q, want a bare 500", rec.Code, rec.Body)
	}
}

func TestNewWithoutLoggerStaysSilentUnderACapturingDefault(t *testing.T) {
	buf := captureDefaultLogger(t)

	serve(New().Handler(ProcessRequest(okHandler)), http.MethodPost, `{"title":`, nil)
	serve(New().Handler(ProcessRequest(okHandler)), http.MethodPost, `{"title":"task"}`, nil)

	if buf.Len() != 0 {
		t.Fatalf("New() without WithLogger logged: %s", buf)
	}
}

// parseFormFirst parses the request form before next runs.
func parseFormFirst(t *testing.T, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		next(w, r)
	}
}
