package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

// ExampleAPI_DocSuccess documents a route's bodies as the instance sends
// them: switching the instance to ResponseModeNone would document the task
// alone and a null error body, with no change to the route.
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
