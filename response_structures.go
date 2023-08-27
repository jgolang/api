package api

import (
	"github.com/jgolang/api/core"
)

// JSONResponse response body structure
// contains the info section, with the response type and the messages for users
// and the content section, with the required data for the request
type JSONResponse struct {
	Header  JSONResponseInfo `json:"info"`
	Content interface{}      `json:"content,omitempty"`
}

// JSONResponseInfo response body info section
type JSONResponseInfo struct {
	Type           core.ResponseType `json:"type"`
	Title          string            `json:"title,omitempty"`
	Message        string            `json:"message,omitempty"`
	Action         string            `json:"action,omitempty"`
	Token          string            `json:"session_id,omitempty"`
	Code           string            `json:"code,omitempty"`
	EventID        string            `json:"event_id,omitempty"`
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}
