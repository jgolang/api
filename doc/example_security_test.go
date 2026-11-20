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
	top, _ := json.Marshal(spec.Security)
	health, _ := json.Marshal(spec.Paths["/health"]["get"].Security)
	fmt.Println(string(top), spec.Paths["/health"]["get"].Public, string(health))
	// Output: [{"bearer":[]}] true null
}
