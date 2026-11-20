package doc_test

import (
	"encoding/json"
	"fmt"

	"github.com/jgolang/api/doc"
)

type conflict struct {
	Code string `json:"code"`
}

func ExampleResponseExamples() {
	docs := doc.New(doc.API{Title: "tasks", Version: "1"})
	docs.Register("POST", "/tasks/{id}/complete", doc.ResponseExamples(409, "Transition not allowed or system record", conflict{},
		map[string]interface{}{
			"transition": conflict{Code: "PLN-016"},
			"system":     conflict{Code: "PLN-024"},
		}))

	media := doc.GenerateOpenAPI(docs).Paths["/tasks/{id}/complete"]["post"].Responses["409"].Content["application/json"]
	raw, err := json.Marshal(media.Examples)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(raw))
	// Output: {"system":{"value":{"code":"PLN-024"}},"transition":{"value":{"code":"PLN-016"}}}
}
