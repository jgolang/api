package api

import "github.com/jgolang/api/core"

// JSONResponse response body structure
// contains the info section, with the response type and the messages for users
// and the content section, with the required data for the request
type JSONResponse struct {
	Header  JSONResponseInfo `json:"header,omitempty"`
	Content interface{}      `json:"content,omitempty"`
}

// JSONResponseOf documents a JSONResponse with a concrete content payload type.
//
// It is intended for OpenAPI schema generation only. Runtime responses keep using
// JSONResponse for backward compatibility.
type JSONResponseOf[T any] struct {
	Header  JSONResponseInfo `json:"header,omitempty"`
	Content T                `json:"content,omitempty"`
}

// JSONErrorResponse documents an error response without a content payload.
type JSONErrorResponse struct {
	Header JSONResponseInfo `json:"header,omitempty"`
}

// SuccessDoc returns a typed success response wrapper for OpenAPI documentation.
func SuccessDoc[T any]() JSONResponseOf[T] {
	return JSONResponseOf[T]{}
}

// ErrorDoc returns an error response wrapper for OpenAPI documentation.
func ErrorDoc() JSONErrorResponse {
	return JSONErrorResponse{}
}

// JSONResponseInfo response body info section
type JSONResponseInfo struct {
	Type    core.ResponseType `json:"type"`
	Title   string            `json:"title,omitempty"`
	Message string            `json:"message,omitempty"`
	Code    string            `json:"code,omitempty"`
	Token   string            `json:"token,omitempty"`
	Action  string            `json:"action,omitempty"`
	EventID string            `json:"event_id,omitempty"`
	Info    map[string]string `json:"info,omitempty"`
}
