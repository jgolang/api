package api_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
)

// libraryRejectionCase is one way the library's middlewares answer a request
// on their own, and the named example DocRejections documents for it.
type libraryRejectionCase struct {
	name       string
	example    string
	status     int
	middleware func(http.HandlerFunc) http.HandlerFunc
	handler    http.HandlerFunc
	request    func() *http.Request
	options    []api.Option
}

func jsonRequest(body string) func() *http.Request {
	return func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
}

func requestWithAuthorization(authorization string) func() *http.Request {
	return func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
		if authorization != "" {
			r.Header.Set("Authorization", authorization)
		}
		return r
	}
}

func unreachedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func panickingHandler(http.ResponseWriter, *http.Request) {
	panic("task store is unavailable")
}

var libraryRejectionCases = []libraryRejectionCase{
	{name: "ProcessRequest unparsable body", example: "invalidRequestContent", status: http.StatusBadRequest,
		middleware: api.ProcessRequest, handler: unreachedHandler, request: jsonRequest("{")},
	{name: "RequestBody unparsable body", example: "invalidRequestContent", status: http.StatusBadRequest,
		middleware: api.RequestBody, handler: unreachedHandler, request: jsonRequest("{")},
	{name: "ProcessRequest body too large", example: "requestContentTooLarge", status: http.StatusRequestEntityTooLarge,
		middleware: api.ProcessRequest, handler: unreachedHandler, request: jsonRequest(`{"title":"a title longer than sixteen bytes"}`),
		options: []api.Option{api.WithMaxBodyBytes(16)}},
	{name: "RequestBody body too large", example: "requestContentTooLarge", status: http.StatusRequestEntityTooLarge,
		middleware: api.RequestBody, handler: unreachedHandler, request: jsonRequest(`{"title":"a title longer than sixteen bytes"}`),
		options: []api.Option{api.WithMaxBodyBytes(16)}},
	{name: "BasicToken without Authorization", example: "invalidAuthorizationHeader", status: http.StatusUnauthorized,
		middleware: api.BasicToken, handler: unreachedHandler, request: requestWithAuthorization("")},
	{name: "CustomToken without Authorization", example: "invalidAuthorizationHeader", status: http.StatusUnauthorized,
		middleware: api.CustomToken, handler: unreachedHandler, request: requestWithAuthorization("")},
	{name: "BasicToken bad credentials", example: "invalidBasicToken", status: http.StatusUnauthorized,
		middleware: api.BasicToken, handler: unreachedHandler, request: requestWithAuthorization("Basic dXNlcjpwYXNz")},
	{name: "CustomToken bad token", example: "invalidBearerToken", status: http.StatusUnauthorized,
		middleware: api.CustomToken, handler: unreachedHandler, request: requestWithAuthorization("Bearer not-a-token")},
	{name: "RequestHeaderSession without session", example: "invalidSessionID", status: http.StatusUnauthorized,
		middleware: api.RequestHeaderSession, handler: unreachedHandler, request: requestWithAuthorization("")},
	{name: "ProcessRequest handler panic", example: "internalServerError", status: http.StatusInternalServerError,
		middleware: api.ProcessRequest, handler: panickingHandler, request: jsonRequest(`{}`)},
}

// quietLogger keeps the panic and body logs of the rejections out of the
// test output.
var quietLogger = api.WithLogger(slog.New(slog.DiscardHandler))

// servedRejection serves test's request through test's middleware, wrapped
// by wrap, and returns the status, Content-Type and decoded body sent.
func servedRejection(t *testing.T, wrap func(http.Handler) http.Handler, test libraryRejectionCase) (status int, contentType string, body any) {
	t.Helper()
	handler := wrap(test.middleware(test.handler))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, test.request())

	return recorder.Code, recorder.Header().Get("Content-Type"), decodedJSON(t, recorder.Body.Bytes())
}

// eventIDTrace matches the event ID part of a message's trace suffix, as in
// "(abc123-E300)", which a documented example has empty.
var eventIDTrace = regexp.MustCompile(`\([^()\s-]+-([^()\s]+)\)$`)

// withoutEventID returns body without its event_id keys and with the event
// ID removed from the trace suffix of its message and detail, since the
// documented examples have no request and so no event ID.
func withoutEventID(body any) any {
	switch value := body.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(value))
		for key, item := range value {
			switch key {
			case "event_id":
				continue
			case "message", "detail":
				if text, ok := item.(string); ok {
					item = eventIDTrace.ReplaceAllString(text, "(-$1)")
				}
			}
			cleaned[key] = withoutEventID(item)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(value))
		for i, item := range value {
			cleaned[i] = withoutEventID(item)
		}
		return cleaned
	}
	return body
}

// assertExampleIsTheWireBody checks that the example responses document for
// test is the body that was served for it.
func assertExampleIsTheWireBody(t *testing.T, responses []doc.RouteResponse, test libraryRejectionCase, status int, contentType string, body any) {
	t.Helper()
	if status != test.status {
		t.Fatalf("wire status = %d, want %d", status, test.status)
	}
	response := responseForStatus(t, responses, status)
	if body == nil {
		// None mode sends JSON null, which is left without an example.
		if len(response.Examples) != 0 {
			t.Fatalf("examples = %v for a null body, want none", response.Examples)
		}
		return
	}
	if documented := response.ContentType; documented != "" && documented != contentType {
		t.Errorf("documented media type = %q, wire = %q", documented, contentType)
	}
	example := namedExample(t, response, test.example)
	if got, want := withoutEventID(exampleJSON(t, example.Value)), withoutEventID(body); !reflect.DeepEqual(got, want) {
		t.Fatalf("example = %v, wire body = %v", got, want)
	}
}

func TestDocRejectionsDocumentsFourStatusesInAscendingOrder(t *testing.T) {
	responses := documentedErrors(api.New().DocRejections())

	if got, want := documentedStatuses(responses), []int{400, 401, 413, 500}; !slices.Equal(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	for _, response := range responses {
		if response.Description != "" {
			t.Errorf("%d description = %q, want empty for the status text", response.Status, response.Description)
		}
	}
}

func TestDocRejectionsNamesOneExamplePerBody(t *testing.T) {
	responses := documentedErrors(api.New().DocRejections())

	want := map[int][]string{
		400: {"invalidRequestContent"},
		401: {"invalidAuthorizationHeader", "invalidBasicToken", "invalidBearerToken", "invalidSessionID"},
		413: {"requestContentTooLarge"},
		500: {"internalServerError"},
	}
	for _, response := range responses {
		var names []string
		for name := range response.Examples {
			names = append(names, name)
		}
		slices.Sort(names)
		if !slices.Equal(names, want[response.Status]) {
			t.Errorf("%d examples = %v, want %v", response.Status, names, want[response.Status])
		}
		for _, name := range names {
			if namedExample(t, response, name).Summary == "" {
				t.Errorf("%d example %q has no summary", response.Status, name)
			}
		}
	}
}

func TestDocRejectionsExampleIsTheBodyTheLibrarySends(t *testing.T) {
	for _, variant := range completeRouteVariants {
		for _, test := range libraryRejectionCases {
			t.Run(variant.name+"/"+test.name, func(t *testing.T) {
				options := append(slices.Clone(variant.options), quietLogger)
				instance := api.New(append(options, test.options...)...)

				responses := documentedErrors(instance.DocRejections())
				status, contentType, body := servedRejection(t, instance.Handler, test)

				assertExampleIsTheWireBody(t, responses, test, status, contentType, body)
			})
		}
	}
}

func TestDocRejectionsOfTheDefaultInstance(t *testing.T) {
	var instance *api.API
	withoutInstance := func(handler http.Handler) http.Handler { return handler }

	responses := documentedErrors(instance.DocRejections())

	for _, test := range libraryRejectionCases {
		if test.options != nil {
			// The body limit is an instance option.
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			status, contentType, body := servedRejection(t, withoutInstance, test)

			assertExampleIsTheWireBody(t, responses, test, status, contentType, body)
		})
	}
}

func TestDocRejectionsDocumentsTheModeBody(t *testing.T) {
	tests := []struct {
		name          string
		mode          api.ResponseMode
		wantSchema    string
		wantMediaType string
		wantExample   bool
	}{
		{name: "envelope", wantExample: true, mode: api.ResponseModeEnvelope, wantSchema: errorResponseRef, wantMediaType: "application/json"},
		{name: "plain", wantExample: true, mode: api.ResponseModePlain, wantSchema: plainErrorRef, wantMediaType: "application/json"},
		{name: "none", mode: api.ResponseModeNone, wantSchema: nullBodySchema, wantMediaType: "application/json"},
		{name: "problem", wantExample: true, mode: api.ResponseModeProblem, wantSchema: problemDetailsRef, wantMediaType: "application/problem+json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := api.New(api.WithResponseMode(test.mode)).DocRejections()

			for _, status := range []string{"400", "401", "413", "500"} {
				description, schema, hasExample := documentedResponse(t, status, option)

				if description == "" {
					t.Errorf("%s description is empty, want the status text", status)
				}
				if schema != test.wantSchema {
					t.Errorf("%s schema = %s, want %s", status, schema, test.wantSchema)
				}
				if hasExample != test.wantExample {
					t.Errorf("%s has example = %t, want %t", status, hasExample, test.wantExample)
				}
				if mediaType := documentedMediaType(t, status, option); mediaType != test.wantMediaType {
					t.Errorf("%s media type = %q, want %q", status, mediaType, test.wantMediaType)
				}
			}
		})
	}
}

func TestDocRejectionsReadsTheMessagesWhenCalled(t *testing.T) {
	previous := api.DefaultBasicUnauthorizedMsg
	t.Cleanup(func() { api.DefaultBasicUnauthorizedMsg = previous })
	api.DefaultBasicUnauthorizedMsg = "Basic credentials rejected"

	responses := documentedErrors(api.New(api.WithResponseMode(api.ResponseModeProblem)).DocRejections())

	example := namedExample(t, responseForStatus(t, responses, http.StatusUnauthorized), "invalidBasicToken")
	body, ok := exampleJSON(t, example.Value).(map[string]any)
	if !ok || body["detail"] != "Basic credentials rejected" {
		t.Fatalf("example = %v, want detail %q", example.Value, "Basic credentials rejected")
	}
}

func TestDocRejectionsAddsNothingToARouteWithoutIt(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.Register(http.MethodPost, "/tasks", api.New().DocRejections())
	docs.Register(http.MethodGet, "/health", doc.Status(http.StatusOK, nil))

	responses := doc.GenerateOpenAPI(docs).Paths["/health"]["get"].Responses

	for _, status := range []string{"400", "401", "413", "500"} {
		if _, ok := responses[status]; ok {
			t.Errorf("GET /health documents %s, want only its own responses", status)
		}
	}
}

func TestDocRejectionsIsReplacedByALaterOptionForTheSameStatus(t *testing.T) {
	instance := api.New()

	responses := generatedResponse(t, "400", instance.DocRejections(), instance.DocErrors(docErrorsTitle)).Content["application/json"].Examples

	if _, ok := responses["TSK-001"]; !ok {
		t.Fatalf("400 examples = %v, want the later DocErrors one", responses)
	}
	if _, ok := responses["invalidRequestContent"]; ok {
		t.Fatalf("400 examples = %v, want the DocRejections one replaced", responses)
	}
}
