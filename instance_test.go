package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

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
// ad03d5f) for handlers that are not wrapped by an instance.
func TestPackageModesKeepV0Bodies(t *testing.T) {
	notFound := errors.NewC(codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound), "x")
	writers := map[string]http.HandlerFunc{
		"success":    func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) },
		"error400":   func(w http.ResponseWriter, r *http.Request) { Error400().Write(w, r) },
		"writeError": func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, notFound) },
	}
	tests := []struct {
		mode       ResponseMode
		writer     string
		wantStatus int
		wantBody   string
	}{
		{ResponseModePlain, "success", 200, `{"action":"","code":"0000","content":null,"event_id":"evt-123456789","info":null,"message":"","title":"","token":"","type":"success"}` + "\n"},
		{ResponseModePlain, "error400", 400, `{"action":"","code":"e001","content":null,"event_id":"evt-123456789","info":null,"message":"The service has not completed the operation! (evt-12-e001)","title":"Error response!","token":"","type":"error"}` + "\n"},
		{ResponseModePlain, "writeError", 404, `{"action":"","code":"pln-111","content":null,"event_id":"evt-123456789","info":null,"message":"task not found (evt-12-pln-111)","title":"Error response!","token":"","type":"error"}` + "\n"},
		{ResponseModeEnvelope, "success", 200, `{"header":{"type":"success","code":"0000","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModeEnvelope, "error400", 400, `{"header":{"type":"error","title":"Error response!","message":"The service has not completed the operation! (evt-12-e001)","code":"e001","event_id":"evt-123456789"}}` + "\n"},
		{ResponseModeEnvelope, "writeError", 404, `{"header":{"type":"error","title":"Error response!","message":"task not found (evt-12-pln-111)","code":"pln-111","event_id":"evt-123456789"}}` + "\n"},
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
		{name: "zero API", r: requestWithInstance(&API{})},
		{name: "typed nil API", r: requestWithInstance((*API)(nil))},
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

func TestQuickStartAnswersTheSameWithTheDefaultInstanceOptions(t *testing.T) {
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
			withInstance := serve(New().Handler(createTask), http.MethodPost, body, nil)

			if withoutInstance.Code != withInstance.Code {
				t.Errorf("status = %d without instance, %d with", withoutInstance.Code, withInstance.Code)
			}
			if a, b := withoutInstance.Header().Get("Content-Type"), withInstance.Header().Get("Content-Type"); a != b {
				t.Errorf("Content-Type = %q without instance, %q with", a, b)
			}
			a := eventIDs.ReplaceAllString(withoutInstance.Body.String(), "ID")
			b := eventIDs.ReplaceAllString(withInstance.Body.String(), "ID")
			if a != b {
				t.Errorf("body without instance: %s\nbody with instance:    %s", a, b)
			}
		})
	}
}
