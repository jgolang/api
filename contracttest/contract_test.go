// Package contracttest checks that the responses api really sends match the
// OpenAPI spec it generates for them.
//
// It is a nested, test-only module so that its OpenAPI validator
// (kin-openapi) never reaches api's go.mod. It never imports api: the root
// module's tests write, per instance variant, the generated spec to
// ../testdata/openapi/complete-route.<variant>.json and the real request and
// response exchanges to ../testdata/contract/complete-route.<variant>.json,
// for the envelope, none, plain and problem modes and a custom formatter, and
// this package only reads those files. Refresh them, and the root module's
// other golden files, from the root module with: go test -update .
package contracttest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// modes are the instance variants the root module records: the four
// response modes and an envelope instance with a custom formatter.
var modes = []string{"envelope", "none", "plain", "problem", "custom-formatter"}

// errBodyWithoutContent reports a response that sends a body for a status
// the spec documents without content. ValidateResponse accepts any body in
// that case, so the harness checks it itself.
var errBodyWithoutContent = errors.New("body sent for a response documented without content")

// knownGaps are spec-vs-wire mismatches the generator cannot describe yet,
// keyed by "<mode>/<exchange name>", with a substring of the error each one
// must fail with. A gap that stops failing fails the test, so it is removed
// here once fixed. It is empty: (*api.API).DocSuccess and DocError document
// the bodies of every mode, including the JSON null that none mode writes
// for an error and the problem details problem mode writes under
// application/problem+json.
var knownGaps = map[string]string{}

// recordedExchange mirrors the root module's recording format. Headers are
// the documented response headers the instance sent, besides Content-Type.
// Body holds the response JSON as is, or a JSON string when the response was
// not JSON. Example, when set, names the named example the body must equal.
type recordedExchange struct {
	Name               string            `json:"name"`
	Method             string            `json:"method"`
	Path               string            `json:"path"`
	RequestContentType string            `json:"requestContentType"`
	RequestBody        string            `json:"requestBody"`
	Status             int               `json:"status"`
	ContentType        string            `json:"contentType"`
	Headers            map[string]string `json:"headers"`
	Body               json.RawMessage   `json:"body"`
	Example            string            `json:"example"`
}

// contract is one mode's spec, ready to validate against, and its recorded
// exchanges.
type contract struct {
	router    routers.Router
	exchanges []recordedExchange
}

func loadContract(t *testing.T, mode string) contract {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromFile(fmt.Sprintf("../testdata/openapi/complete-route.%s.json", mode))
	if err != nil {
		t.Fatalf("load %s spec: %v", mode, err)
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("%s spec is not valid OpenAPI: %v", mode, err)
	}
	makeSchemasStrict(spec)
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("build %s router: %v", mode, err)
	}

	recording, err := os.ReadFile(fmt.Sprintf("../testdata/contract/complete-route.%s.json", mode))
	if err != nil {
		t.Fatalf("read %s exchanges: %v", mode, err)
	}
	var exchanges []recordedExchange
	if err := json.Unmarshal(recording, &exchanges); err != nil {
		t.Fatalf("decode %s exchanges: %v", mode, err)
	}
	if len(exchanges) == 0 {
		t.Fatalf("no %s exchanges recorded", mode)
	}
	return contract{router: router, exchanges: exchanges}
}

// makeSchemasStrict closes every object schema that lists properties and
// says nothing about additional ones. The generator emits open objects,
// which is valid OpenAPI, but a contract that lets undeclared fields through
// would not catch a field the wire sends and the spec omits.
func makeSchemasStrict(spec *openapi3.T) {
	seen := make(map[*openapi3.Schema]bool)
	for _, schema := range spec.Components.Schemas {
		makeSchemaStrict(schema, seen)
	}
	for _, path := range spec.Paths.Map() {
		for _, operation := range path.Operations() {
			if operation.RequestBody != nil && operation.RequestBody.Value != nil {
				for _, media := range operation.RequestBody.Value.Content {
					makeSchemaStrict(media.Schema, seen)
				}
			}
			for _, response := range operation.Responses.Map() {
				if response.Value == nil {
					continue
				}
				for _, media := range response.Value.Content {
					makeSchemaStrict(media.Schema, seen)
				}
			}
		}
	}
}

func makeSchemaStrict(ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
	if ref == nil || ref.Value == nil || seen[ref.Value] {
		return
	}
	schema := ref.Value
	seen[schema] = true
	if len(schema.Properties) > 0 && schema.AdditionalProperties.Has == nil && schema.AdditionalProperties.Schema == nil {
		closed := false
		schema.AdditionalProperties.Has = &closed
	}
	for _, property := range schema.Properties {
		makeSchemaStrict(property, seen)
	}
	makeSchemaStrict(schema.Items, seen)
	makeSchemaStrict(schema.AdditionalProperties.Schema, seen)
	for _, group := range []openapi3.SchemaRefs{schema.AllOf, schema.AnyOf, schema.OneOf} {
		for _, member := range group {
			makeSchemaStrict(member, seen)
		}
	}
}

// validateExchange checks the recorded response against the spec: status,
// media type and body, plus that a status documented without content sends
// no body.
func validateExchange(router routers.Router, exchange recordedExchange) error {
	request := httptest.NewRequest(exchange.Method, exchange.Path, strings.NewReader(exchange.RequestBody))
	request.Header.Set("Content-Type", exchange.RequestContentType)
	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		return fmt.Errorf("find route: %w", err)
	}

	body := responseBody(exchange.Body)
	header := http.Header{"Content-Type": {exchange.ContentType}}
	for name, value := range exchange.Headers {
		header.Set(name, value)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    request,
			PathParams: pathParams,
			Route:      route,
			Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status:  exchange.Status,
		Header:  header,
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	input.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		return err
	}

	response := route.Operation.Responses.Status(exchange.Status)
	if response == nil {
		response = route.Operation.Responses.Default()
	}
	if response != nil && response.Value != nil && len(response.Value.Content) == 0 && len(body) > 0 {
		return fmt.Errorf("status %d: %w: %s", exchange.Status, errBodyWithoutContent, body)
	}
	return nil
}

// responseBody returns the bytes the instance sent. The recorder stores a
// non-JSON body as a JSON string; any other value, null included, is the
// JSON body itself.
func responseBody(recorded json.RawMessage) []byte {
	var text string
	if strings.HasPrefix(string(recorded), `"`) && json.Unmarshal(recorded, &text) == nil {
		return []byte(text)
	}
	return recorded
}

func TestRecordedResponsesMatchSpec(t *testing.T) {
	seenGaps := make(map[string]bool)
	for _, mode := range modes {
		contract := loadContract(t, mode)
		for _, exchange := range contract.exchanges {
			name := mode + "/" + exchange.Name
			t.Run(name, func(t *testing.T) {
				err := validateExchange(contract.router, exchange)

				wantGap, isKnownGap := knownGaps[name]
				seenGaps[name] = isKnownGap
				switch {
				case isKnownGap && err == nil:
					t.Fatalf("known gap %q no longer fails: remove it from knownGaps", name)
				case isKnownGap && !strings.Contains(err.Error(), wantGap):
					t.Fatalf("known gap %q fails with %v, want an error containing %q", name, err, wantGap)
				case !isKnownGap && err != nil:
					t.Fatalf("recorded response does not match the spec: %v", err)
				}
			})
		}
	}
	// A gap whose exchange is no longer recorded would otherwise pass unseen.
	for name := range knownGaps {
		if !seenGaps[name] {
			t.Errorf("known gap %q matches no recorded exchange: remove or rename it in knownGaps", name)
		}
	}
}

// TestDocumentedExamplesMatchRecordedBodies checks that every documented
// response example equals the body the instance sent for that status, under
// the media type it sent, so an example cannot drift from the wire while
// still matching its schema. Each recording must compare at least one.
func TestDocumentedExamplesMatchRecordedBodies(t *testing.T) {
	for _, mode := range modes {
		contract := loadContract(t, mode)
		compared := 0
		for _, exchange := range contract.exchanges {
			examples := documentedExamples(t, contract, exchange)
			if len(examples) == 0 {
				continue
			}
			compared++
			t.Run(mode+"/"+exchange.Name, func(t *testing.T) {
				if err := checkExample(examples, exchange); err != nil {
					t.Fatal(err)
				}
			})
		}
		if compared == 0 {
			t.Errorf("no %s exchange has a documented example", mode)
		}
	}
}

// checkExample reports whether examples, by media type, document the body
// of exchange under the media type it was sent with.
func checkExample(examples map[string]any, exchange recordedExchange) error {
	example, ok := examples[exchange.ContentType]
	if !ok {
		return fmt.Errorf("status %d documents examples only under other media types than the sent %q", exchange.Status, exchange.ContentType)
	}
	var body any
	if err := json.Unmarshal(responseBody(exchange.Body), &body); err != nil {
		return fmt.Errorf("decode body %s: %w", exchange.Body, err)
	}
	if !reflect.DeepEqual(example, body) {
		return fmt.Errorf("documented example = %v, recorded body = %s", example, exchange.Body)
	}
	return nil
}

// TestExampleCheckRejectsAnExampleUnderAnotherMediaType proves the example
// check compares media types: an exchange whose example is documented only
// under a media type other than the one sent must fail, even though the
// example equals the body.
func TestExampleCheckRejectsAnExampleUnderAnotherMediaType(t *testing.T) {
	for _, mode := range modes {
		contract := loadContract(t, mode)
		checked := 0
		for _, exchange := range contract.exchanges {
			examples := documentedExamples(t, contract, exchange)
			if len(examples) == 0 {
				continue
			}
			checked++
			t.Run(mode+"/"+exchange.Name, func(t *testing.T) {
				exchange.ContentType = "application/vnd.other+json"

				err := checkExample(examples, exchange)

				if err == nil || !strings.Contains(err.Error(), "only under other media types") {
					t.Fatalf("check error = %v, want it to reject the example under another media type", err)
				}
			})
		}
		if checked == 0 {
			t.Errorf("no %s exchange has a documented example", mode)
		}
	}
}

// documentedExamples returns, by media type, the example the spec documents
// for the status of exchange: the named example exchange.Example when it is
// set, failing when a media type lacks it, else the single example.
func documentedExamples(t *testing.T, contract contract, exchange recordedExchange) map[string]any {
	t.Helper()
	request := httptest.NewRequest(exchange.Method, exchange.Path, nil)
	route, _, err := contract.router.FindRoute(request)
	if err != nil {
		t.Fatalf("find route of %q: %v", exchange.Name, err)
	}
	response := route.Operation.Responses.Status(exchange.Status)
	if response == nil || response.Value == nil {
		return nil
	}
	examples := make(map[string]any)
	for mediaType, media := range response.Value.Content {
		if media == nil {
			continue
		}
		if exchange.Example != "" {
			named, ok := media.Examples[exchange.Example]
			if !ok || named == nil || named.Value == nil {
				t.Fatalf("status %d under %q has no example %q", exchange.Status, mediaType, exchange.Example)
			}
			examples[mediaType] = named.Value.Value
			continue
		}
		if media.Example != nil {
			examples[mediaType] = media.Example
		}
	}
	return examples
}

// TestValidatorRejectsMismatches proves the harness fails on drift: each
// case breaks each success exchange in one way that the spec does not allow.
func TestValidatorRejectsMismatches(t *testing.T) {
	tests := []struct {
		name      string
		modes     []string
		mutate    func(t *testing.T, exchange *recordedExchange)
		wantError string
	}{
		{
			name:  "undeclared top-level field",
			modes: modes,
			mutate: func(t *testing.T, exchange *recordedExchange) {
				exchange.Body = withField(t, exchange.Body, nil, "unexpected", true)
			},
			wantError: `property "unexpected" is unsupported`,
		},
		{
			name:  "undeclared content field",
			modes: []string{"envelope", "plain"},
			mutate: func(t *testing.T, exchange *recordedExchange) {
				exchange.Body = withField(t, exchange.Body, []string{"content"}, "unexpected", true)
			},
			wantError: `property "unexpected" is unsupported`,
		},
		{
			name:  "wrong media type",
			modes: modes,
			mutate: func(t *testing.T, exchange *recordedExchange) {
				exchange.ContentType = "text/plain"
			},
			wantError: "Content-Type has unexpected value",
		},
		{
			name:  "undocumented status",
			modes: modes,
			mutate: func(t *testing.T, exchange *recordedExchange) {
				exchange.Status = http.StatusTeapot
			},
			wantError: "status is not supported",
		},
	}
	for _, test := range tests {
		for _, mode := range test.modes {
			for _, exchangeName := range mutatedExchanges {
				t.Run(mode+"/"+exchangeName+"/"+test.name, func(t *testing.T) {
					contract := loadContract(t, mode)
					exchange := exchangeNamed(t, contract.exchanges, exchangeName)
					test.mutate(t, &exchange)

					err := validateExchange(contract.router, exchange)

					if err == nil || !strings.Contains(err.Error(), test.wantError) {
						t.Fatalf("validation error = %v, want one containing %q", err, test.wantError)
					}
				})
			}
		}
	}
}

// mutatedExchanges are the success exchanges the mismatch cases break: the
// 201 documented with (*api.API).DocSuccessExample and the 200 documented
// with (*api.API).DocSuccess, so both kinds of documented body must reject
// drift.
var mutatedExchanges = []string{"valid task", "found task"}

// exchangeNamed returns the recorded exchange called name.
func exchangeNamed(t *testing.T, exchanges []recordedExchange, name string) recordedExchange {
	t.Helper()
	for _, exchange := range exchanges {
		if exchange.Name == name {
			return exchange
		}
	}
	t.Fatalf("no %q exchange recorded", name)
	return recordedExchange{}
}

// withField returns body with key set to value in the object found by
// following parents from the top level.
func withField(t *testing.T, body json.RawMessage, parents []string, key string, value any) json.RawMessage {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatalf("decode body %s: %v", body, err)
	}
	object := root
	for _, parent := range parents {
		child, ok := object[parent].(map[string]any)
		if !ok {
			t.Fatalf("body %s has no object at %q", body, parent)
		}
		object = child
	}
	object[key] = value
	changed, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	return changed
}
