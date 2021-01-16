package api

import "net/http"

// Response interface
type Response interface {
	Write(w http.ResponseWriter)
}
