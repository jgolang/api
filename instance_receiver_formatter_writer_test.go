package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jgolang/api/core"
)

// fakeReceiver records the request it parsed and returns fixed content.
type fakeReceiver struct {
	path string
}

func (receiver *fakeReceiver) ProcessBody(r *http.Request) (*core.RequestDataContext, error) {
	receiver.path = r.URL.Path
	return &core.RequestDataContext{Context: r.Context(), Content: core.JSONContent(`{"title":"fake"}`)}, nil
}

func (receiver *fakeReceiver) ProcessEncryptedBody(*http.Request) (*core.RequestEncryptedData, error) {
	return &core.RequestEncryptedData{}, nil
}

func (receiver *fakeReceiver) GetRouteVar(string, *http.Request) string { return "" }

// fakeFormatter records the response data and returns a fixed body.
type fakeFormatter struct {
	data      core.ResponseData
	formatted *core.ResponseFormatted
}

func (formatter *fakeFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	formatter.data = data
	formatter.formatted = &core.ResponseFormatted{HTTPStatusCode: data.HTTPStatusCode, Body: "formatted-by-fake"}
	return formatter.formatted
}

// fakeWriter counts its calls, keeps the last formatted response and writes
// a fixed body. It is safe for concurrent use.
type fakeWriter struct {
	calls     atomic.Int32
	mu        sync.Mutex
	formatted *core.ResponseFormatted
}

func (writer *fakeWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	writer.calls.Add(1)
	writer.mu.Lock()
	writer.formatted = response
	writer.mu.Unlock()
	w.WriteHeader(response.HTTPStatusCode)
	w.Write([]byte("written-by-fake"))
}

// fakeGuarantor records that a token reached it.
type fakeGuarantor struct {
	basicCalls, customCalls int
}

func (guarantor *fakeGuarantor) ValidateBasicToken(string) (string, string, bool) {
	guarantor.basicCalls++
	return "client", "secret", true
}

func (guarantor *fakeGuarantor) ValidateCustomToken(string, core.CustomTokenValidator) (json.RawMessage, bool) {
	guarantor.customCalls++
	return json.RawMessage(`{}`), true
}

// useSecurityGuarantor registers guarantor on the default instance for one
// test and restores the built-in one afterwards.
func useSecurityGuarantor(t *testing.T, guarantor core.APISecurityGuarantor) {
	t.Helper()
	RegisterNewAPISecurityGuarantor(guarantor)
	t.Cleanup(func() { RegisterNewAPISecurityGuarantor(&SecurityGuaranter{}) })
}

// useParamValidator registers validator on the default instance for one test
// and restores the built-in one afterwards.
func useParamValidator(t *testing.T, validator func(any) (string, error)) {
	t.Helper()
	RegisterParamValidator(validator)
	t.Cleanup(func() { RegisterParamValidator(ParamValidatorV0) })
}

var writeSuccess = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) })

func TestInstanceUsesItsCustomReceiverFormatterAndWriter(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	receiver, formatter, writer := &fakeReceiver{}, &fakeFormatter{}, &fakeWriter{}
	a := New(WithRequestReceiver(receiver), WithResponseFormatter(formatter), WithResponseWriter(writer))

	rec := serve(a.Handler(ProcessRequest(writeSuccess)), http.MethodPost, `{"title":"task"}`, nil)

	if receiver.path != "/tasks" {
		t.Errorf("receiver parsed path %q, want /tasks", receiver.path)
	}
	if formatter.data.ResponseType != SuccessType {
		t.Errorf("formatter got response type %q, want success", formatter.data.ResponseType)
	}
	if writer.formatted == nil || writer.formatted != formatter.formatted {
		t.Errorf("writer did not get the formatter's output")
	}
	if got := rec.Body.String(); got != "written-by-fake" {
		t.Errorf("body = %q, want only what the writer wrote", got)
	}
}

func TestCustomComponentsDoNotLeakIntoTheDefaultInstance(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	_ = New(WithRequestReceiver(&fakeReceiver{}), WithResponseFormatter(&fakeFormatter{}), WithResponseWriter(&fakeWriter{}))

	rec := serve(writeSuccess, http.MethodGet, "", nil)

	if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
		t.Errorf("body = %s, want the built-in envelope", rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestCustomFormatterAloneDecidesTheBody(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	a := New(WithResponseFormatter(&fakeFormatter{}), WithResponseMode(ResponseModePlain))

	rec := serve(a.Handler(writeSuccess), http.MethodGet, "", nil)

	if got, want := rec.Body.String(), "\"formatted-by-fake\"\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestBuiltInFormatterPassedAsOptionUsesTheInstanceMode(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	tests := map[string]core.APIResponseFormatter{
		"value":   ResponseFormatter{},
		"pointer": &ResponseFormatter{},
	}
	for name, formatter := range tests {
		t.Run(name, func(t *testing.T) {
			a := New(WithResponseFormatter(formatter), WithResponseMode(ResponseModePlain))

			rec := serve(a.Handler(writeSuccess), http.MethodGet, "", nil)

			if got := responseModeOf(rec.Body.Bytes()); got != ResponseModePlain {
				t.Fatalf("response mode = %s, want the instance's plain (%s)", got, rec.Body)
			}
		})
	}
}

func TestBuiltInReceiverPassedAsOptionUsesTheInstanceMode(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	tests := map[string]core.APIRequestReceiver{
		"v0 value":   RequestReceiver{},
		"v0 pointer": &RequestReceiver{},
		"v2 value":   RequestReceiverV2{},
		"v2 pointer": &RequestReceiverV2{},
	}
	for name, receiver := range tests {
		t.Run(name, func(t *testing.T) {
			a := New(WithRequestReceiver(receiver), WithRequestMode(RequestModeEnvelope))

			rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, `{"header":{},"content":{"title":"task"}}`, nil)

			var body struct {
				Content struct {
					Title string `json:"title"`
				} `json:"content"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Content.Title != "task" {
				t.Fatalf("want the envelope unwrapped, got %s", rec.Body)
			}
		})
	}
}

func TestNilComponentsKeepTheBuiltInOnes(t *testing.T) {
	usePackageModes(t, RequestModeEnvelope, ResponseModePlain)
	tests := map[string][]Option{
		"nil interfaces":          {WithRequestReceiver(nil), WithResponseFormatter(nil), WithResponseWriter(nil)},
		"nil v0 receiver pointer": {WithRequestReceiver((*RequestReceiver)(nil))},
		"nil v2 receiver pointer": {WithRequestReceiver((*RequestReceiverV2)(nil))},
		"nil formatter pointer":   {WithResponseFormatter((*ResponseFormatter)(nil))},
	}
	for name, opts := range tests {
		t.Run(name, func(t *testing.T) {
			a := New(opts...)

			rec := serve(a.Handler(ProcessRequest(echoTitle(t))), http.MethodPost, `{"title":"task"}`, nil)

			if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
				t.Fatalf("response mode = %s, want the built-in envelope (%s)", got, rec.Body)
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
		})
	}
}

// The package helpers (ValidateBasicToken, UnmarshalBody, the token
// middlewares) already use the package core; these tests cover the instance
// core's own guarantor and param validator.
func TestInstanceCoreForwardsToTheRegisteredSecurityGuarantor(t *testing.T) {
	guarantor := &fakeGuarantor{}
	useSecurityGuarantor(t, guarantor)
	a := New()

	a.core.ValidateBasicToken("token")
	a.core.ValidateCustomToken("token", nil)

	if guarantor.basicCalls != 1 || guarantor.customCalls != 1 {
		t.Fatalf("guarantor calls = basic %d, custom %d; want 1 and 1", guarantor.basicCalls, guarantor.customCalls)
	}
}

func TestInstanceCoreForwardsToTheRegisteredParamValidator(t *testing.T) {
	var calls int
	useParamValidator(t, func(any) (string, error) {
		calls++
		return "", nil
	})
	a := New()

	a.core.ValidateParams(struct{}{})

	if calls != 1 {
		t.Fatalf("validator calls = %d, want 1", calls)
	}
}

func TestBuiltInstanceIgnoresLaterOptions(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	writer := &fakeWriter{}
	opts := []Option{WithResponseMode(ResponseModePlain), WithResponseWriter(writer)}
	a := New(opts...)

	_ = New(WithResponseMode(ResponseModeNone), WithResponseWriter(ResponseWriter{}), WithResponseFormatter(&fakeFormatter{}))
	_ = New(opts...)
	rec := serve(a.Handler(writeSuccess), http.MethodGet, "", nil)

	if got := rec.Body.String(); got != "written-by-fake" {
		t.Fatalf("body = %q, want a's own writer", got)
	}
	if responseModeOf(mustJSON(t, writer.formatted.Body)) != ResponseModePlain {
		t.Fatalf("formatted body = %v, want a's plain mode", writer.formatted.Body)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestInstancesWithCustomAndBuiltInWritersServeConcurrently(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeEnvelope)
	writer := &fakeWriter{}
	custom := New(WithResponseWriter(writer)).Handler(writeSuccess)
	builtIn := New(WithResponseMode(ResponseModePlain)).Handler(writeSuccess)
	const requests = 50

	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() {
			if got := serve(custom, http.MethodGet, "", nil).Body.String(); got != "written-by-fake" {
				t.Errorf("custom body = %q", got)
			}
		})
		wg.Go(func() {
			if got := responseModeOf(serve(builtIn, http.MethodGet, "", nil).Body.Bytes()); got != ResponseModePlain {
				t.Errorf("built-in response mode = %s, want plain", got)
			}
		})
	}
	wg.Wait()

	if got := writer.calls.Load(); got != requests {
		t.Fatalf("custom writer calls = %d, want %d", got, requests)
	}
}
