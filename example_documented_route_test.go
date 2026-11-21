package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/api/stdadapter"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// Example_documentedRoute registers one fully documented route: parameters,
// a request body with an example, bearer security, a 201 envelope response
// with its real wire example and Location header, and the error responses.
func Example_documentedRoute() {
	type newTask struct {
		Title string `json:"title"`
	}
	type task struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	titleRequired := codes.New("TSK-001", "title is required").WithKind(codes.KindValidation)

	createTask := func(w http.ResponseWriter, r *http.Request) {
		var body newTask
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			api.WriteError(w, r, errors.WrapC(err, titleRequired))
			return
		}
		if body.Title == "" {
			api.WriteError(w, r, errors.NewC(titleRequired, "empty title"))
			return
		}
		created := task{ID: 42, Title: body.Title}
		w.Header().Set("Location", fmt.Sprintf("/projects/%s/tasks/%d", r.PathValue("projectID"), created.ID))
		response := api.SuccessWithContent(created)
		response.HTTPStatusCode = http.StatusCreated
		response.Write(w, r)
	}

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.AddSecurityScheme("bearerAuth", doc.BearerSecurity("JWT"))
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Post(router, "/projects/{projectID}/tasks", createTask,
		doc.OperationID("createTask"),
		doc.Summary("Create a task"),
		doc.Tags("tasks"),
		doc.PathWithDescription("projectID", doc.Int, true, "Project that owns the task"),
		doc.QueryWithDescription("notify", doc.Bool, false, "Notify the project members"),
		doc.HeaderWithDescription("X-Request-ID", doc.String, false, "Client request ID for tracing"),
		doc.BodyWithExample(newTask{}, newTask{Title: "Write docs"}),
		doc.Security("bearerAuth"),
		// The example is the body the handler sends in the default envelope mode.
		doc.StatusWithExample(http.StatusCreated, envelope.Success[task](), map[string]any{
			"header":  map[string]any{"type": "success", "code": "0000"},
			"content": map[string]any{"id": 42, "title": "Write docs"},
		}, doc.ResponseHeader("Location", doc.String, "URL of the created task")),
		doc.ResponseWithDescription(http.StatusBadRequest, "The title is missing", envelope.Error()),
		doc.ResponseWithDescription(http.StatusUnauthorized, "The bearer token is missing or invalid", envelope.Error()),
		doc.ResponseWithDescription(http.StatusInternalServerError, "Unexpected server error", envelope.Error()),
	)

	handler := api.New().Handler(router)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/projects/7/tasks", strings.NewReader(`{"title":"Write docs"}`)))

	operation := doc.GenerateOpenAPI(docs).Paths["/projects/{projectID}/tasks"]["post"]
	statuses := make([]string, 0, len(operation.Responses))
	for status := range operation.Responses {
		statuses = append(statuses, status)
	}
	slices.Sort(statuses)

	fmt.Println(recorder.Code)
	fmt.Println(recorder.Header().Get("Location"))
	fmt.Println(strings.TrimSpace(recorder.Body.String()))
	fmt.Println(strings.Join(statuses, " "))
	// Output:
	// 201
	// /projects/7/tasks/42
	// {"header":{"type":"success","code":"0000"},"content":{"id":42,"title":"Write docs"}}
	// 201 400 401 500
}
