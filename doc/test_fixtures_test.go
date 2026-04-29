package doc

import "github.com/jgolang/api/core"

type testRequestOf[T any] struct {
	Header  testRequestInfo `json:"header,omitempty"`
	Content *T              `json:"content,omitempty"`
}

type testResponseOf[T any] struct {
	Header  testResponseInfo `json:"header,omitempty"`
	Content *T               `json:"content,omitempty"`
}

type testErrorResponse struct {
	Header testResponseInfo `json:"header,omitempty"`
}

type testRequestInfo struct {
	UUID string `json:"uuid,omitempty" example:"ADAD3-ADD33-AFSFK"`
}

type testResponseInfo struct {
	Type core.ResponseType `json:"type" example:"success"`
}
