package api

import (
	"encoding/json"
	"net/http"

	"github.com/jgolang/api/core"
)

// Response interface
type Response interface {
	Write(w http.ResponseWriter)
}

// ResponseWriter doc ...
type ResponseWriter struct{}

// Write doc ...
func (r ResponseWriter) Write(response *core.ResponseFormatted, w http.ResponseWriter) {
	for key := range response.Headers {
		w.Header().Set(key, response.Headers[key])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	err := json.NewEncoder(w).Encode(response.Data)
	if err != nil {
		Fatal(err)
	}
	return
}
