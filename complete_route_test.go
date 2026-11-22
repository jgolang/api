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
	"strconv"
	"strings"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/core"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/api/stdadapter"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

// completeRouteVariant is an instance configuration whose spec and real
// exchanges are pinned in testdata as complete-route.<name>.json; the
// contracttest module validates one against the other.
type completeRouteVariant struct {
	name    string
	options []api.Option
}

// completeRouteVariants are the four response modes and a custom formatter.
var completeRouteVariants = []completeRouteVariant{
	{name: "envelope", options: []api.Option{api.WithResponseMode(api.ResponseModeEnvelope)}},
	{name: "none", options: []api.Option{api.WithResponseMode(api.ResponseModeNone)}},
	{name: "plain", options: []api.Option{api.WithResponseMode(api.ResponseModePlain)}},
	{name: "problem", options: []api.Option{api.WithResponseMode(api.ResponseModeProblem)}},
	{name: "custom-formatter", options: []api.Option{api.WithResponseFormatter(infoFormatter{})}},
}

// infoFormatter is a custom formatter that builds an envelope with only the
// type, code and a fixed info in its header, so the recordings show that
// DocSuccessExample documents what a custom formatter sends. It reads no
// package variable, so the recording does not depend on global state.
type infoFormatter struct{}

func (infoFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{
		HTTPStatusCode: data.HTTPStatusCode,
		Headers:        data.Headers,
		Body: envelope.Response{
			Header: envelope.ResponseInfo{
				Type: data.ResponseType,
				Code: string(data.ResponseCode),
				Info: map[string]string{"formatter": "custom"},
			},
			Content: data.Content,
		},
	}
}

// refreshHint tells a reader how to rewrite the golden files after an
// intended change.
const refreshHint = "if the change is intended, run: go test -run 'CompleteRoute' -update . and review the diff"

// createdTaskID is fixed so the documented 201 example equals the real body.
const createdTaskID = 42

var errTaskTitleRequired = codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)

var errTaskNotFound = codes.New("TSK-404", "task not found").WithKind(codes.KindNotFound)

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

// newCompleteRoute returns the handler of an instance configured by options
// serving realistic routes that use every kind of doc option, documented for
// the bodies the instance really sends. Stories that pin other modes or
// validate the spec reuse it.
func newCompleteRoute(options ...api.Option) (http.Handler, *doc.Docs) {
	instance := api.New(options...)
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
		instance.DocSuccessExample(http.StatusCreated, createdTask,
			doc.ResponseHeader("Location", doc.String, "URL of the created task")),
		instance.DocError(http.StatusBadRequest, "The title is missing or the body is not valid JSON"),
		instance.DocError(http.StatusUnauthorized, "The bearer token is missing or invalid"),
		instance.DocError(http.StatusInternalServerError, "Unexpected server error"),
	)
	api.Get(router, "/projects/{projectID}/tasks/{taskID}", getTask,
		doc.OperationID("getTask"),
		doc.Summary("Get a task"),
		doc.Tags("tasks"),
		doc.PathWithDescription("projectID", doc.String, true, "Project that owns the task"),
		doc.PathWithDescription("taskID", doc.Int, true, "Task to return"),
		instance.DocSuccess(http.StatusOK, projectTaskResponse{}),
		instance.DocError(http.StatusNotFound, "Task not found"),
	)
	return instance.Handler(router), docs
}

// createdTask is the task createTask returns for the request example, the
// sample of the documented 201 example.
var createdTask = projectTaskResponse{ID: createdTaskID, ProjectID: "7", Title: "Write docs", Priority: "high"}

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

// getTask finds only the task createTask creates.
func getTask(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("taskID") != strconv.Itoa(createdTaskID) {
		api.WriteError(w, r, errors.NewC(errTaskNotFound, "no task %s", r.PathValue("taskID")))
		return
	}
	api.SuccessWithContent(projectTaskResponse{
		ID:        createdTaskID,
		ProjectID: r.PathValue("projectID"),
		Title:     "Write docs",
		Priority:  "high",
	}).Write(w, r)
}

func TestCompleteRouteSpecMatchesGolden(t *testing.T) {
	for _, variant := range completeRouteVariants {
		t.Run(variant.name, func(t *testing.T) {
			_, docs := newCompleteRoute(variant.options...)

			got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
			if err != nil {
				t.Fatalf("marshal spec: %v", err)
			}

			assertMatchesGolden(t, fmt.Sprintf("testdata/openapi/complete-route.%s.json", variant.name), got)
		})
	}
}

// recordedExchange is one real request and the response the instance sent,
// as the contracttest module reads it. Body holds the response JSON as is,
// or a JSON string when the response is not JSON (see recordedBody).
type recordedExchange struct {
	Name               string            `json:"name"`
	Method             string            `json:"method"`
	Path               string            `json:"path"`
	RequestContentType string            `json:"requestContentType"`
	RequestBody        string            `json:"requestBody"`
	Status             int               `json:"status"`
	ContentType        string            `json:"contentType"`
	Headers            map[string]string `json:"headers,omitempty"`
	Body               json.RawMessage   `json:"body"`
}

// recordedHeaders are the response headers the complete route documents,
// besides Content-Type; only these are recorded so the files stay stable.
var recordedHeaders = []string{"Location"}

// completeRouteRequests cover the 201 and both 400 paths of createTask, and
// the 200 and 404 of getTask.
var completeRouteRequests = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{name: "valid task", method: http.MethodPost, path: "/projects/7/tasks", body: `{"title":"Write docs","priority":"high"}`},
	{name: "empty title", method: http.MethodPost, path: "/projects/7/tasks", body: `{"title":" "}`},
	{name: "malformed body", method: http.MethodPost, path: "/projects/7/tasks", body: `{`},
	{name: "found task", method: http.MethodGet, path: "/projects/7/tasks/42"},
	{name: "missing task", method: http.MethodGet, path: "/projects/7/tasks/1"},
}

func TestCompleteRouteExchangesMatchGolden(t *testing.T) {
	for _, variant := range completeRouteVariants {
		t.Run(variant.name, func(t *testing.T) {
			handler, _ := newCompleteRoute(variant.options...)
			exchanges := make([]recordedExchange, 0, len(completeRouteRequests))
			for _, request := range completeRouteRequests {
				exchanges = append(exchanges, recordExchange(t, handler, request.name, request.method, request.path, request.body))
			}

			got, err := json.MarshalIndent(exchanges, "", "  ")
			if err != nil {
				t.Fatalf("marshal exchanges: %v", err)
			}

			assertMatchesGolden(t, fmt.Sprintf("testdata/contract/complete-route.%s.json", variant.name), got)
		})
	}
}

func recordExchange(t *testing.T, handler http.Handler, name, method, path, body string) recordedExchange {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	requestContentType := ""
	if body != "" {
		requestContentType = "application/json"
		request.Header.Set("Content-Type", requestContentType)
	}

	handler.ServeHTTP(recorder, request)

	return recordedExchange{
		Name:               name,
		Method:             method,
		Path:               path,
		RequestContentType: requestContentType,
		RequestBody:        body,
		Status:             recorder.Code,
		ContentType:        recorder.Header().Get("Content-Type"),
		Headers:            documentedHeaders(recorder.Header()),
		Body:               recordedBody(t, recorder.Body.Bytes()),
	}
}

// documentedHeaders returns the non-empty recordedHeaders of header, or nil
// when there are none.
func documentedHeaders(header http.Header) map[string]string {
	var headers map[string]string
	for _, name := range recordedHeaders {
		value := header.Get(name)
		if value == "" {
			continue
		}
		if headers == nil {
			headers = make(map[string]string)
		}
		headers[name] = value
	}
	return headers
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
			handler, _ := newCompleteRoute(api.WithResponseMode(api.ResponseModeEnvelope))
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

// TestCompleteRouteCreatedExampleMatchesWire keeps each variant's
// documented 201 example equal to the body the handler really sends for the
// request example.
func TestCompleteRouteCreatedExampleMatchesWire(t *testing.T) {
	for _, variant := range completeRouteVariants {
		t.Run(variant.name, func(t *testing.T) {
			handler, docs := newCompleteRoute(variant.options...)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/projects/7/tasks", strings.NewReader(`{"title":"Write docs","priority":"high"}`))

			handler.ServeHTTP(recorder, request)

			gotBody := decodedJSON(t, recorder.Body.Bytes())
			created := doc.GenerateOpenAPI(docs).Paths["/projects/{projectID}/tasks"]["post"].Responses["201"]
			media, ok := created.Content[recorder.Header().Get("Content-Type")]
			if !ok {
				t.Fatalf("201 documents no %q media type", recorder.Header().Get("Content-Type"))
			}
			example, err := json.Marshal(media.Example)
			if err != nil {
				t.Fatalf("marshal example: %v", err)
			}
			if !reflect.DeepEqual(gotBody, decodedJSON(t, example)) {
				t.Fatalf("wire body = %s, documented example = %s", recorder.Body, example)
			}
			if location := recorder.Header().Get("Location"); location != "/projects/7/tasks/42" {
				t.Fatalf("Location = %q, want /projects/7/tasks/42", location)
			}
		})
	}
}

// decodedJSON returns encoded decoded into generic values, so two bodies
// compare by content whatever their key order.
func decodedJSON(t *testing.T, encoded []byte) any {
	t.Helper()
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode %s: %v", encoded, err)
	}
	return decoded
}
