package envelope

import "github.com/jgolang/api/core"

// Response is the runtime JSON response envelope.
type Response struct {
	Header  ResponseInfo `json:"header,omitempty"`
	Content any          `json:"content,omitempty"`
}

// ResponseOf documents a response envelope with a concrete content payload type.
type ResponseOf[T any] struct {
	// No omitempty: the spec lists header as required, as on the wire, and
	// encoding/json never omits a struct, so the encoded body is unchanged.
	Header  ResponseInfo `json:"header"`
	Content *T           `json:"content,omitempty"`
}

// ErrorResponse documents an error response without a content payload.
type ErrorResponse struct {
	// No omitempty: the spec lists header as required, as on the wire.
	Header ResponseInfo `json:"header"`
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
	Code    string            `json:"code,omitempty" example:"0000" description:"0000 by default on success; on an error, the error's code, such as E001 or TSK-404."`
	Token   string            `json:"token,omitempty" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	Action  string            `json:"action,omitempty" example:"refresh"`
	EventID string            `json:"event_id,omitempty" example:"f716243f2c92df55fcd8f67018b1dcfb"`
	Info    map[string]string `json:"info,omitempty" example:"{\"field\":\"value\"}"`
}
