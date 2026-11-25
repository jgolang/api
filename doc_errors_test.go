package api_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

var (
	docErrorsNotFound   = codes.New("TSK-404", "task not found").WithKind(codes.KindNotFound)
	docErrorsConflict   = codes.New("TSK-409", "task is locked").WithKind(codes.KindConflict)
	docErrorsNoKind     = codes.New("TSK-500", "task store is unavailable")
	docErrorsTitle      = codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)
	docErrorsPriority   = codes.New("TSK-002", "priority is unknown").WithKind(codes.KindValidation)
	docErrorsNoMessage  = codes.New("TSK-403", "").WithKind(codes.KindForbidden)
	docErrorsSameString = codes.New("TSK-404", "another message").WithKind(codes.KindConflict)
)

// documentedErrors applies option to an empty route and returns its
// responses in the order the option wrote them.
func documentedErrors(option doc.RouteOption) []doc.RouteResponse {
	var route doc.Route
	option(&route)
	return route.Responses
}

// documentedStatuses returns the statuses of responses, in order.
func documentedStatuses(responses []doc.RouteResponse) []int {
	statuses := make([]int, 0, len(responses))
	for _, response := range responses {
		statuses = append(statuses, response.Status)
	}
	return statuses
}

// responseForStatus returns the response documented for status.
func responseForStatus(t *testing.T, responses []doc.RouteResponse, status int) doc.RouteResponse {
	t.Helper()
	for _, response := range responses {
		if response.Status == status {
			return response
		}
	}
	t.Fatalf("status %d is not documented; statuses %v", status, documentedStatuses(responses))
	return doc.RouteResponse{}
}

// namedExample returns the example documented under name in response.
func namedExample(t *testing.T, response doc.RouteResponse, name string) doc.ExampleObject {
	t.Helper()
	example, ok := response.Examples[name]
	if !ok {
		t.Fatalf("status %d has no example %q; examples %v", response.Status, name, response.Examples)
	}
	object, ok := example.(doc.ExampleObject)
	if !ok {
		t.Fatalf("example %q is %T, want doc.ExampleObject", name, example)
	}
	return object
}

// writtenErrorBody serves WriteError for err through handler (an instance's
// Handler, or a plain handler on the default instance) and returns the
// status, Content-Type and decoded body.
func writtenErrorBody(t *testing.T, wrap func(http.Handler) http.Handler, err error) (status int, contentType string, body any) {
	t.Helper()
	handler := wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, err)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/tasks/1", nil))

	return recorder.Code, recorder.Header().Get("Content-Type"), decodedJSON(t, recorder.Body.Bytes())
}

// exampleJSON returns value decoded from its JSON, so an example decoded
// with UseNumber compares with a body decoded into float64.
func exampleJSON(t *testing.T, value any) any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}
	return decodedJSON(t, encoded)
}

func TestDocErrorsGroupsCodesByStatusInAscendingOrder(t *testing.T) {
	option := api.New().DocErrors(docErrorsNoKind, docErrorsConflict, docErrorsNotFound)

	responses := documentedErrors(option)

	if got, want := documentedStatuses(responses), []int{404, 409, 500}; !slices.Equal(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	for _, name := range []string{"TSK-404", "TSK-409", "TSK-500"} {
		found := false
		for _, response := range responses {
			if _, ok := response.Examples[name]; ok {
				found = true
			}
		}
		if !found {
			t.Errorf("no response has the example %q", name)
		}
	}
}

func TestDocErrorsPutsCodesOfOneStatusInOneResponse(t *testing.T) {
	responses := documentedErrors(api.New().DocErrors(docErrorsTitle, docErrorsPriority))

	if len(responses) != 1 || responses[0].Status != http.StatusBadRequest {
		t.Fatalf("statuses = %v, want [400]", documentedStatuses(responses))
	}
	if len(responses[0].Examples) != 2 {
		t.Fatalf("examples = %v, want TSK-001 and TSK-002", responses[0].Examples)
	}
	namedExample(t, responses[0], "TSK-001")
	namedExample(t, responses[0], "TSK-002")
}

func TestDocErrorsFollowsTheInstanceKindStatus(t *testing.T) {
	instance := api.New(api.WithKindStatus(codes.KindConflict, http.StatusUnprocessableEntity))

	responses := documentedErrors(instance.DocErrors(docErrorsConflict))
	status, _, body := writtenErrorBody(t, instance.Handler, errors.NewC(docErrorsConflict, "locked by job 7"))

	if got := documentedStatuses(responses); !slices.Equal(got, []int{http.StatusUnprocessableEntity}) {
		t.Fatalf("statuses = %v, want [422]", got)
	}
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("wire status = %d, want 422", status)
	}
	example := namedExample(t, responses[0], "TSK-409")
	if got := exampleJSON(t, example.Value); !reflect.DeepEqual(got, body) {
		t.Fatalf("example = %v, wire body = %v", got, body)
	}
}

func TestDocErrorsNamesEachExampleByCodeWithTheMessageAsSummary(t *testing.T) {
	tests := []struct {
		name        string
		code        codes.Code
		status      int
		wantSummary string
	}{
		{name: "message", code: docErrorsNotFound, status: http.StatusNotFound, wantSummary: "task not found"},
		{name: "no message", code: docErrorsNoMessage, status: http.StatusForbidden, wantSummary: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responses := documentedErrors(api.New().DocErrors(test.code))

			example := namedExample(t, responseForStatus(t, responses, test.status), test.code.Str())

			if example.Summary != test.wantSummary {
				t.Fatalf("summary = %q, want %q", example.Summary, test.wantSummary)
			}
		})
	}
}

func TestDocErrorsExampleIsTheBodyWriteErrorSends(t *testing.T) {
	for _, variant := range completeRouteVariants {
		for _, code := range []codes.Code{docErrorsNotFound, docErrorsConflict, docErrorsNoKind, docErrorsNoMessage} {
			t.Run(variant.name+"/"+code.Str(), func(t *testing.T) {
				instance := api.New(variant.options...)

				responses := documentedErrors(instance.DocErrors(code))
				status, contentType, body := writtenErrorBody(t, instance.Handler, errors.NewC(code, "internal detail"))

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
				example := namedExample(t, response, code.Str())
				if got := exampleJSON(t, example.Value); !reflect.DeepEqual(got, body) {
					t.Fatalf("example = %v, wire body = %v", got, body)
				}
			})
		}
	}
}

func TestDocErrorsDocumentsTheModeBody(t *testing.T) {
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
			option := api.New(api.WithResponseMode(test.mode)).DocErrors(docErrorsNotFound)

			description, schema, hasExample := documentedResponse(t, "404", option)

			if description != "Not Found" {
				t.Errorf("description = %q, want the status text", description)
			}
			if schema != test.wantSchema {
				t.Errorf("schema = %s, want %s", schema, test.wantSchema)
			}
			if hasExample != test.wantExample {
				t.Errorf("404 has example = %t, want %t", hasExample, test.wantExample)
			}
			if mediaType := documentedMediaType(t, "404", option); mediaType != test.wantMediaType {
				t.Errorf("media type = %q, want %q", mediaType, test.wantMediaType)
			}
		})
	}
}

func TestDocErrorsLeavesTheNullBodyOfNoneModeWithoutExample(t *testing.T) {
	responses := documentedErrors(api.New(api.WithResponseMode(api.ResponseModeNone)).DocErrors(docErrorsNotFound, docErrorsConflict))

	if got := documentedStatuses(responses); !slices.Equal(got, []int{http.StatusNotFound, http.StatusConflict}) {
		t.Fatalf("statuses = %v, want [404 409]", got)
	}
	for _, response := range responses {
		if response.Examples != nil {
			t.Errorf("%d examples = %v, want none for a JSON null body", response.Status, response.Examples)
		}
	}
}

func TestDocErrorsKeepsTheFirstOfDuplicateCodes(t *testing.T) {
	responses := documentedErrors(api.New().DocErrors(docErrorsNotFound, docErrorsNotFound, docErrorsSameString))

	// docErrorsSameString is sent as 409, so its status is documented, but
	// without the example its repeated name already has under 404.
	if got := documentedStatuses(responses); !slices.Equal(got, []int{http.StatusNotFound, http.StatusConflict}) {
		t.Fatalf("statuses = %v, want [404 409]", got)
	}
	if len(responses[0].Examples) != 1 {
		t.Fatalf("examples = %v, want only TSK-404", responses[0].Examples)
	}
	if summary := namedExample(t, responses[0], "TSK-404").Summary; summary != "task not found" {
		t.Fatalf("summary = %q, want the first code's message", summary)
	}
}

func TestDocErrorsDocumentsEveryStatusOfCodesWithTheSameName(t *testing.T) {
	// Both codes have an empty Str(), so both are named by the default
	// error code, but WriteError sends each one with its own status.
	notFound := codes.New("", "task not found").WithKind(codes.KindNotFound)
	conflict := codes.New("", "task is locked").WithKind(codes.KindConflict)
	instance := api.New()

	responses := documentedErrors(instance.DocErrors(notFound, conflict))

	var wireStatuses []int
	for _, code := range []codes.Code{notFound, conflict} {
		status, _, _ := writtenErrorBody(t, instance.Handler, errors.NewC(code, "internal detail"))
		wireStatuses = append(wireStatuses, status)
	}
	if got := documentedStatuses(responses); !slices.Equal(got, wireStatuses) {
		t.Fatalf("documented statuses = %v, wire statuses = %v", got, wireStatuses)
	}
	name := string(api.ResponseCodes.DefaultError)
	namedExample(t, responseForStatus(t, responses, http.StatusNotFound), name)
	if examples := responseForStatus(t, responses, http.StatusConflict).Examples; len(examples) != 0 {
		t.Fatalf("409 examples = %v, want none: a repeated name keeps only its first example", examples)
	}
}

func TestDocErrorsDocumentsANilCoderAsAnUncodedError(t *testing.T) {
	instance := api.New()

	responses := documentedErrors(instance.DocErrors(nil))
	_, _, body := writtenErrorBody(t, instance.Handler, nil)

	response := responseForStatus(t, responses, http.StatusInternalServerError)
	example := namedExample(t, response, string(api.ResponseCodes.InternalServerError))
	if got := exampleJSON(t, example.Value); !reflect.DeepEqual(got, body) {
		t.Fatalf("example = %v, wire body = %v", got, body)
	}
}

func TestDocErrorsDocumentsANilPointerCoderAsAnUncodedError(t *testing.T) {
	responses := documentedErrors(api.New().DocErrors((*codes.Code)(nil)))

	response := responseForStatus(t, responses, http.StatusInternalServerError)
	namedExample(t, response, string(api.ResponseCodes.InternalServerError))
}

func TestDocErrorsWithoutCodesAddsNoResponse(t *testing.T) {
	responses := documentedErrors(api.New().DocErrors())

	if len(responses) != 0 {
		t.Fatalf("responses = %v, want none", responses)
	}
}

func TestDocErrorsOfTheDefaultInstance(t *testing.T) {
	var instance *api.API
	withoutInstance := func(handler http.Handler) http.Handler { return handler }

	responses := documentedErrors(instance.DocErrors(docErrorsConflict))
	status, _, body := writtenErrorBody(t, withoutInstance, errors.NewC(docErrorsConflict, "locked"))

	if status != http.StatusConflict {
		t.Fatalf("wire status = %d, want 409", status)
	}
	example := namedExample(t, responseForStatus(t, responses, http.StatusConflict), "TSK-409")
	if got := exampleJSON(t, example.Value); !reflect.DeepEqual(got, body) {
		t.Fatalf("example = %v, wire body = %v", got, body)
	}
}

func TestDocErrorsLeavesACodeWithoutBodyWithoutExample(t *testing.T) {
	tests := []struct {
		name      string
		formatter fixedFormatter
	}{
		{name: "nil formatted response", formatter: fixedFormatter{nilResponse: true}},
		{name: "unencodable body", formatter: fixedFormatter{body: math.NaN()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := api.New(api.WithResponseFormatter(test.formatter)).DocErrors(docErrorsNotFound)

			_, schema, hasExample := documentedResponse(t, "404", option)

			if schema != errorResponseRef {
				t.Errorf("schema = %s, want %s", schema, errorResponseRef)
			}
			if hasExample {
				t.Errorf("404 has an example, want none")
			}
		})
	}
}

func TestDocErrorsDoesNotCallTheErrorHook(t *testing.T) {
	calls := 0
	instance := api.New(api.WithErrorHook(func(*http.Request, error, int) { calls++ }))

	instance.DocErrors(docErrorsNotFound, docErrorsConflict)

	if calls != 0 {
		t.Fatalf("error hook called %d times, want 0", calls)
	}
}
