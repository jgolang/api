package api

import "net/http"

// Response interface
type Response interface {
	Send(w http.ResponseWriter)
}
