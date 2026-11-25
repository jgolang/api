package api_test

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/core"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
)

type docOptionsTask struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

const (
	docOptionsTaskRef  = `{"$ref":"#/components/schemas/DocOptionsTask"}`
	envelopeTaskSchema = `{"type":"object","properties":{"content":{"nullable":true,"allOf":[{"$ref":"#/components/schemas/DocOptionsTask"}]},"header":{"$ref":"#/components/schemas/ResponseInfo"}},"required":["header"]}`
	envelopeOnlyHeader = `{"type":"object","properties":{"header":{"$ref":"#/components/schemas/ResponseInfo"}},"required":["header"]}`
	errorResponseRef   = `{"$ref":"#/components/schemas/ErrorResponse"}`
	plainErrorRef      = `{"$ref":"#/components/schemas/PlainErrorResponse"}`
	problemDetailsRef  = `{"$ref":"#/components/schemas/ProblemDetails"}`
	nullBodySchema     = `{"type":"object","enum":[null],"nullable":true}`
)

// documentedResponse returns the description and the JSON of the schema
// that opts document for status, and whether the response has an example.
func documentedResponse(t *testing.T, status string, opts ...doc.RouteOption) (description, schema string, hasExample bool) {
	t.Helper()
	response := generatedResponse(t, status, opts...)
	if len(response.Content) > 1 {
		t.Fatalf("response %s has %d media types, want one", status, len(response.Content))
	}
	for _, media := range response.Content {
		encoded, err := json.Marshal(media.Schema)
		if err != nil {
			t.Fatalf("marshal schema: %v", err)
		}
		return response.Description, string(encoded), media.Example != nil || len(media.Examples) > 0
	}
	return response.Description, "", false
}

// documentedMediaType returns the media type opts document for status, or
// "" for a response without content.
func documentedMediaType(t *testing.T, status string, opts ...doc.RouteOption) string {
	t.Helper()
	for mediaType := range generatedResponse(t, status, opts...).Content {
		return mediaType
	}
	return ""
}

func generatedResponse(t *testing.T, status string, opts ...doc.RouteOption) doc.ResponseObject {
	t.Helper()
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks", opts...)

	response, ok := doc.GenerateOpenAPI(docs).Paths["/tasks"]["get"].Responses[status]
	if !ok {
		t.Fatalf("response %s is not documented", status)
	}
	return response
}

// docTypeSchema returns the JSON of the component schema that the envelope
// doc type body generates, which a wrapper DocSuccess builds at run time
// must equal.
func docTypeSchema(t *testing.T, body any) string {
	t.Helper()
	encoded, err := json.Marshal(documentedBody(t, body))
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	return string(encoded)
}

func TestDocSuccessDocumentsTheInstanceModeBody(t *testing.T) {
	plain := api.New(api.WithResponseMode(api.ResponseModePlain))
	problem := api.New(api.WithResponseMode(api.ResponseModeProblem))
	tests := []struct {
		name       string
		api        *api.API
		status     int
		sample     any
		wantSchema string
	}{
		{name: "envelope", api: api.New(), status: http.StatusOK, sample: docOptionsTask{}, wantSchema: envelopeTaskSchema},
		{name: "envelope pointer sample", api: api.New(), status: http.StatusOK, sample: &docOptionsTask{}, wantSchema: envelopeTaskSchema},
		{name: "envelope nil sample", api: api.New(), status: http.StatusOK, sample: nil, wantSchema: envelopeOnlyHeader},
		{name: "envelope 204 without content", api: api.New(), status: http.StatusNoContent, sample: docOptionsTask{}, wantSchema: ""},
		{name: "none 304 without content", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusNotModified, sample: nil, wantSchema: ""},
		{name: "envelope 1xx without content", api: api.New(), status: http.StatusContinue, sample: docOptionsTask{}, wantSchema: ""},
		{name: "plain", api: plain, status: http.StatusOK, sample: docOptionsTask{}, wantSchema: docTypeSchema(t, envelope.PlainSuccess[docOptionsTask]())},
		{name: "plain pointer sample", api: plain, status: http.StatusOK, sample: &docOptionsTask{}, wantSchema: docTypeSchema(t, envelope.PlainSuccess[docOptionsTask]())},
		{name: "plain nil sample", api: plain, status: http.StatusOK, sample: nil, wantSchema: docTypeSchema(t, envelope.PlainSuccess[any]())},
		{name: "plain 204 without content", api: plain, status: http.StatusNoContent, sample: docOptionsTask{}, wantSchema: ""},
		{name: "problem 204 without content", api: problem, status: http.StatusNoContent, sample: docOptionsTask{}, wantSchema: ""},
		{name: "none", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: docOptionsTask{}, wantSchema: docOptionsTaskRef},
		{name: "none pointer sample", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: &docOptionsTask{}, wantSchema: docOptionsTaskRef},
		{name: "none nil sample", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: nil, wantSchema: nullBodySchema},
		{name: "problem success as none", api: problem, status: http.StatusOK, sample: docOptionsTask{}, wantSchema: docOptionsTaskRef},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := test.api.DocSuccess(test.status, test.sample)

			_, schema, hasExample := documentedResponse(t, strconv.Itoa(test.status), option)

			if schema != test.wantSchema {
				t.Errorf("schema = %s, want %s", schema, test.wantSchema)
			}
			if mediaType := documentedMediaType(t, strconv.Itoa(test.status), option); schema != "" && mediaType != "application/json" {
				t.Errorf("media type = %q, want application/json", mediaType)
			}
			if hasExample {
				t.Error("response has an example, want schema only")
			}
		})
	}
}

func TestDocErrorDocumentsTheInstanceModeBody(t *testing.T) {
	tests := []struct {
		name            string
		api             *api.API
		status          int
		description     string
		wantDescription string
		wantSchema      string
		wantMediaType   string
	}{
		{name: "envelope", api: api.New(), status: http.StatusNotFound, description: "Task not found", wantDescription: "Task not found", wantSchema: errorResponseRef, wantMediaType: "application/json"},
		{name: "none", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusBadRequest, wantDescription: "Bad Request", wantSchema: nullBodySchema, wantMediaType: "application/json"},
		{name: "plain", api: api.New(api.WithResponseMode(api.ResponseModePlain)), status: http.StatusBadRequest, wantDescription: "Bad Request", wantSchema: plainErrorRef, wantMediaType: "application/json"},
		{name: "problem", api: api.New(api.WithResponseMode(api.ResponseModeProblem)), status: http.StatusNotFound, description: "Task not found", wantDescription: "Task not found", wantSchema: problemDetailsRef, wantMediaType: "application/problem+json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := test.api.DocError(test.status, test.description)

			description, schema, hasExample := documentedResponse(t, strconv.Itoa(test.status), option)

			if description != test.wantDescription {
				t.Errorf("description = %q, want %q", description, test.wantDescription)
			}
			if schema != test.wantSchema {
				t.Errorf("schema = %s, want %s", schema, test.wantSchema)
			}
			if mediaType := documentedMediaType(t, strconv.Itoa(test.status), option); mediaType != test.wantMediaType {
				t.Errorf("media type = %q, want only %q", mediaType, test.wantMediaType)
			}
			if hasExample {
				t.Error("response has an example, want schema only")
			}
		})
	}
}

func TestDocErrorDocumentsAStatusWithoutBodyWithoutContent(t *testing.T) {
	modes := []api.ResponseMode{api.ResponseModeEnvelope, api.ResponseModeNone, api.ResponseModePlain, api.ResponseModeProblem}
	statuses := []int{http.StatusContinue, http.StatusNoContent, http.StatusNotModified}
	for _, mode := range modes {
		for _, status := range statuses {
			t.Run(string(mode)+"/"+strconv.Itoa(status), func(t *testing.T) {
				option := api.New(api.WithResponseMode(mode)).DocError(status, "")

				description, schema, hasExample := documentedResponse(t, strconv.Itoa(status), option)

				if mediaType := documentedMediaType(t, strconv.Itoa(status), option); mediaType != "" {
					t.Errorf("media type = %q (schema %s), want no content", mediaType, schema)
				}
				if hasExample {
					t.Error("response has an example, want none")
				}
				if want := http.StatusText(status); description != want {
					t.Errorf("description = %q, want %q", description, want)
				}
			})
		}
	}
}

func TestDocOptionsOfTheDefaultInstanceReadCurrentResponseMode(t *testing.T) {
	saved := api.CurrentResponseMode
	t.Cleanup(func() { api.CurrentResponseMode = saved })
	api.CurrentResponseMode = api.ResponseModeNone
	var defaultInstance *api.API

	success := defaultInstance.DocSuccess(http.StatusOK, docOptionsTask{})
	failure := defaultInstance.DocError(http.StatusBadRequest, "")
	api.CurrentResponseMode = api.ResponseModeEnvelope // read when called, not when generated

	_, successSchema, _ := documentedResponse(t, "200", success)
	_, errorSchema, _ := documentedResponse(t, "400", failure)
	if successSchema != docOptionsTaskRef {
		t.Errorf("success schema = %s, want %s", successSchema, docOptionsTaskRef)
	}
	if errorSchema != nullBodySchema {
		t.Errorf("error schema = %s, want %s", errorSchema, nullBodySchema)
	}
}

func TestDocOptionsOfTheDefaultInstanceNormalizeAnUnknownMode(t *testing.T) {
	saved := api.CurrentResponseMode
	t.Cleanup(func() { api.CurrentResponseMode = saved })
	api.CurrentResponseMode = "unknown"
	var defaultInstance *api.API

	_, schema, _ := documentedResponse(t, "200", defaultInstance.DocSuccess(http.StatusOK, docOptionsTask{}))

	if schema != envelopeTaskSchema {
		t.Errorf("schema = %s, want %s", schema, envelopeTaskSchema)
	}
}

func TestDocOptionsKeepTheLastResponseOfAStatus(t *testing.T) {
	instance := api.New()

	_, schema, _ := documentedResponse(t, "200",
		doc.Status(http.StatusOK, docOptionsTask{}),
		instance.DocSuccess(http.StatusOK, nil),
	)
	description, overridden, _ := documentedResponse(t, "404",
		instance.DocError(http.StatusNotFound, "Task not found"),
		doc.ResponseWithDescription(http.StatusNotFound, "Gone", docOptionsTask{}),
	)

	if schema != envelopeOnlyHeader {
		t.Errorf("200 schema = %s, want %s", schema, envelopeOnlyHeader)
	}
	if description != "Gone" || overridden != docOptionsTaskRef {
		t.Errorf("404 = %q %s, want %q %s", description, overridden, "Gone", docOptionsTaskRef)
	}
}

// fixedFormatter is a custom formatter that sends body under contentType
// whatever the response, or nothing when nilResponse is set.
type fixedFormatter struct {
	body        any
	contentType string
	nilResponse bool
}

func (formatter fixedFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	if formatter.nilResponse {
		return nil
	}
	var headers map[string]string
	if formatter.contentType != "" {
		headers = map[string]string{"Content-Type": formatter.contentType}
	}
	return &core.ResponseFormatted{HTTPStatusCode: data.HTTPStatusCode, Headers: headers, Body: formatter.body}
}

// documentedExample returns the media type, the JSON of the schema and the
// JSON of the example that opts document for status; the example is "" when
// there is none.
func documentedExample(t *testing.T, status string, opts ...doc.RouteOption) (mediaType, schema, example string) {
	t.Helper()
	for mediaType, media := range generatedResponse(t, status, opts...).Content {
		encodedSchema, err := json.Marshal(media.Schema)
		if err != nil {
			t.Fatalf("marshal schema: %v", err)
		}
		if media.Example == nil {
			return mediaType, string(encodedSchema), ""
		}
		encodedExample, err := json.Marshal(media.Example)
		if err != nil {
			t.Fatalf("marshal example: %v", err)
		}
		return mediaType, string(encodedSchema), string(encodedExample)
	}
	return "", "", ""
}

func TestDocSuccessExampleFormatsTheSampleAsTheInstanceSendsIt(t *testing.T) {
	task := docOptionsTask{ID: 42, Title: "Write docs"}
	const taskJSON = `{"id":42,"title":"Write docs"}`
	tests := []struct {
		name          string
		api           *api.API
		status        int
		sample        any
		wantSchema    string
		wantMediaType string
		wantExample   string
	}{
		{
			name: "envelope", api: api.New(), status: http.StatusCreated, sample: task,
			wantSchema: envelopeTaskSchema, wantMediaType: "application/json",
			wantExample: `{"content":` + taskJSON + `,"header":{"code":"0000","type":"success"}}`,
		},
		{
			name: "plain", api: api.New(api.WithResponseMode(api.ResponseModePlain)), status: http.StatusCreated, sample: task,
			wantSchema: docTypeSchema(t, envelope.PlainSuccess[docOptionsTask]()), wantMediaType: "application/json",
			wantExample: `{"action":"","code":"0000","content":` + taskJSON + `,"event_id":"","info":null,"message":"","title":"","token":"","type":"success"}`,
		},
		{
			name: "none", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: task,
			wantSchema: docOptionsTaskRef, wantMediaType: "application/json", wantExample: taskJSON,
		},
		{
			name: "problem", api: api.New(api.WithResponseMode(api.ResponseModeProblem)), status: http.StatusOK, sample: &task,
			wantSchema: docOptionsTaskRef, wantMediaType: "application/json", wantExample: taskJSON,
		},
		{
			name: "envelope nil sample", api: api.New(), status: http.StatusOK, sample: nil,
			wantSchema: envelopeOnlyHeader, wantMediaType: "application/json",
			wantExample: `{"header":{"code":"0000","type":"success"}}`,
		},
		{
			name: "none nil sample sends null", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: nil,
			wantSchema: nullBodySchema, wantMediaType: "application/json",
		},
		{
			name: "no body status", api: api.New(), status: http.StatusNoContent, sample: task,
		},
		{
			name:   "custom formatter body and content type",
			api:    api.New(api.WithResponseFormatter(fixedFormatter{body: map[string]any{"task": task}, contentType: "application/vnd.tasks+json"})),
			status: http.StatusOK, sample: task,
			wantSchema: envelopeTaskSchema, wantMediaType: "application/vnd.tasks+json",
			wantExample: `{"task":` + taskJSON + `}`,
		},
		{
			name:   "custom formatter without a body",
			api:    api.New(api.WithResponseFormatter(fixedFormatter{nilResponse: true})),
			status: http.StatusOK, sample: task,
			wantSchema: envelopeTaskSchema, wantMediaType: "application/json",
		},
		{
			name: "unencodable body", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: math.NaN(),
			wantSchema: `{"type":"number","format":"double"}`, wantMediaType: "application/json",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := test.api.DocSuccessExample(test.status, test.sample)

			mediaType, schema, example := documentedExample(t, strconv.Itoa(test.status), option)

			if mediaType != test.wantMediaType {
				t.Errorf("media type = %q, want %q", mediaType, test.wantMediaType)
			}
			if schema != test.wantSchema {
				t.Errorf("schema = %s, want %s", schema, test.wantSchema)
			}
			if example != test.wantExample {
				t.Errorf("example = %s, want %s", example, test.wantExample)
			}
		})
	}
}

func TestDocSuccessExampleSendsTheCodeAsTheFormatterDoes(t *testing.T) {
	saved := api.ResponseCodes.Success
	t.Cleanup(func() { api.ResponseCodes.Success = saved })
	api.ResponseCodes.Success = "TSK-000"
	instance := api.New(api.WithLowercaseCodes())

	_, _, example := documentedExample(t, "200", instance.DocSuccessExample(http.StatusOK, nil))

	if want := `{"header":{"code":"tsk-000","type":"success"}}`; example != want {
		t.Errorf("example = %s, want %s", example, want)
	}
}

func TestDocSuccessExampleDocumentsTheHeaders(t *testing.T) {
	option := api.New().DocSuccessExample(http.StatusCreated, docOptionsTask{ID: 42},
		doc.ResponseHeader("Location", doc.String, "URL of the created task"))

	response := generatedResponse(t, "201", option)

	header, ok := response.Headers["Location"]
	if !ok || header.Description != "URL of the created task" {
		t.Errorf("headers = %+v, want Location", response.Headers)
	}
}

func TestDocSuccessExampleOfTheDefaultInstanceUsesTheRegisteredFormatter(t *testing.T) {
	api.RegisterNewAPIResponseFormatter(fixedFormatter{body: "registered"})
	t.Cleanup(func() { api.RegisterNewAPIResponseFormatter(api.ResponseFormatter{}) })
	var defaultInstance *api.API

	_, _, example := documentedExample(t, "200", defaultInstance.DocSuccessExample(http.StatusOK, docOptionsTask{}))

	if example != `"registered"` {
		t.Errorf("example = %s, want %q", example, "registered")
	}
}

func TestDocSuccessExampleIsFormattedWhenCalled(t *testing.T) {
	saved := api.CurrentResponseMode
	t.Cleanup(func() { api.CurrentResponseMode = saved })
	api.CurrentResponseMode = api.ResponseModeNone
	var defaultInstance *api.API

	option := defaultInstance.DocSuccessExample(http.StatusOK, docOptionsTask{ID: 42})
	api.CurrentResponseMode = api.ResponseModeEnvelope // read when called, not when generated

	_, _, example := documentedExample(t, "200", option)
	if want := `{"id":42,"title":""}`; example != want {
		t.Errorf("example = %s, want %s", example, want)
	}
}
