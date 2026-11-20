package doc_test

import (
	"encoding/json"
	"fmt"

	"github.com/jgolang/api/doc"
)

func ExamplePublic() {
	docs := doc.New(doc.API{Title: "tasks", Version: "1"})
	docs.AddSecurityScheme("bearer", doc.BearerSecurity("JWT"))
	docs.SetDefaultSecurity("bearer")
	docs.Register("GET", "/health", doc.Public())

	spec := doc.GenerateOpenAPI(docs)
	top, err := json.Marshal(spec.Security)
	if err != nil {
		fmt.Println(err)
		return
	}
	health, err := json.Marshal(spec.Paths["/health"]["get"])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(top))
	fmt.Println(string(health))
	// Output:
	// [{"bearer":[]}]
	// {"responses":{"204":{"description":"No Content"}},"security":[]}
}
