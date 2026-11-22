package envelope_test

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
)

type exampleTask struct {
	ID int `json:"id"`
}

// printDocumentedKeys prints the properties and required keys that body
// documents for a response.
func printDocumentedKeys(body any) {
	docs := doc.New(doc.API{Title: "Tasks", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks/{id}", doc.Status(http.StatusOK, body))
	spec := doc.GenerateOpenAPI(docs)
	ref := spec.Paths["/tasks/{id}"]["get"].Responses["200"].Content["application/json"].Schema.Ref
	schema := spec.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")]

	properties := slices.Sorted(maps.Keys(schema.Properties))
	required := slices.Sorted(slices.Values(schema.Required))
	fmt.Println("properties:", properties)
	fmt.Println("required:", required)
}

func ExampleProblem() {
	printDocumentedKeys(envelope.Problem())
	// Output:
	// properties: [code detail event_id status title type]
	// required: [type]
}

func ExamplePlainSuccess() {
	printDocumentedKeys(envelope.PlainSuccess[exampleTask]())
	// Output:
	// properties: [action code content event_id info message title token type]
	// required: [action code content event_id info message title token type]
}

func ExamplePlainError() {
	printDocumentedKeys(envelope.PlainError())
	// Output:
	// properties: [action code content event_id info message title token type]
	// required: [action code content event_id info message title token type]
}
