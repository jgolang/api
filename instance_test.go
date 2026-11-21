package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jgolang/api/core"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// usePackageModes sets the package-level modes for one test and restores them
// afterwards.
func usePackageModes(t *testing.T, request RequestMode, response ResponseMode) {
	t.Helper()
	oldRequest, oldResponse := CurrentRequestMode, CurrentResponseMode
	CurrentRequestMode, CurrentResponseMode = request, response
	t.Cleanup(func() {
		CurrentRequestMode, CurrentResponseMode = oldRequest, oldResponse
	})
}

// serve runs handler once and returns the recorded response.
func serve(handler http.Handler, method, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/tasks", bytes.NewBufferString(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// responseModeOf tells which response mode wrote body: plain bodies carry
// "type" at the top level, envelope bodies carry it under "header". It
// returns "" for any other body, and is safe to call from any goroutine.
func responseModeOf(body []byte) ResponseMode {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return ""
	}
	if _, ok := fields["header"]; ok {
		return ResponseModeEnvelope
	}
	if _, ok := fields["type"]; ok {
		return ResponseModePlain
	}
	return ""
}

// echoTitle answers with the title it decoded from the request content.
func echoTitle(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decoding content: %v", err)
		}
		SuccessWithContent(payload).Write(w, r)
	}
}

func TestInstanceUsesItsOwnModes(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	a := New(WithResponseMode(ResponseModePlain), WithRequestMode(RequestModeEnvelope))

	rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, `{"header":{},"content":{"title":"task"}}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	var body struct {
		Type    string `json:"type"`
		Content struct {
			Title string `json:"title"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Type != "success" || body.Content.Title != "task" {
		t.Fatalf("want plain body with unwrapped content, got %s", rec.Body)
	}
}

func TestNewWithoutOptionsUsesV0Defaults(t *testing.T) {
	usePackageModes(t, RequestModeEnvelope, ResponseModePlain)
	a := New()

	rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, `{"title":"task"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
		t.Fatalf("response mode = %s, want envelope", got)
	}
	var body struct {
		Content struct {
			Title string `json:"title"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Content.Title != "task" {
		t.Fatalf("want plain request parsing, got %s", rec.Body)
	}
}

func TestNewFallsBackToDefaultsForUnknownModes(t *testing.T) {
	usePackageModes(t, RequestModeEnvelope, ResponseModePlain)
	a := New(WithRequestMode("x"), WithResponseMode("y"))

	rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, `{"title":"task"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
		t.Fatalf("response mode = %s, want envelope", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"title":"task"`)) {
		t.Fatalf("want plain request parsing, got %s", rec.Body)
	}
}

// TestPackageModesKeepV0Bodies pins the bodies written by v0.10.0 (baseline
// ad03d5f) for handlers that are not wrapped by an instance, except that
// since v1.0.0 the default instance sends codes as declared, like New().
func TestPackageModesKeepV0Bodies(t *testing.T) {
	notFound := errors.NewC(codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound), "x")
	writers := map[string]http.HandlerFunc{
		"success":    func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) },
		"error400":   func(w http.ResponseWriter, r *http.Request) { Error400().Write(w, r) },
		"writeError": func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, notFound) },
		// 200: the v0 default status of Warning and Informative (100) cannot carry a body.
		"warning":     func(w http.ResponseWriter, r *http.Request) { Warning{HTTPStatusCode: http.StatusOK}.Write(w, r) },
		"informative": func(w http.ResponseWriter, r *http.Request) { Informative{HTTPStatusCode: http.StatusOK}.Write(w, r) },
	}
	tests := []struct {
		mode       ResponseMode
		writer     string
		wantStatus int
		wantBody   string
	}{
		{ResponseModePlain, "success", 200, `{"action":"","code":"0000","content":null,"event_id":"evt-123456789","info":null,"message":"","title":"","token":"","type":"success"}` + "\n"},
		{ResponseModePlain, "error400", 400, `{"action":"","code":"E001","content":null,"event_id":"evt-123456789","info":null,"message":"The service has not completed the operation! (evt-12-E001)","title":"Error response!","token":"","type":"error"}` + "\n"},
		{ResponseModePlain, "writeError", 404, `{"action":"","code":"PLN-111","content":null,"event_id":"evt-123456789","info":null,"message":"task not found (evt-12-PLN-111)","title":"Error response!","token":"","type":"error"}` + "\n"},
		{ResponseModeEnvelope, "success", 200, `{"header":{"type":"success","code":"0000","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModeEnvelope, "error400", 400, `{"header":{"type":"error","title":"Error response!","message":"The service has not completed the operation! (evt-12-E001)","code":"E001","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModeEnvelope, "writeError", 404, `{"header":{"type":"error","title":"Error response!","message":"task not found (evt-12-PLN-111)","code":"PLN-111","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModePlain, "warning", 200, `{"action":"","code":"0000","content":null,"event_id":"evt-123456789","info":null,"message":"The application has been successful but with potential problems! (evt-12-0000)","title":"Alert!","token":"","type":"warning"}` + "\n"},
		{ResponseModePlain, "informative", 200, `{"action":"","code":"0000","content":null,"event_id":"evt-123456789","info":null,"message":"The request has been successful! (evt-12-0000)","title":"Information!","token":"","type":"info"}` + "\n"},
		{ResponseModeEnvelope, "warning", 200, `{"header":{"type":"warning","title":"Alert!","message":"The application has been successful but with potential problems! (evt-12-0000)","code":"0000","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModeEnvelope, "informative", 200, `{"header":{"type":"info","title":"Information!","message":"The request has been successful! (evt-12-0000)","code":"0000","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModeNone, "success", 200, "null\n"},
		{ResponseModeNone, "error400", 400, "null\n"},
		{ResponseModeNone, "writeError", 404, "null\n"},
		{ResponseModeNone, "warning", 200, "null\n"},
		{ResponseModeNone, "informative", 200, "null\n"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"/"+tt.writer, func(t *testing.T) {
			usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
			SetResponseMode(tt.mode)

			rec := serve(writers[tt.writer], http.MethodGet, "", map[string]string{EventIDHeaderKey: "evt-123456789"})

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" || len(rec.Header()) != 1 {
				t.Errorf("headers = %v, want only Content-Type: application/json", rec.Header())
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %s\nwant   %s", got, tt.wantBody)
			}
		})
	}
}

func TestInstanceAnswersMiddlewareRejectionsInItsMode(t *testing.T) {
	ok := func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) }
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		body       string
		headers    map[string]string
		wantStatus int
	}{
		{name: "ProcessRequest bad JSON", handler: ProcessRequest(ok), body: `{"title":`, wantStatus: http.StatusBadRequest},
		{name: "RequestBody bad JSON", handler: RequestBody(ok), body: `{"title":`, wantStatus: http.StatusBadRequest},
		{name: "BasicToken bad header", handler: BasicToken(ok), headers: map[string]string{"Authorization": "Nope"}, wantStatus: http.StatusUnauthorized},
		{name: "CustomToken bad header", handler: CustomToken(ok), headers: map[string]string{"Authorization": "Nope"}, wantStatus: http.StatusUnauthorized},
		{name: "Warning", handler: func(w http.ResponseWriter, r *http.Request) { Warning{HTTPStatusCode: http.StatusOK}.Write(w, r) }, wantStatus: http.StatusOK},
		{name: "Informative", handler: func(w http.ResponseWriter, r *http.Request) { Informative{HTTPStatusCode: http.StatusOK}.Write(w, r) }, wantStatus: http.StatusOK},
		{name: "WriteError", handler: func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, nil) }, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
			a := New(WithResponseMode(ResponseModePlain))

			rec := serve(a.Handler(tt.handler), http.MethodPost, tt.body, tt.headers)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if got := responseModeOf(rec.Body.Bytes()); got != ResponseModePlain {
				t.Fatalf("response mode = %s, want plain (%s)", got, rec.Body)
			}
		})
	}
}

func TestRequestBodyParsesWithTheInstanceRequestMode(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	a := New(WithRequestMode(RequestModeEnvelope))
	reached := false
	next := func(w http.ResponseWriter, r *http.Request) { reached = true }

	// A JSON array is valid plain content but not an envelope.
	rec := serve(a.Handler(RequestBody(next)), http.MethodPost, `[1]`, nil)

	if reached || rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 from envelope parsing, got %d (reached next: %v)", rec.Code, reached)
	}
}

func TestInstancesServeConcurrentlyInTheirOwnModes(t *testing.T) {
	usePackageModes(t, RequestModeAuto, ResponseModeNone)
	instances := map[ResponseMode]*API{
		ResponseModePlain:    New(WithResponseMode(ResponseModePlain)),
		ResponseModeEnvelope: New(WithResponseMode(ResponseModeEnvelope)),
	}
	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) })

	var wg sync.WaitGroup
	for want, a := range instances {
		served := a.Handler(handler)
		for range 20 {
			wg.Go(func() {
				rec := serve(served, http.MethodGet, "", nil)
				if got := responseModeOf(rec.Body.Bytes()); got != want {
					t.Errorf("response mode = %s, want %s", got, want)
				}
			})
		}
	}
	wg.Wait()

	if CurrentRequestMode != RequestModeAuto || CurrentResponseMode != ResponseModeNone {
		t.Fatalf("package modes changed to %s/%s", CurrentRequestMode, CurrentResponseMode)
	}
}

func TestInstanceFromFallsBackToTheDefaultInstance(t *testing.T) {
	tests := []struct {
		name string
		r    *http.Request
	}{
		{name: "nil request", r: nil},
		{name: "request without instance", r: httptest.NewRequest(http.MethodGet, "/", nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := instanceFrom(tt.r); got != defaultInstance {
				t.Fatalf("instanceFrom = %p, want the default instance %p", got, defaultInstance)
			}
		})
	}
}

// requestWithInstance returns a request that Handler served with a.
func requestWithInstance(a *API) *http.Request {
	var served *http.Request
	a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { served = r })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	return served
}

func TestQuickStartAnswersLikeTheDefaultInstance(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	titleRequired := codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)
	createTask := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			Error400().Write(w, r)
			return
		}
		if payload.Title == "" {
			WriteError(w, r, errors.NewC(titleRequired, "empty title"))
			return
		}
		success := SuccessWithContent(payload)
		success.HTTPStatusCode = http.StatusCreated
		success.Write(w, r)
	})
	// Event IDs are random per request: blank them and the id in the trace suffix.
	eventIDs := regexp.MustCompile(`"event_id":"[0-9a-f]+"|\([0-9a-f]*-`)

	for _, body := range []string{`{"title":"Buy milk"}`, `{"title":""}`, `{"title":`, `[1]`} {
		t.Run(body, func(t *testing.T) {
			withoutInstance := serve(createTask, http.MethodPost, body, nil)
			// WithLowercaseCodes keeps an instance's codes as v0 sent them.
			lowercase := serve(New(WithLowercaseCodes()).Handler(createTask), http.MethodPost, body, nil)
			declared := serve(New().Handler(createTask), http.MethodPost, body, nil)

			for name, withInstance := range map[string]*httptest.ResponseRecorder{"New(WithLowercaseCodes())": lowercase, "New()": declared} {
				if withoutInstance.Code != withInstance.Code {
					t.Errorf("status = %d without instance, %d with %s", withoutInstance.Code, withInstance.Code, name)
				}
				if a, b := withoutInstance.Header().Get("Content-Type"), withInstance.Header().Get("Content-Type"); a != b {
					t.Errorf("Content-Type = %q without instance, %q with %s", a, b, name)
				}
			}
			a := eventIDs.ReplaceAllString(withoutInstance.Body.String(), "ID")
			if b := eventIDs.ReplaceAllString(declared.Body.String(), "ID"); a != b {
				t.Errorf("body without instance: %s\nbody with New():        %s", a, b)
			}
			// The lowercase instance differs only in code case.
			if b := eventIDs.ReplaceAllString(lowercase.Body.String(), "ID"); strings.ToLower(a) != strings.ToLower(b) {
				t.Errorf("body without instance: %s\nbody with lowercase instance: %s", a, b)
			}
		})
	}
}

func TestNewSkipsNilOptions(t *testing.T) {
	var unset Option

	usePackageModes(t, RequestModeEnvelope, ResponseModeEnvelope)
	a := New(nil, unset, WithResponseMode(ResponseModePlain))

	rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, `{"title":"task"}`, nil)

	if got := responseModeOf(rec.Body.Bytes()); got != ResponseModePlain {
		t.Fatalf("response mode = %s, want plain (%s)", got, rec.Body)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"title":"task"`)) {
		t.Fatalf("want plain request parsing, got %s", rec.Body)
	}
}

func TestHandlerOnAZeroOrNilAPIKeepsTheOuterInstance(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	outer := New(WithResponseMode(ResponseModePlain))
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) })

	for name, inner := range map[string]*API{"zero API": {}, "nil API": nil} {
		t.Run(name, func(t *testing.T) {
			rec := serve(outer.Handler(inner.Handler(ok)), http.MethodGet, "", nil)

			if got := responseModeOf(rec.Body.Bytes()); got != ResponseModePlain {
				t.Fatalf("response mode = %s, want the outer instance's plain", got)
			}
		})
	}
	t.Run("alone", func(t *testing.T) {
		rec := serve((&API{}).Handler(ok), http.MethodGet, "", nil)

		if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
			t.Fatalf("response mode = %s, want the default instance's envelope", got)
		}
	})
}

func TestTheInnermostInstanceWins(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeNone)
	outer := New(WithResponseMode(ResponseModePlain))
	inner := New(WithResponseMode(ResponseModeEnvelope))
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) })

	rec := serve(outer.Handler(inner.Handler(ok)), http.MethodGet, "", nil)

	if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
		t.Fatalf("response mode = %s, want the inner instance's envelope", got)
	}
}

func TestNormalizeModes(t *testing.T) {
	requestModes := map[RequestMode]RequestMode{
		RequestModeNone:     RequestModeNone,
		RequestModePlain:    RequestModePlain,
		RequestModeEnvelope: RequestModeEnvelope,
		RequestModeAuto:     RequestModeAuto,
		"":                  RequestModePlain,
		"unknown":           RequestModePlain,
	}
	for in, want := range requestModes {
		if got := normalizeRequestMode(in); got != want {
			t.Errorf("normalizeRequestMode(%q) = %q, want %q", in, got, want)
		}
	}
	responseModes := map[ResponseMode]ResponseMode{
		ResponseModeNone:     ResponseModeNone,
		ResponseModePlain:    ResponseModePlain,
		ResponseModeEnvelope: ResponseModeEnvelope,
		"":                   ResponseModeEnvelope,
		"unknown":            ResponseModeEnvelope,
	}
	for in, want := range responseModes {
		if got := normalizeResponseMode(in); got != want {
			t.Errorf("normalizeResponseMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAutoInstanceParsesEnvelopeAndPlainBodies(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	a := New(WithRequestMode(RequestModeAuto))

	for _, body := range []string{`{"header":{},"content":{"title":"task"}}`, `{"title":"task"}`} {
		rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, body, nil)

		if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"title":"task"`)) {
			t.Errorf("body %s: got %d %s, want 200 with the unwrapped title", body, rec.Code, rec.Body)
		}
	}
}

type stubFormatter struct{}

func (stubFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{HTTPStatusCode: data.HTTPStatusCode, Body: "stub"}
}

type stubReceiver struct{ RequestReceiver }

func (stubReceiver) ProcessEncryptedBody(r *http.Request) (*core.RequestEncryptedData, error) {
	return &core.RequestEncryptedData{Data: "stub"}, nil
}

func TestRegisteredImplementationsServeOnlyTheDefaultInstance(t *testing.T) {
	RegisterNewAPIResponseFormatter(stubFormatter{})
	RegisterNewAPIRequestReceiver(stubReceiver{})
	t.Cleanup(func() {
		RegisterNewAPIResponseFormatter(ResponseFormatter{})
		RegisterNewAPIRequestReceiver(RequestReceiver{})
	})
	write := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) })
	var encryptedData any
	readEncrypted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, _ := ProcessEncryptedBody(r)
		encryptedData = request.Data
	})
	encryptedBody := `{"data":"built-in"}`

	t.Run("default instance", func(t *testing.T) {
		if got := serve(write, http.MethodGet, "", nil).Body.String(); got != "\"stub\"\n" {
			t.Errorf("body = %q, want the stub formatter's", got)
		}
		serve(readEncrypted, http.MethodPost, encryptedBody, nil)
		if encryptedData != "stub" {
			t.Errorf("encrypted data = %v, want the stub receiver's", encryptedData)
		}
	})
	t.Run("instance", func(t *testing.T) {
		a := New()
		if got := responseModeOf(serve(a.Handler(write), http.MethodGet, "", nil).Body.Bytes()); got != ResponseModeEnvelope {
			t.Errorf("response mode = %q, want the built-in formatter's envelope", got)
		}
		serve(a.Handler(readEncrypted), http.MethodPost, encryptedBody, nil)
		if encryptedData != "built-in" {
			t.Errorf("encrypted data = %v, want the built-in receiver's", encryptedData)
		}
	})
}
