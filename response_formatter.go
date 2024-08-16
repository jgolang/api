package api

import (
	"fmt"

	"github.com/jgolang/api/core"
)

// ResponseFormatter implementation of core.APIResponseFormatter interface.
type ResponseFormatter struct{}

// shortID returns the first 8 characters of the input string
func shortID(s string) string {
	if len(s) < 8 {
		return s
	}
	return s[:8]
}

// Format the response body information.
func (formatter ResponseFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	msg := fmt.Sprintf("%s (%s-%s)", data.Message, shortID(data.EventID), data.ResponseCode)
	return &core.ResponseFormatted{
		HTTPStatusCode: data.HTTPStatusCode,
		Headers:        data.Headers,
		Body: JSONResponse{
			Content: data.Content,
			Header: JSONResponseInfo{
				Title:   data.Title,
				Message: data.Message,
				Type:    data.ResponseType,
				Action:  data.Actions,
				Token:   data.SecurityToken,
				Code:    string(data.ResponseCode),
				EventID: msg,
				Info:    data.Info,
			},
		},
	}
}
