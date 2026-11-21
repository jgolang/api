package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// jsonBodyOfSize returns a valid JSON object of exactly size bytes (size >= 8).
func jsonBodyOfSize(size int) string {
	return `{"a":"` + strings.Repeat("x", size-8) + `"}`
}

// reachHandler answers 200 and records that it ran.
func reachHandler(reached *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		Success200().Write(w, r)
	}
}

// envelopeHeader decodes the header of an envelope response body.
func envelopeHeader(t *testing.T, body []byte) (title, code string) {
	t.Helper()
	var envelope struct {
		Header struct {
			Title string `json:"title"`
			Code  string `json:"code"`
		} `json:"header"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return envelope.Header.Title, envelope.Header.Code
}

func TestWithMaxBodyBytesResolvesTheLimit(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want int64
	}{
		{name: "no option", want: defaultMaxBodyBytes},
		{name: "positive", opts: []Option{WithMaxBodyBytes(16)}, want: 16},
		{name: "zero ignored", opts: []Option{WithMaxBodyBytes(0)}, want: defaultMaxBodyBytes},
		{name: "negative ignored", opts: []Option{WithMaxBodyBytes(-1)}, want: defaultMaxBodyBytes},
		{name: "later call wins", opts: []Option{WithMaxBodyBytes(8), WithMaxBodyBytes(32)}, want: 32},
		{name: "later zero keeps earlier", opts: []Option{WithMaxBodyBytes(8), WithMaxBodyBytes(0)}, want: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.opts...)

			if a.maxBodyBytes != tt.want {
				t.Fatalf("maxBodyBytes = %d, want %d", a.maxBodyBytes, tt.want)
			}
		})
	}
}

func TestDefaultMaxBodyBytesIsTenMiB(t *testing.T) {
	if defaultMaxBodyBytes != 10<<20 {
		t.Fatalf("defaultMaxBodyBytes = %d, want %d", defaultMaxBodyBytes, 10<<20)
	}
}

func TestProcessRequestBoundsTheBody(t *testing.T) {
	tests := []struct {
		name        string
		a           *API
		bodySize    int
		wantStatus  int
		wantReached bool
	}{
		{name: "under option limit", a: New(WithMaxBodyBytes(16)), bodySize: 10, wantStatus: http.StatusOK, wantReached: true},
		{name: "exactly at option limit", a: New(WithMaxBodyBytes(16)), bodySize: 16, wantStatus: http.StatusOK, wantReached: true},
		{name: "over option limit", a: New(WithMaxBodyBytes(16)), bodySize: 17, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "default limit fits", a: New(), bodySize: defaultMaxBodyBytes, wantStatus: http.StatusOK, wantReached: true},
		{name: "over default limit", a: New(), bodySize: defaultMaxBodyBytes + 1, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "later call wins", a: New(WithMaxBodyBytes(8), WithMaxBodyBytes(32)), bodySize: 20, wantStatus: http.StatusOK, wantReached: true},
		{name: "later zero keeps earlier", a: New(WithMaxBodyBytes(8), WithMaxBodyBytes(0)), bodySize: 10, wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool

			rec := serve(tt.a.Handler(ProcessRequest(reachHandler(&reached))), http.MethodPost, jsonBodyOfSize(tt.bodySize), nil)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if reached != tt.wantReached {
				t.Fatalf("handler reached = %v, want %v", reached, tt.wantReached)
			}
		})
	}
}

func TestBodyOverLimitAnswersTooLargeEnvelope(t *testing.T) {
	var reached bool
	a := New(WithMaxBodyBytes(16))

	rec := serve(a.Handler(ProcessRequest(reachHandler(&reached))), http.MethodPost, jsonBodyOfSize(17), nil)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	title, code := envelopeHeader(t, rec.Body.Bytes())
	if title != "Request content too large" || code != "E002" {
		t.Fatalf("title, code = %q, %q; want %q, %q", title, code, "Request content too large", "E002")
	}
}

func TestBodyOverLimitIsLoggedAsRejected(t *testing.T) {
	var logs bytes.Buffer
	var reached bool
	a := New(WithMaxBodyBytes(16), WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))

	serve(a.Handler(ProcessRequest(reachHandler(&reached))), http.MethodPost, jsonBodyOfSize(17), nil)

	if !strings.Contains(logs.String(), `msg="request body rejected"`) {
		t.Fatalf("logs = %s, want a request body rejected record", logs.String())
	}
}

func TestRequestBodyOverLimitAnswersTooLarge(t *testing.T) {
	var reached bool
	a := New(WithMaxBodyBytes(16))

	rec := serve(a.Handler(RequestBody(reachHandler(&reached))), http.MethodPost, jsonBodyOfSize(17), nil)

	if rec.Code != http.StatusRequestEntityTooLarge || reached {
		t.Fatalf("status = %d, reached = %v; want 413 without the handler", rec.Code, reached)
	}
	title, code := envelopeHeader(t, rec.Body.Bytes())
	if title != "Request content too large" || code != "E002" {
		t.Fatalf("title, code = %q, %q; want %q, %q", title, code, "Request content too large", "E002")
	}
}

func TestUnmarshalBodyOverLimitAnswersTooLarge(t *testing.T) {
	a := New(WithMaxBodyBytes(16))
	handler := func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		if response := UnmarshalBody(&payload, r); response != nil {
			response.Write(w, r)
			return
		}
		Success200().Write(w, r)
	}

	rec := serve(a.Handler(http.HandlerFunc(handler)), http.MethodPost, jsonBodyOfSize(17), nil)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	title, code := envelopeHeader(t, rec.Body.Bytes())
	if title != "Request content too large" || code != "E002" {
		t.Fatalf("title, code = %q, %q; want %q, %q", title, code, "Request content too large", "E002")
	}
}

func TestProblemModeBodyOverLimit(t *testing.T) {
	var reached bool
	a := problemInstance(WithMaxBodyBytes(16))

	rec := serve(a.Handler(ProcessRequest(reachHandler(&reached))), http.MethodPost, jsonBodyOfSize(17), nil)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != problemContentType {
		t.Fatalf("Content-Type = %q, want %q", got, problemContentType)
	}
	members := decodeMembers(t, rec.Body.Bytes())
	if members["title"] != "Request Entity Too Large" || members["code"] != "E002" || members["status"] != 413.0 {
		t.Fatalf("members = %v, want title Request Entity Too Large, code E002, status 413", members)
	}
}

func TestDefaultInstanceHasNoBodyLimit(t *testing.T) {
	var reached bool

	rec := serve(ProcessRequest(reachHandler(&reached)), http.MethodPost, jsonBodyOfSize(defaultMaxBodyBytes+1), nil)

	if rec.Code != http.StatusOK || !reached {
		t.Fatalf("status = %d, reached = %v; want 200 with the handler", rec.Code, reached)
	}
}

func TestDefaultInstanceKeeps400ForAServiceLimit(t *testing.T) {
	var reached bool
	handler := http.MaxBytesHandler(ProcessRequest(reachHandler(&reached)), 16)

	rec := serve(handler, http.MethodPost, jsonBodyOfSize(17), nil)

	if rec.Code != http.StatusBadRequest || reached {
		t.Fatalf("status = %d, reached = %v; want 400 without the handler", rec.Code, reached)
	}
	title, code := envelopeHeader(t, rec.Body.Bytes())
	if title != "Invalid request content" || code != "e002" {
		t.Fatalf("title, code = %q, %q; want %q, %q", title, code, "Invalid request content", "e002")
	}
}

func TestHandlerDoesNotWrapABodylessRequest(t *testing.T) {
	var body any
	handler := New().Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = r.Body
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tasks", nil))

	if body != http.NoBody {
		t.Fatalf("body = %T, want http.NoBody", body)
	}
}
