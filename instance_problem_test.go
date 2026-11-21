package api

import (
	"encoding/json"
	"math"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

var taskNotFound = codes.New("PLN-404", "task not found").WithKind(codes.KindNotFound)

func writeTaskNotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, errors.NewC(taskNotFound, "task 7"))
}

func problemInstance(opts ...Option) *API {
	return New(append([]Option{WithResponseMode(ResponseModeProblem)}, opts...)...)
}

// decodeMembers decodes a JSON object body into its members.
func decodeMembers(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var members map[string]any
	if err := json.Unmarshal(body, &members); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return members
}

func TestProblemInstanceWritesAnErrorAsProblemDetails(t *testing.T) {
	a := problemInstance()

	rec := serve(a.Handler(http.HandlerFunc(writeTaskNotFound)), http.MethodGet, "", map[string]string{EventIDHeaderKey: "evt-123"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != problemContentType {
		t.Fatalf("Content-Type = %q, want %q", got, problemContentType)
	}
	want := `{"type":"about:blank","title":"Error response!","status":404,"detail":"task not found","code":"PLN-404","event_id":"evt-123"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestProblemDetailsMembers(t *testing.T) {
	tests := []struct {
		name    string
		a       *API
		handler http.HandlerFunc
		body    string
		headers map[string]string
		// ignoreEventID drops event_id before comparing: only ProcessRequest
		// generates one for a request without an event ID header.
		ignoreEventID bool
		want          map[string]any
	}{
		{
			name:    "no event ID",
			a:       problemInstance(),
			handler: writeTaskNotFound,
			want:    map[string]any{"type": "about:blank", "title": "Error response!", "status": 404.0, "detail": "task not found", "code": "PLN-404"},
		},
		{
			name:    "lowercase codes",
			a:       problemInstance(WithLowercaseCodes()),
			handler: writeTaskNotFound,
			want:    map[string]any{"type": "about:blank", "title": "Error response!", "status": 404.0, "detail": "task not found", "code": "pln-404"},
		},
		{
			name:    "Error400",
			a:       problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) { Error400().Write(w, r) },
			want:    map[string]any{"type": "about:blank", "title": DefaultErrorTitle, "status": 400.0, "detail": DefaultErrorMessage, "code": "E001"},
		},
		{
			name:    "Error401",
			a:       problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) { Error401().Write(w, r) },
			want:    map[string]any{"type": "about:blank", "title": UnauthorizedTitle, "status": 401.0, "detail": UnauthorizedMessage, "code": "B001"},
		},
		{
			name:    "WriteError nil",
			a:       problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, nil) },
			want:    map[string]any{"type": "about:blank", "title": InternalServerTitle, "status": 500.0, "detail": InternalServerMessage, "code": "E300"},
		},
		{
			name:    "formatter value",
			a:       problemInstance(WithResponseFormatter(ResponseFormatter{})),
			handler: writeTaskNotFound,
			want:    map[string]any{"type": "about:blank", "title": "Error response!", "status": 404.0, "detail": "task not found", "code": "PLN-404"},
		},
		{
			name:    "formatter pointer",
			a:       problemInstance(WithResponseFormatter(&ResponseFormatter{})),
			handler: writeTaskNotFound,
			want:    map[string]any{"type": "about:blank", "title": "Error response!", "status": 404.0, "detail": "task not found", "code": "PLN-404"},
		},
		{
			// The rejection comes before ProcessRequest generates an event ID.
			name:    "ProcessRequest bad JSON",
			a:       problemInstance(),
			handler: ProcessRequest(writeSuccess),
			body:    `{"title":`,
			want:    map[string]any{"type": "about:blank", "title": "Invalid request content", "status": 400.0, "detail": "Request content empty json structure", "code": "E002"},
		},
		{
			name:          "ProcessRequest panic",
			a:             problemInstance(),
			handler:       ProcessRequest(func(http.ResponseWriter, *http.Request) { panic("boom") }),
			ignoreEventID: true,
			want:          map[string]any{"type": "about:blank", "title": InternalServerTitle, "status": 500.0, "detail": InternalServerMessage, "code": "E300"},
		},
		{
			name:    "WithKindStatus 410",
			a:       problemInstance(WithKindStatus(codes.KindNotFound, http.StatusGone)),
			handler: writeTaskNotFound,
			want:    map[string]any{"type": "about:blank", "title": DefaultErrorTitle, "status": 410.0, "detail": "task not found", "code": "PLN-404"},
		},
		{
			name:    "WithKindStatus 503",
			a:       problemInstance(WithKindStatus(codes.KindNotFound, http.StatusServiceUnavailable)),
			handler: writeTaskNotFound,
			want:    map[string]any{"type": "about:blank", "title": InternalServerTitle, "status": 503.0, "detail": "task not found", "code": "PLN-404"},
		},
		{
			name:    "BasicToken",
			a:       problemInstance(),
			handler: BasicToken(writeSuccess),
			want:    map[string]any{"type": "about:blank", "title": UnauthorizedTitle, "status": 401.0, "detail": DefaultInvalidAuthHeaderMsg, "code": "B001"},
		},
		{
			name:    "CustomToken",
			a:       problemInstance(),
			handler: CustomToken(writeSuccess),
			want:    map[string]any{"type": "about:blank", "title": UnauthorizedTitle, "status": 401.0, "detail": DefaultInvalidAuthHeaderMsg, "code": "B001"},
		},
		{
			name:    "RequestHeaderJSON",
			a:       problemInstance(),
			handler: RequestHeaderJSON(writeSuccess),
			body:    `{}`,
			headers: map[string]string{"Content-Type": "text/plain"},
			want:    map[string]any{"type": "about:blank", "title": DefaultErrorTitle, "status": 415.0, "detail": "Content-Type not is JSON!", "code": "E002"},
		},
		{
			name:    "RequestHeaderSession",
			a:       problemInstance(),
			handler: RequestHeaderSession(writeSuccess),
			want:    map[string]any{"type": "about:blank", "title": UnauthorizedTitle, "status": 401.0, "detail": "Invalid session ID", "code": "B001"},
		},
		{
			name:    "RequestBody",
			a:       problemInstance(),
			handler: RequestBody(writeSuccess),
			body:    `{"title":`,
			want:    map[string]any{"type": "about:blank", "title": "Invalid request content", "status": 400.0, "detail": "Request content empty json structure", "code": "E002"},
		},
		{
			name:    "Error403",
			a:       problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) { Error403().Write(w, r) },
			want:    map[string]any{"type": "about:blank", "title": UnauthorizedTitle, "status": 403.0, "detail": UnauthorizedMessage, "code": "B002"},
		},
		{
			name:    "Error500",
			a:       problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) { Error500().Write(w, r) },
			want:    map[string]any{"type": "about:blank", "title": InternalServerTitle, "status": 500.0, "detail": InternalServerMessage, "code": "E300"},
		},
		{
			name:    "Error501",
			a:       problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) { Error501().Write(w, r) },
			want:    map[string]any{"type": "about:blank", "title": InternalServerTitle, "status": 501.0, "detail": InternalServerMessage, "code": "E001"},
		},
		{
			// Token, content, actions and info never reach a problem body.
			name: "token, content and info left out",
			a:    problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) {
				response := Error400()
				response.Content = map[string]string{"field": "title"}
				response.SecurityToken = "s3cr3t"
				response.Actions = "retry"
				response.Info = map[string]string{"k": "v"}
				response.Write(w, r)
			},
			headers: map[string]string{"Authorization": "Bearer t0k3n"},
			want:    map[string]any{"type": "about:blank", "title": DefaultErrorTitle, "status": 400.0, "detail": DefaultErrorMessage, "code": "E001"},
		},
		{
			name: "empty Content-Type in Headers",
			a:    problemInstance(),
			handler: func(w http.ResponseWriter, r *http.Request) {
				Error{Headers: map[string]string{"Content-Type": ""}}.Write(w, r)
			},
			want: map[string]any{"type": "about:blank", "title": DefaultErrorTitle, "status": 400.0, "detail": DefaultErrorMessage, "code": "E001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.a.Handler(tt.handler), http.MethodPost, tt.body, tt.headers)

			if got := rec.Header().Get("Content-Type"); got != problemContentType {
				t.Fatalf("Content-Type = %q, want %q", got, problemContentType)
			}
			if want := int(tt.want["status"].(float64)); rec.Code != want {
				t.Fatalf("status = %d, want %d", rec.Code, want)
			}
			got := decodeMembers(t, rec.Body.Bytes())
			if tt.ignoreEventID {
				delete(got, "event_id")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("members = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProblemInstanceWritesOtherResponsesAsContent(t *testing.T) {
	content := map[string]int{"id": 1}
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"Success", func(w http.ResponseWriter, r *http.Request) { SuccessWithContent(content).Write(w, r) }},
		{"Warning", func(w http.ResponseWriter, r *http.Request) {
			Warning{HTTPStatusCode: http.StatusOK, Content: content}.Write(w, r)
		}},
		{"Informative", func(w http.ResponseWriter, r *http.Request) {
			Informative{HTTPStatusCode: http.StatusOK, Content: content}.Write(w, r)
		}},
		// A Content-Type the handler put on w.Header() never labels the body.
		{"handler preset text/html", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			SuccessWithContent(content).Write(w, r)
		}},
		{"empty Content-Type in Headers", func(w http.ResponseWriter, r *http.Request) {
			Success{Content: content, Headers: map[string]string{"Content-Type": ""}}.Write(w, r)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(problemInstance().Handler(tt.handler), http.MethodGet, "", nil)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != `{"id":1}` {
				t.Fatalf("body = %s, want the content alone", got)
			}
		})
	}
}

func TestProblemInstanceKeepsHeadersWithoutChangingTheCallersMap(t *testing.T) {
	headers := map[string]string{"X-Trace": "t"}
	handler := func(w http.ResponseWriter, r *http.Request) {
		Error{Headers: headers}.Write(w, r)
	}

	rec := serve(problemInstance().Handler(http.HandlerFunc(handler)), http.MethodGet, "", nil)

	if got := rec.Header().Get("X-Trace"); got != "t" {
		t.Fatalf("X-Trace = %q, want t", got)
	}
	if got := rec.Header().Get("Content-Type"); got != problemContentType {
		t.Fatalf("Content-Type = %q, want %q", got, problemContentType)
	}
	if want := map[string]string{"X-Trace": "t"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("caller's headers = %v, want %v", headers, want)
	}
}

func TestProblemInstanceKeepsTheCallersContentType(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		Error{Headers: map[string]string{"content-type": "application/vnd.x+json"}}.Write(w, r)
	}

	rec := serve(problemInstance().Handler(http.HandlerFunc(handler)), http.MethodGet, "", nil)

	if got := rec.Header().Get("Content-Type"); got != "application/vnd.x+json" {
		t.Fatalf("Content-Type = %q, want the caller's", got)
	}
}

func TestBuiltInWriterKeepsAContentTypeSetInHeaders(t *testing.T) {
	csv := func(w http.ResponseWriter, r *http.Request) {
		Success{Headers: map[string]string{"Content-Type": "text/csv"}}.Write(w, r)
	}
	plain := func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) }
	empty := func(w http.ResponseWriter, r *http.Request) {
		Success{Headers: map[string]string{"content-type": ""}}.Write(w, r)
	}
	preset := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		Success200().Write(w, r)
	}
	plainInstance := New(WithResponseMode(ResponseModePlain))
	tests := []struct {
		name    string
		handler http.Handler
		want    string
	}{
		{"default instance with Content-Type", http.HandlerFunc(csv), "text/csv"},
		{"default instance without Content-Type", http.HandlerFunc(plain), "application/json"},
		{"plain instance with Content-Type", plainInstance.Handler(http.HandlerFunc(csv)), "text/csv"},
		{"plain instance without Content-Type", plainInstance.Handler(http.HandlerFunc(plain)), "application/json"},
		{"default instance with empty Content-Type", http.HandlerFunc(empty), "application/json"},
		{"plain instance with empty Content-Type", plainInstance.Handler(http.HandlerFunc(empty)), "application/json"},
		{"default instance with handler preset text/html", http.HandlerFunc(preset), "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.handler, http.MethodGet, "", nil)

			if got := rec.Header().Get("Content-Type"); got != tt.want {
				t.Fatalf("Content-Type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetResponseModeTreatsProblemAsUnknown(t *testing.T) {
	usePackageModes(t, CurrentRequestMode, CurrentResponseMode)

	SetResponseMode(ResponseModeProblem)
	rec := serve(http.HandlerFunc(writeTaskNotFound), http.MethodGet, "", nil)

	if CurrentResponseMode != ResponseModeEnvelope {
		t.Fatalf("CurrentResponseMode = %s, want envelope", CurrentResponseMode)
	}
	assertEnvelopeError(t, rec.Header().Get("Content-Type"), rec.Body.Bytes())
}

func TestDefaultInstanceWritesEnvelopeWhenThePackageModeIsProblem(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModeProblem)

	rec := serve(http.HandlerFunc(writeTaskNotFound), http.MethodGet, "", nil)

	assertEnvelopeError(t, rec.Header().Get("Content-Type"), rec.Body.Bytes())
}

func assertEnvelopeError(t *testing.T, contentType string, body []byte) {
	t.Helper()
	if contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if got := responseModeOf(body); got != ResponseModeEnvelope {
		t.Fatalf("response mode = %s, want envelope (%s)", got, body)
	}
}

func TestProblemInstanceAnswersProblem500WhenResponseCannotBeEncoded(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		SuccessWithContent(math.NaN()).Write(w, r)
	}

	rec := serve(problemInstance().Handler(http.HandlerFunc(handler)), http.MethodGet, "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != problemContentType {
		t.Fatalf("Content-Type = %q, want %q", got, problemContentType)
	}
	want := map[string]any{"type": "about:blank", "title": InternalServerTitle, "status": 500.0, "detail": InternalServerMessage, "code": "E300"}
	if got := decodeMembers(t, rec.Body.Bytes()); !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %v, want %v", got, want)
	}
}

func TestProblemEnvelopeAndDefaultInstancesServeConcurrently(t *testing.T) {
	problem := problemInstance().Handler(http.HandlerFunc(writeTaskNotFound))
	envelope := New().Handler(http.HandlerFunc(writeTaskNotFound))
	unwrapped := http.HandlerFunc(writeTaskNotFound)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			rec := serve(problem, http.MethodGet, "", nil)
			if got := rec.Header().Get("Content-Type"); got != problemContentType {
				t.Errorf("problem Content-Type = %q", got)
			}
		})
		wg.Go(func() {
			rec := serve(envelope, http.MethodGet, "", nil)
			if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
				t.Errorf("instance response mode = %s, want envelope", got)
			}
		})
		wg.Go(func() {
			rec := serve(unwrapped, http.MethodGet, "", nil)
			if got := responseModeOf(rec.Body.Bytes()); got != ResponseModeEnvelope {
				t.Errorf("default response mode = %s, want envelope", got)
			}
		})
	}
	wg.Wait()
}

func TestEncodeFailureSendsTheFormattersContentType(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		Success{Content: math.NaN(), Headers: map[string]string{"content-type": "text/csv", "X-Trace": "t"}}.Write(w, r)
	}
	tests := []struct {
		name string
		a    *API
		want string
	}{
		{"envelope instance", New(), "application/json"},
		{"problem instance", problemInstance(), problemContentType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.a.Handler(http.HandlerFunc(handler)), http.MethodGet, "", nil)

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.want {
				t.Fatalf("Content-Type = %q, want %q", got, tt.want)
			}
			if got := rec.Header().Get("X-Trace"); got != "t" {
				t.Fatalf("X-Trace = %q, want t", got)
			}
		})
	}
}
