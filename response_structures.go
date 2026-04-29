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
	Content *T               `json:"content,omitempty"`
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
	Type    core.ResponseType `json:"type" example:"success"`
	Title   string            `json:"title,omitempty" example:"Success"`
	Message string            `json:"message,omitempty" example:"Operation completed"`
	Code    string            `json:"code,omitempty" example:"OK"`
	Token   string            `json:"token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	Action  string            `json:"action,omitempty" example:"refresh"`
	EventID string            `json:"event_id,omitempty" example:"f716243f2c92df55fcd8f67018b1dcfb"`
	Info    map[string]string `json:"info,omitempty" example:"{\"field\":\"value\"}"`
}
