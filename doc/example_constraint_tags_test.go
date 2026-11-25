package doc_test

import (
	"fmt"

	"github.com/jgolang/api/doc"
)

func ExampleCheckSchemaTags() {
	type capture struct {
		Priority int `json:"priority" enum:"1,two"`
	}
	docs := doc.New(doc.API{Title: "captures", Version: "1"})
	docs.Register("POST", "/captures", doc.Body(capture{}))

	fmt.Println(doc.CheckSchemaTags(docs))
	// Output: [c001](Invalid configuration.): doc: field doc_test.capture.Priority has an invalid enum tag "1,two": strconv.ParseInt: parsing "two": invalid syntax
}

func ExampleSchemaFromType_constraintTags() {
	type capture struct {
		TargetType string `json:"target_type" enum:"goal,project,task,note"`
		Priority   int    `json:"priority" enum:"1,2,3" default:"2"`
	}

	schema := doc.SchemaFromType(capture{})
	fmt.Println(schema.Properties["target_type"].Enum)
	fmt.Println(schema.Properties["priority"].Enum, schema.Properties["priority"].Default)
	// Output:
	// [goal project task note]
	// [1 2 3] 2
}
