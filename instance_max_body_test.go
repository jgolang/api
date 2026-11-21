package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
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

func TestDefaultInstanceAcceptsABodyAtTheLimit(t *testing.T) {
	var reached bool

	rec := serve(ProcessRequest(reachHandler(&reached)), http.MethodPost, jsonBodyOfSize(defaultMaxBodyBytes), nil)

	if rec.Code != http.StatusOK || !reached {
		t.Fatalf("status = %d, reached = %v; want 200 with the handler", rec.Code, reached)
	}
}

func TestDefaultInstanceAnswers413ForAServiceLimit(t *testing.T) {
	var reached bool
	handler := http.MaxBytesHandler(ProcessRequest(reachHandler(&reached)), 16)

	rec := serve(handler, http.MethodPost, jsonBodyOfSize(17), nil)

	if rec.Code != http.StatusRequestEntityTooLarge || reached {
		t.Fatalf("status = %d, reached = %v; want 413 without the handler", rec.Code, reached)
	}
	title, code := envelopeHeader(t, rec.Body.Bytes())
	if title != "Request content too large" || code != "E002" {
		t.Fatalf("title, code = %q, %q; want %q, %q", title, code, "Request content too large", "E002")
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

// TestInstanceLimitAboveTheDefaultIsNotCapped checks that the default
// instance's 10 MiB bound never applies under an instance with a higher limit.
func TestInstanceLimitAboveTheDefaultIsNotCapped(t *testing.T) {
	unmarshal := func(reached *bool) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]string
			if response := UnmarshalBody(&payload, r); response != nil {
				response.Write(w, r)
				return
			}
			reachHandler(reached)(w, r)
		})
	}
	tests := []struct {
		name    string
		handler func(reached *bool) http.Handler
	}{
		{name: "ProcessRequest", handler: func(reached *bool) http.Handler { return ProcessRequest(reachHandler(reached)) }},
		{name: "RequestBody", handler: func(reached *bool) http.Handler { return RequestBody(reachHandler(reached)) }},
		{name: "UnmarshalBody", handler: unmarshal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool
			a := New(WithMaxBodyBytes(defaultMaxBodyBytes + 16))

			rec := serve(a.Handler(tt.handler(&reached)), http.MethodPost, jsonBodyOfSize(defaultMaxBodyBytes+1), nil)

			if rec.Code != http.StatusOK || !reached {
				t.Fatalf("status = %d, reached = %v; want 200 with the handler", rec.Code, reached)
			}
		})
	}
}

func TestNestedInstancesApplyTheInnermostLimit(t *testing.T) {
	small, large := New(WithMaxBodyBytes(16)), New(WithMaxBodyBytes(1024))
	tests := []struct {
		name        string
		outer       *API
		inner       *API
		bodySize    int
		wantStatus  int
		wantReached bool
	}{
		{name: "larger inner raises the outer limit", outer: small, inner: large, bodySize: 64, wantStatus: http.StatusOK, wantReached: true},
		{name: "smaller inner lowers the outer limit", outer: large, inner: small, bodySize: 64, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "over the inner limit", outer: small, inner: large, bodySize: 2048, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "under both", outer: small, inner: large, bodySize: 12, wantStatus: http.StatusOK, wantReached: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reached bool
			handler := tt.outer.Handler(tt.inner.Handler(ProcessRequest(reachHandler(&reached))))

			rec := serve(handler, http.MethodPost, jsonBodyOfSize(tt.bodySize), nil)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if reached != tt.wantReached {
				t.Fatalf("handler reached = %v, want %v", reached, tt.wantReached)
			}
		})
	}
}

func TestInnerInstanceCannotRaiseAServiceLimit(t *testing.T) {
	var reached bool
	inner := New(WithMaxBodyBytes(1024)).Handler(ProcessRequest(reachHandler(&reached)))
	handler := http.MaxBytesHandler(New(WithMaxBodyBytes(16)).Handler(inner), 32)

	rec := serve(handler, http.MethodPost, jsonBodyOfSize(64), nil)

	if rec.Code != http.StatusRequestEntityTooLarge || reached {
		t.Fatalf("status = %d, reached = %v; want 413 without the handler", rec.Code, reached)
	}
}

// TestRaisedLimitParsesALargeURLEncodedForm checks that r.Body stays the
// reader http.MaxBytesReader returns: net/http's form parser caps any other
// body at 10 MB, whatever the instance limit.
func TestRaisedLimitParsesALargeURLEncodedForm(t *testing.T) {
	const formSize = 11 << 20
	var parseErr error
	var fieldSize int
	inner := New(WithMaxBodyBytes(25 << 20)).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parseErr = r.ParseForm()
		fieldSize = len(r.PostForm.Get("a"))
	}))
	body := "a=" + strings.Repeat("x", formSize)
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{name: "single instance", handler: inner},
		{name: "nested in a 10 MiB instance", handler: New().Handler(inner)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			tt.handler.ServeHTTP(httptest.NewRecorder(), r)

			if parseErr != nil || fieldSize != formSize {
				t.Fatalf("ParseForm = %v, field size %d; want nil and %d", parseErr, fieldSize, formSize)
			}
		})
	}
}

// TestInnerInstanceRaisesTheLimitForAnUploadRoute serves an upload route
// under a 25 MiB instance inside a router-wide instance with the 10 MiB
// default: the route streams a 25 MiB multipart part, and a sibling route
// keeps the 10 MiB limit.
func TestInnerInstanceRaisesTheLimitForAnUploadRoute(t *testing.T) {
	const uploadLimit = 25 << 20
	const fileSize = uploadLimit - 64<<10 // about 25 MiB, leaving room for the multipart framing

	var received int64
	upload := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Errorf("MultipartReader: %v", err)
			return
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Errorf("NextPart: %v", err)
			return
		}
		received, err = io.Copy(io.Discard, part)
		if err != nil {
			t.Errorf("reading the part: %v", err)
			return
		}
		Success200().Write(w, r)
	})
	var siblingReached bool
	mux := http.NewServeMux()
	mux.Handle("POST /files", New(WithMaxBodyBytes(uploadLimit)).Handler(upload))
	mux.Handle("POST /tasks", ProcessRequest(reachHandler(&siblingReached)))
	server := serveStream(t, New().Handler(mux))

	t.Run("upload route accepts about 25 MiB", func(t *testing.T) {
		body, contentType := streamedMultipart(t, fileSize)

		res, err := http.Post(server.URL+"/files", contentType, body)
		if err != nil {
			t.Fatalf("POST /files: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK || received != fileSize {
			t.Fatalf("status = %d, received = %d; want 200 and %d bytes", res.StatusCode, received, fileSize)
		}
	})

	t.Run("sibling route keeps 10 MiB", func(t *testing.T) {
		res, err := http.Post(server.URL+"/tasks", "application/json", strings.NewReader(jsonBodyOfSize(uploadLimit)))
		if err != nil {
			t.Fatalf("POST /tasks: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusRequestEntityTooLarge || siblingReached {
			t.Fatalf("status = %d, reached = %v; want 413 without the handler", res.StatusCode, siblingReached)
		}
	})
}

// streamedMultipart returns a multipart body with one file part of size
// bytes, written through a pipe so the client streams it.
func streamedMultipart(t *testing.T, size int) (io.Reader, string) {
	t.Helper()
	pipeReader, pipeWriter := io.Pipe()
	form := multipart.NewWriter(pipeWriter)
	go func() {
		part, err := form.CreateFormFile("file", "upload.bin")
		if err == nil {
			_, err = io.CopyN(part, zeroReader{}, int64(size))
		}
		if err == nil {
			err = form.Close()
		}
		pipeWriter.CloseWithError(err)
	}()
	return pipeReader, form.FormDataContentType()
}

// zeroReader yields zero bytes forever.
type zeroReader struct{}

func (zeroReader) Read(b []byte) (int, error) {
	clear(b)
	return len(b), nil
}
