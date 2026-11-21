package api

import (
	stderrors "errors"
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

// countPackageErrorLogs replaces Print and PrintError with no-ops for one
// test and returns how many times PrintError was called. The default
// instance still logs through them until it moves to slog.
func countPackageErrorLogs(t *testing.T) *int {
	t.Helper()
	previousPrint, previousPrintError := Print, PrintError
	calls := 0
	Print = func(string, ...interface{}) {}
	PrintError = func(...interface{}) { calls++ }
	t.Cleanup(func() { Print, PrintError = previousPrint, previousPrintError })
	return &calls
}

// sameAsNewEventIDs matches what differs between two runs of one request:
// the event ID in bodies and the short ID in the trace suffix.
var sameAsNewEventIDs = regexp.MustCompile(`"event_id":"[0-9a-f]+"|\([0-9a-f]*-`)

// TestDefaultInstanceAnswersLikeNew checks that a v0 program without
// api.New answers exactly as the same handler under api.New().Handler,
// apart from event IDs.
func TestDefaultInstanceAnswersLikeNew(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	countPackageErrorLogs(t)
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
	countPackageErrorLogs(t)
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
	errorLogs := countPackageErrorLogs(t)
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{name: "ProcessRequest", handler: ProcessRequest(okHandler)},
		{name: "RequestBody", handler: RequestBody(okHandler)},
		{name: "UnmarshalBody", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if response := UnmarshalBody(&payload, r); response != nil {
				response.Write(w, r)
			}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*errorLogs = 0

			rec := serve(tt.handler, http.MethodPost, jsonBodyOfSize(defaultMaxBodyBytes+1), nil)

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413", rec.Code)
			}
			if title, code := envelopeHeader(t, rec.Body.Bytes()); title != "Request content too large" || code != string(ResponseCodes.InvalidJSON) {
				t.Fatalf("title, code = %q, %q, want the too-large title and %s", title, code, ResponseCodes.InvalidJSON)
			}
			if *errorLogs != 1 {
				t.Fatalf("PrintError called %d times, want 1", *errorLogs)
			}
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
