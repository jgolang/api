package envelope

import "github.com/jgolang/api/core"

// Response is the runtime JSON response envelope.
type Response struct {
	Header  ResponseInfo `json:"header,omitempty"`
	Content any          `json:"content,omitempty"`
}

// ResponseOf documents a response envelope with a concrete content payload type.
type ResponseOf[T any] struct {
	Header  ResponseInfo `json:"header,omitempty"`
	Content *T           `json:"content,omitempty"`
}

// ErrorResponse documents an error response without a content payload.
type ErrorResponse struct {
	Header ResponseInfo `json:"header,omitempty"`
}

// Success returns a typed response envelope.
func Success[T any]() ResponseOf[T] {
	return ResponseOf[T]{}
}

// Error returns an error response envelope.
func Error() ErrorResponse {
	return ErrorResponse{}
}

// ResponseInfo contains response header metadata.
type ResponseInfo struct {
	Type    core.ResponseType `json:"type" example:"success"`
	Title   string            `json:"title,omitempty" example:"Success"`
	Message string            `json:"message,omitempty" example:"Operation completed"`
	Code    string            `json:"code,omitempty" example:"OK"`
	Token   string            `json:"token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	Action  string            `json:"action,omitempty" example:"refresh"`
	EventID string            `json:"event_id,omitempty" example:"f716243f2c92df55fcd8f67018b1dcfb"`
	Info    map[string]string `json:"info,omitempty" example:"{\"field\":\"value\"}"`
}
