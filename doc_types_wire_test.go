package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

type wireTask struct {
	ID int `json:"id"`
}

// wireMembers serves handler through an instance in mode and returns the
// members of the JSON object body it writes.
func wireMembers(t *testing.T, mode api.ResponseMode, handler http.HandlerFunc) map[string]json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set(api.EventIDHeaderKey, "evt-123")
	rec := httptest.NewRecorder()

	api.New(api.WithResponseMode(mode)).Handler(handler).ServeHTTP(rec, req)

	var members map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return members
}

// documentedBody returns the component schema that the doc type body
// generates for a response.
func documentedBody(t *testing.T, body any) *doc.Schema {
	t.Helper()
	docs := doc.New(doc.API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks", doc.Status(http.StatusOK, body))
	spec := doc.GenerateOpenAPI(docs)

	for _, media := range spec.Paths["/tasks"]["get"].Responses["200"].Content {
		name := strings.TrimPrefix(media.Schema.Ref, "#/components/schemas/")
		if schema := spec.Components.Schemas[name]; schema != nil {
			return schema
		}
	}
	t.Fatalf("expected a component schema for %T", body)
	return nil
}

func sortedKeys[V any](members map[string]V) []string {
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestDocTypesMatchWireKeys(t *testing.T) {
	taskNotFound := codes.New("TSK-404", "task not found").WithKind(codes.KindNotFound)
	writeError := func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, errors.NewC(taskNotFound, "task 7"))
	}
	writeTask := func(w http.ResponseWriter, r *http.Request) {
		api.SuccessWithContent(wireTask{ID: 7}).Write(w, r)
	}
	tests := []struct {
		name    string
		mode    api.ResponseMode
		handler http.HandlerFunc
		body    any
		// alwaysWritten is true when the mode writes every key, even an
		// empty one, so each must be documented as required.
		alwaysWritten bool
	}{
		{name: "problem error", mode: api.ResponseModeProblem, handler: writeError, body: envelope.Problem()},
		{name: "plain success", mode: api.ResponseModePlain, handler: writeTask, body: envelope.PlainSuccess[wireTask](), alwaysWritten: true},
		{name: "plain error", mode: api.ResponseModePlain, handler: writeError, body: envelope.PlainError(), alwaysWritten: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members := wireMembers(t, tt.mode, tt.handler)

			schema := documentedBody(t, tt.body)

			wire, documented := sortedKeys(members), sortedKeys(schema.Properties)
			if !slices.Equal(documented, wire) {
				t.Fatalf("documented properties = %v, wire keys = %v", documented, wire)
			}
			for key, value := range members {
				if string(value) == "null" && !schema.Properties[key].Nullable {
					t.Errorf("wire writes %s as null, but it is not documented as nullable", key)
				}
				if tt.alwaysWritten && !slices.Contains(schema.Required, key) {
					t.Errorf("wire always writes %s, but it is not documented as required", key)
				}
			}
		})
	}
}
