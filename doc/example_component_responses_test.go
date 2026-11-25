package doc_test

import (
	"fmt"
	"sort"

	"github.com/jgolang/api/doc"
)

func ExampleDocs_AddResponse() {
	docs := doc.New(doc.API{Title: "tasks", Version: "1"})
	docs.AddResponse("TaskNotFound", doc.ResponseSchema(404, "Task not found", &doc.Schema{Type: "object"}))
	docs.Register("GET", "/tasks/{id}", doc.ResponseRef(404, "TaskNotFound"))
	docs.Register("DELETE", "/tasks/{id}", doc.ResponseRef(404, "TaskNotFound"))

	spec := doc.GenerateOpenAPI(docs)
	fmt.Println(spec.Paths["/tasks/{id}"]["get"].Responses["404"].Ref)
	names := make([]string, 0, len(spec.Components.Responses))
	for name := range spec.Components.Responses {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Println(names)
	// Output:
	// #/components/responses/TaskNotFound
	// [TaskNotFound]
}
