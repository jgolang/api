package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

// ExampleAPI_DocSuccess documents a route's bodies as the instance sends
// them: switching the instance to ResponseModeNone would document the task
// alone and a null error body, with no change to the route. The
// ExampleAPI_DocSuccess_<mode> examples show the same route in each mode.
func ExampleAPI_DocSuccess() {
	type Task struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	instance := api.New() // envelope responses

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Get(router, "/tasks/{taskID}", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocSuccess(http.StatusOK, Task{}),
		instance.DocError(http.StatusNotFound, "Task not found"),
	)

	operation := doc.GenerateOpenAPI(docs).Paths["/tasks/{taskID}"]["get"]
	for _, status := range []string{"200", "404"} {
		response := operation.Responses[status]
		schema, err := json.Marshal(response.Content["application/json"].Schema)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(status, response.Description, string(schema))
	}
	// Output:
	// 200 OK {"type":"object","properties":{"content":{"nullable":true,"allOf":[{"$ref":"#/components/schemas/Task"}]},"header":{"$ref":"#/components/schemas/ResponseInfo"}},"required":["header"]}
	// 404 Task not found {"$ref":"#/components/schemas/ErrorResponse"}
}

// ExampleAPI_DocError documents the error body of a none-mode instance, which
// sends JSON null for an error because an error has no content.
func ExampleAPI_DocError() {
	instance := api.New(api.WithResponseMode(api.ResponseModeNone))

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Post(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocError(http.StatusBadRequest, "The title is missing"),
	)

	response := doc.GenerateOpenAPI(docs).Paths["/tasks"]["post"].Responses["400"]
	schema, err := json.Marshal(response.Content["application/json"].Schema)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(response.Description, string(schema))
	// Output:
	// The title is missing {"type":"object","enum":[null],"nullable":true}
}

// ExampleAPI_DocSuccessExample documents a created task with an example that
// the instance's formatter builds from the sample, so it is the body the
// route sends: the blank title and message are omitted and the code is
// ResponseCodes.Success.
func ExampleAPI_DocSuccessExample() {
	type TaskResponse struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	instance := api.New() // envelope responses

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Post(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocSuccessExample(http.StatusCreated, TaskResponse{ID: 42, Title: "Write docs"},
			doc.ResponseHeader("Location", doc.String, "URL of the created task")),
	)

	response := doc.GenerateOpenAPI(docs).Paths["/tasks"]["post"].Responses["201"]
	example, err := json.Marshal(response.Content["application/json"].Example)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(example))
	// Output:
	// {"content":{"id":42,"title":"Write docs"},"header":{"code":"0000","type":"success"}}
}

// ExampleAPI_DocSuccess_envelope documents GET /tasks/{taskID} on an
// envelope-mode instance: an envelope with header and content for a success
// and the header alone for an error.
func ExampleAPI_DocSuccess_envelope() {
	type TaskResponse struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	instance := api.New(api.WithResponseMode(api.ResponseModeEnvelope))

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Get(router, "/tasks/{taskID}", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocSuccess(http.StatusOK, TaskResponse{}),
		instance.DocError(http.StatusNotFound, "Task not found"),
	)

	printTaskResponses(docs)
	// Output:
	// 200 application/json {"type":"object","properties":{"content":{"nullable":true,"allOf":[{"$ref":"#/components/schemas/TaskResponse"}]},"header":{"$ref":"#/components/schemas/ResponseInfo"}},"required":["header"]}
	// 404 application/json {"$ref":"#/components/schemas/ErrorResponse"}
}

// ExampleAPI_DocSuccess_none documents GET /tasks/{taskID} on a none-mode
// instance: the task alone for a success and JSON null for an error, which
// has no content.
func ExampleAPI_DocSuccess_none() {
	type TaskResponse struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	instance := api.New(api.WithResponseMode(api.ResponseModeNone))

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Get(router, "/tasks/{taskID}", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocSuccess(http.StatusOK, TaskResponse{}),
		instance.DocError(http.StatusNotFound, "Task not found"),
	)

	printTaskResponses(docs)
	// Output:
	// 200 application/json {"$ref":"#/components/schemas/TaskResponse"}
	// 404 application/json {"type":"object","enum":[null],"nullable":true}
}

// ExampleAPI_DocSuccess_plain documents GET /tasks/{taskID} on a plain-mode
// instance: the nine top-level plain keys, with the task under content, and
// PlainErrorResponse for an error.
func ExampleAPI_DocSuccess_plain() {
	type TaskResponse struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	instance := api.New(api.WithResponseMode(api.ResponseModePlain))

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Get(router, "/tasks/{taskID}", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocSuccess(http.StatusOK, TaskResponse{}),
		instance.DocError(http.StatusNotFound, "Task not found"),
	)

	printTaskResponses(docs)
	// Output:
	// 200 application/json {"type":"object","properties":{"action":{"type":"string","example":"refresh"},"code":{"type":"string","description":"0000 by default on success.","example":"0000"},"content":{"nullable":true,"allOf":[{"$ref":"#/components/schemas/TaskResponse"}]},"event_id":{"type":"string","example":"f716243f2c92df55fcd8f67018b1dcfb"},"info":{"type":"object","additionalProperties":{"type":"string"},"nullable":true,"example":{"field":"value"}},"message":{"type":"string"},"title":{"type":"string"},"token":{"type":"string","example":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},"type":{"type":"string","example":"success"}},"required":["title","message","type","code","token","action","event_id","info","content"]}
	// 404 application/json {"$ref":"#/components/schemas/PlainErrorResponse"}
}

// ExampleAPI_DocSuccess_problem documents GET /tasks/{taskID} on a
// problem-mode instance: the task alone for a success and RFC 9457 problem
// details under application/problem+json for an error.
func ExampleAPI_DocSuccess_problem() {
	type TaskResponse struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	instance := api.New(api.WithResponseMode(api.ResponseModeProblem))

	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	api.Get(router, "/tasks/{taskID}", func(w http.ResponseWriter, r *http.Request) {},
		instance.DocSuccess(http.StatusOK, TaskResponse{}),
		instance.DocError(http.StatusNotFound, "Task not found"),
	)

	printTaskResponses(docs)
	// Output:
	// 200 application/json {"$ref":"#/components/schemas/TaskResponse"}
	// 404 application/problem+json {"$ref":"#/components/schemas/ProblemDetails"}
}

// printTaskResponses prints, for the 200 and 404 responses of
// GET /tasks/{taskID}, the status, each media type and its schema.
func printTaskResponses(docs *doc.Docs) {
	operation := doc.GenerateOpenAPI(docs).Paths["/tasks/{taskID}"]["get"]
	for _, status := range []string{"200", "404"} {
		content := operation.Responses[status].Content
		mediaTypes := make([]string, 0, len(content))
		for mediaType := range content {
			mediaTypes = append(mediaTypes, mediaType)
		}
		slices.Sort(mediaTypes)
		for _, mediaType := range mediaTypes {
			schema, err := json.Marshal(content[mediaType].Schema)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(status, mediaType, string(schema))
		}
	}
}
