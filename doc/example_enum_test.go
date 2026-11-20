package doc_test

import (
	"fmt"

	"github.com/jgolang/api/doc"
)

func ExampleQueryEnum() {
	docs := doc.New(doc.API{Title: "tasks", Version: "1"})
	docs.Register("GET", "/tasks", doc.QueryEnum("in_inbox", doc.Bool, false, "Only true is accepted.", true))

	schema := doc.GenerateOpenAPI(docs).Paths["/tasks"]["get"].Parameters[0].Schema
	fmt.Println(schema.Type, schema.Enum)
	// Output: boolean [true]
}
