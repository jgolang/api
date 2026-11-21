package doc_test

import (
	"encoding/json"
	"fmt"

	"github.com/jgolang/api/doc"
)

func ExampleDocs_SetServers() {
	docs := doc.New(doc.API{
		Title:   "Tasks API",
		Version: "1.0.0",
		Contact: &doc.Contact{Name: "Tasks team", Email: "tasks@example.com"},
		License: &doc.License{Name: "MIT"},
	})
	docs.SetServers(doc.Server{URL: "https://api.example.com", Description: "Production"})

	spec, err := json.Marshal(doc.GenerateOpenAPI(docs))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(spec))
	// Output:
	// {"openapi":"3.0.3","info":{"title":"Tasks API","version":"1.0.0","contact":{"name":"Tasks team","email":"tasks@example.com"},"license":{"name":"MIT"}},"servers":[{"url":"https://api.example.com","description":"Production"}],"paths":{}}
}
