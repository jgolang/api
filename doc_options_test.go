package api_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
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
	nullBodySchema     = `{"type":"object","enum":[null],"nullable":true}`
)

// documentedResponse returns the description and the JSON of the schema
// that opts document for status, and whether the response has an example.
func documentedResponse(t *testing.T, status string, opts ...doc.RouteOption) (description, schema string, hasExample bool) {
	t.Helper()
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks", opts...)

	response, ok := doc.GenerateOpenAPI(docs).Paths["/tasks"]["get"].Responses[status]
	if !ok {
		t.Fatalf("response %s is not documented", status)
	}
	media, ok := response.Content["application/json"]
	if !ok {
		return response.Description, "", false
	}
	encoded, err := json.Marshal(media.Schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	return response.Description, string(encoded), media.Example != nil || len(media.Examples) > 0
}

func TestDocSuccessDocumentsTheInstanceModeBody(t *testing.T) {
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
		{name: "plain documented as envelope", api: api.New(api.WithResponseMode(api.ResponseModePlain)), status: http.StatusOK, sample: docOptionsTask{}, wantSchema: envelopeTaskSchema},
		{name: "none", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: docOptionsTask{}, wantSchema: docOptionsTaskRef},
		{name: "none pointer sample", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: &docOptionsTask{}, wantSchema: docOptionsTaskRef},
		{name: "none nil sample", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusOK, sample: nil, wantSchema: nullBodySchema},
		{name: "problem success as none", api: api.New(api.WithResponseMode(api.ResponseModeProblem)), status: http.StatusOK, sample: docOptionsTask{}, wantSchema: docOptionsTaskRef},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			option := test.api.DocSuccess(test.status, test.sample)

			_, schema, hasExample := documentedResponse(t, strconv.Itoa(test.status), option)

			if schema != test.wantSchema {
				t.Errorf("schema = %s, want %s", schema, test.wantSchema)
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
	}{
		{name: "envelope", api: api.New(), status: http.StatusNotFound, description: "Task not found", wantDescription: "Task not found", wantSchema: errorResponseRef},
		{name: "none", api: api.New(api.WithResponseMode(api.ResponseModeNone)), status: http.StatusBadRequest, wantDescription: "Bad Request", wantSchema: nullBodySchema},
		{name: "plain documented as envelope", api: api.New(api.WithResponseMode(api.ResponseModePlain)), status: http.StatusBadRequest, wantDescription: "Bad Request", wantSchema: errorResponseRef},
		{name: "problem documented as envelope", api: api.New(api.WithResponseMode(api.ResponseModeProblem)), status: http.StatusBadRequest, wantDescription: "Bad Request", wantSchema: errorResponseRef},
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
			if hasExample {
				t.Error("response has an example, want schema only")
			}
		})
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
