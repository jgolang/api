package envelope

import "github.com/jgolang/api/core"

// PlainResponseOf documents the body that ResponseModePlain writes, with a
// concrete content payload type. It is for documentation only and does not
// change the wire.
//
// Plain mode writes every key, even an empty one, so none has omitempty.
// Info and Content are pointers, and carry validate:"required", so the spec
// lists them as required and nullable: the wire writes null when they are
// unset.
type PlainResponseOf[T any] struct {
	Title   string             `json:"title"`
	Message string             `json:"message"`
	Type    core.ResponseType  `json:"type" example:"success"`
	Code    string             `json:"code" example:"0000" description:"0000 by default on success."`
	Token   string             `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	Action  string             `json:"action" example:"refresh"`
	EventID string             `json:"event_id" example:"f716243f2c92df55fcd8f67018b1dcfb"`
	Info    *map[string]string `json:"info" validate:"required" example:"{\"field\":\"value\"}"`
	Content *T                 `json:"content" validate:"required"`
}

// PlainErrorResponse documents the body that ResponseModePlain writes for
// an error. It is for documentation only and does not change the wire.
//
// Content is usually null on an error, but an error that carries content
// writes it, so it is documented as any nullable value. See PlainResponseOf
// for why Info and Content carry validate:"required".
type PlainErrorResponse struct {
	Title   string             `json:"title" example:"Error response!"`
	Message string             `json:"message" example:"task not found (f71624-TSK-404)"`
	Type    core.ResponseType  `json:"type" example:"error"`
	Code    string             `json:"code" example:"TSK-404" description:"The error's code, such as E001 or TSK-404."`
	Token   string             `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"`
	Action  string             `json:"action" example:"refresh"`
	EventID string             `json:"event_id" example:"f716243f2c92df55fcd8f67018b1dcfb"`
	Info    *map[string]string `json:"info" validate:"required" example:"{\"field\":\"value\"}"`
	Content *any               `json:"content" validate:"required"`
}

// PlainSuccess returns a plain-mode body for documenting a response with a
// T payload in ResponseModePlain.
func PlainSuccess[T any]() PlainResponseOf[T] {
	return PlainResponseOf[T]{}
}

// PlainError returns a plain-mode body for documenting an error in
// ResponseModePlain.
func PlainError() PlainErrorResponse {
	return PlainErrorResponse{}
}
