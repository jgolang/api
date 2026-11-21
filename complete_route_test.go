package api_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/api/stdadapter"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

// completeRouteModes are the response modes whose spec and real exchanges
// are pinned in testdata; the contracttest module validates one against the
// other.
var completeRouteModes = []api.ResponseMode{api.ResponseModeEnvelope, api.ResponseModeNone}

// refreshHint tells a reader how to rewrite the golden files after an
// intended change.
const refreshHint = "if the change is intended, run: go test -run 'CompleteRoute' -update . and review the diff"

// createdTaskID is fixed so the documented 201 example equals the real body.
const createdTaskID = 42

var errTaskTitleRequired = codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)

type createProjectTaskRequest struct {
	Title    string `json:"title"`
	Priority string `json:"priority,omitempty"`
}

type projectTaskResponse struct {
	ID        int    `json:"id"`
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Priority  string `json:"priority,omitempty"`
}

// newCompleteRoute returns a realistic route that uses every kind of doc
// option, documented for the bodies mode really sends. Stories that pin
// other modes or validate the spec reuse it.
func newCompleteRoute(mode api.ResponseMode) (http.Handler, *doc.Docs) {
	createdBody, errorBody := documentedBodies(mode)
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.AddSecurityScheme("bearerAuth", doc.BearerSecurity("JWT"))
	router := stdadapter.New(http.NewServeMux(), docs)

	api.Post(router, "/projects/{projectID}/tasks", createTask,
		doc.OperationID("createTask"),
		doc.Summary("Create a task"),
		doc.Tags("tasks"),
		doc.PathWithDescription("projectID", doc.String, true, "Project that owns the task"),
		doc.QueryWithDescription("notify", doc.Bool, false, "Notify the project members"),
		doc.HeaderWithDescription("X-Request-ID", doc.String, false, "Client request ID for tracing"),
		doc.BodyWithExample(createProjectTaskRequest{}, createProjectTaskRequest{Title: "Write docs", Priority: "high"}),
		doc.Security("bearerAuth"),
		doc.StatusWithExample(http.StatusCreated, createdBody, createdTaskExample(mode),
			doc.ResponseHeader("Location", doc.String, "URL of the created task")),
		doc.ResponseWithDescription(http.StatusBadRequest, "The title is missing or the body is not valid JSON", errorBody),
		doc.ResponseWithDescription(http.StatusUnauthorized, "The bearer token is missing or invalid", errorBody),
		doc.ResponseWithDescription(http.StatusInternalServerError, "Unexpected server error", errorBody),
	)
	return router, docs
}

// documentedBodies returns the 201 and error body models of mode. None mode
// writes the content alone: the task on success, and JSON null on an error
// because an error has no content. The doc layer cannot describe a null
// body, so none-mode errors are documented without one; the contract test
// records that as a known gap until epic 2 fixes the wire.
func documentedBodies(mode api.ResponseMode) (createdBody, errorBody any) {
	if mode == api.ResponseModeNone {
		return projectTaskResponse{}, nil
	}
	return envelope.Success[projectTaskResponse](), envelope.Error()
}

// createdTaskExample is the body createTask sends in mode for the request
// example, captured from a real run.
func createdTaskExample(mode api.ResponseMode) map[string]any {
	content := map[string]any{
		"id":        createdTaskID,
		"projectId": "7",
		"title":     "Write docs",
		"priority":  "high",
	}
	if mode == api.ResponseModeNone {
		return content
	}
	return map[string]any{
		"header": map[string]any{
			"type": "success",
			"code": "0000",
		},
		"content": content,
	}
}

func createTask(w http.ResponseWriter, r *http.Request) {
	var request createProjectTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.WriteError(w, r, errors.WrapC(err, errTaskTitleRequired))
		return
	}
	if strings.TrimSpace(request.Title) == "" {
		api.WriteError(w, r, errors.NewC(errTaskTitleRequired, "empty title"))
		return
	}
	task := projectTaskResponse{
		ID:        createdTaskID,
		ProjectID: r.PathValue("projectID"),
		Title:     request.Title,
		Priority:  request.Priority,
	}
	w.Header().Set("Location", fmt.Sprintf("/projects/%s/tasks/%d", task.ProjectID, task.ID))
	response := api.SuccessWithContent(task)
	response.HTTPStatusCode = http.StatusCreated
	response.Write(w, r)
}

func TestCompleteRouteSpecMatchesGolden(t *testing.T) {
	for _, mode := range completeRouteModes {
		t.Run(string(mode), func(t *testing.T) {
			_, docs := newCompleteRoute(mode)

			got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
			if err != nil {
				t.Fatalf("marshal spec: %v", err)
			}

			assertMatchesGolden(t, fmt.Sprintf("testdata/openapi/complete-route.%s.json", mode), got)
		})
	}
}

// recordedExchange is one real request and the response the instance sent,
// as the contracttest module reads it. Body holds the response JSON as is,
// or a JSON string when the response is not JSON (see recordedBody).
type recordedExchange struct {
	Name               string          `json:"name"`
	Method             string          `json:"method"`
	Path               string          `json:"path"`
	RequestContentType string          `json:"requestContentType"`
	RequestBody        string          `json:"requestBody"`
	Status             int             `json:"status"`
	ContentType        string          `json:"contentType"`
	Body               json.RawMessage `json:"body"`
}

// completeRouteRequests cover the 201 and both 400 paths of createTask.
var completeRouteRequests = []struct {
	name string
	body string
}{
	{name: "valid task", body: `{"title":"Write docs","priority":"high"}`},
	{name: "empty title", body: `{"title":" "}`},
	{name: "malformed body", body: `{`},
}

func TestCompleteRouteExchangesMatchGolden(t *testing.T) {
	for _, mode := range completeRouteModes {
		t.Run(string(mode), func(t *testing.T) {
			router, _ := newCompleteRoute(mode)
			handler := api.New(api.WithResponseMode(mode)).Handler(router)
			exchanges := make([]recordedExchange, 0, len(completeRouteRequests))
			for _, request := range completeRouteRequests {
				exchanges = append(exchanges, recordExchange(t, handler, request.name, request.body))
			}

			got, err := json.MarshalIndent(exchanges, "", "  ")
			if err != nil {
				t.Fatalf("marshal exchanges: %v", err)
			}

			assertMatchesGolden(t, fmt.Sprintf("testdata/contract/complete-route.%s.json", mode), got)
		})
	}
}

func recordExchange(t *testing.T, handler http.Handler, name, body string) recordedExchange {
	t.Helper()
	const path = "/projects/7/tasks"
	const requestContentType = "application/json"
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", requestContentType)

	handler.ServeHTTP(recorder, request)

	return recordedExchange{
		Name:               name,
		Method:             http.MethodPost,
		Path:               path,
		RequestContentType: requestContentType,
		RequestBody:        body,
		Status:             recorder.Code,
		ContentType:        recorder.Header().Get("Content-Type"),
		Body:               recordedBody(t, recorder.Body.Bytes()),
	}
}

// recordedBody keeps a JSON body as is, so the golden file shows it as
// JSON. Any other body, and a body that is itself a JSON string, is stored
// as a JSON string, so a reader can tell the two apart by the first byte.
func recordedBody(t *testing.T, body []byte) json.RawMessage {
	t.Helper()
	trimmed := bytes.TrimSpace(body)
	if json.Valid(trimmed) && trimmed[0] != '"' {
		return trimmed
	}
	quoted, err := json.Marshal(string(body))
	if err != nil {
		t.Fatalf("quote body: %v", err)
	}
	return quoted
}

// assertMatchesGolden compares got with the golden file at path, rewriting
// it first when -update is set.
func assertMatchesGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	got = append(got, '\n')
	if *updateGolden {
		writeGolden(t, path, got)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (%s)", path, err, refreshHint)
	}
	if diff := firstDifference(want, got); diff != "" {
		t.Fatalf("%s differs from the generated output:\n%s\n%s", path, diff, refreshHint)
	}
}

func writeGolden(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create golden folder: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write golden %s: %v", path, err)
	}
}

// diffContextLines is how many lines before the first difference are shown,
// so a reviewer sees where in the spec the change sits.
const diffContextLines = 3

// firstDifference returns the first line where want and got differ, with a
// few preceding lines of context, or "" when they are equal. A full diff is
// left to git; this only has to point the reader at the change.
func firstDifference(want, got []byte) string {
	if bytes.Equal(want, got) {
		return ""
	}
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	index := 0
	for index < len(wantLines) && index < len(gotLines) && wantLines[index] == gotLines[index] {
		index++
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "first difference at line %d\n", index+1)
	for context := max(0, index-diffContextLines); context < index; context++ {
		fmt.Fprintf(&builder, "  %s\n", wantLines[context])
	}
	fmt.Fprintf(&builder, "- want: %s\n", lineOrAbsent(wantLines, index))
	fmt.Fprintf(&builder, "+ got:  %s\n", lineOrAbsent(gotLines, index))
	return builder.String()
}

func lineOrAbsent(lines []string, index int) string {
	if index >= len(lines) {
		return "<absent>"
	}
	return lines[index]
}

func TestFirstDifference(t *testing.T) {
	tests := []struct {
		name        string
		want        string
		got         string
		contains    []string
		notContains []string
	}{
		{name: "equal", want: "a\nb\n", got: "a\nb\n"},
		{
			name:        "differing line",
			want:        "a\nb\nc\nd\ne\n",
			got:         "a\nb\nc\nd\nX\n",
			contains:    []string{"line 5", "- want: e\n", "+ got:  X\n", "  b\n", "  d\n"},
			notContains: []string{"  a\n"},
		},
		{
			name:     "got is longer",
			want:     "a",
			got:      "a\nb",
			contains: []string{"line 2", "- want: <absent>", "+ got:  b\n"},
		},
		{
			name:     "want is longer",
			want:     "a\nb",
			got:      "a",
			contains: []string{"line 2", "- want: b\n", "+ got:  <absent>"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := firstDifference([]byte(test.want), []byte(test.got))

			if len(test.contains) == 0 && diff != "" {
				t.Fatalf("firstDifference() = %q, want empty", diff)
			}
			for _, fragment := range test.contains {
				if !strings.Contains(diff, fragment) {
					t.Errorf("firstDifference() = %q, want it to contain %q", diff, fragment)
				}
			}
			for _, fragment := range test.notContains {
				if strings.Contains(diff, fragment) {
					t.Errorf("firstDifference() = %q, want it not to contain %q", diff, fragment)
				}
			}
		})
	}
}

func TestCompleteRouteServesDocumentedResponses(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "valid task", body: `{"title":"Write docs","priority":"high"}`, wantStatus: http.StatusCreated, wantCode: `"code":"0000"`},
		{name: "empty title", body: `{"title":" "}`, wantStatus: http.StatusBadRequest, wantCode: `"code":"TSK-001"`},
		{name: "malformed body", body: `{`, wantStatus: http.StatusBadRequest, wantCode: `"code":"TSK-001"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, _ := newCompleteRoute(api.ResponseModeEnvelope)
			handler := api.New().Handler(router)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/projects/7/tasks", strings.NewReader(test.body))

			handler.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, test.wantStatus, recorder.Body)
			}
			if !strings.Contains(recorder.Body.String(), test.wantCode) {
				t.Fatalf("body %s, want it to contain %s", recorder.Body, test.wantCode)
			}
		})
	}
}

// TestCompleteRouteCreatedExampleMatchesWire keeps each mode's documented
// 201 example equal to the body the handler really sends for the request
// example.
func TestCompleteRouteCreatedExampleMatchesWire(t *testing.T) {
	for _, mode := range completeRouteModes {
		t.Run(string(mode), func(t *testing.T) {
			router, _ := newCompleteRoute(mode)
			handler := api.New(api.WithResponseMode(mode)).Handler(router)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/projects/7/tasks", strings.NewReader(`{"title":"Write docs","priority":"high"}`))

			handler.ServeHTTP(recorder, request)

			var gotBody any
			if err := json.Unmarshal(recorder.Body.Bytes(), &gotBody); err != nil {
				t.Fatalf("decode body %s: %v", recorder.Body, err)
			}
			wantJSON, err := json.Marshal(createdTaskExample(mode))
			if err != nil {
				t.Fatalf("marshal example: %v", err)
			}
			var wantBody any
			if err := json.Unmarshal(wantJSON, &wantBody); err != nil {
				t.Fatalf("decode example: %v", err)
			}
			if !reflect.DeepEqual(gotBody, wantBody) {
				t.Fatalf("wire body = %s, documented example = %s", recorder.Body, wantJSON)
			}
			if location := recorder.Header().Get("Location"); location != "/projects/7/tasks/42" {
				t.Fatalf("Location = %q, want /projects/7/tasks/42", location)
			}
		})
	}
}
