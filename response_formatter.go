package api

import (
	"fmt"

	"github.com/jgolang/api/core"
)

// ResponseFormatter implementation of core.APIResponseFormatter interface.
type ResponseFormatter struct{}

// TraceIDLength specifies the number of characters to return from the beginning of the input trace ID.
var TraceIDLength = 6

// shortID returns the first n characters of the input traceID
func shortID(traceID string) string {
	if len(traceID) < TraceIDLength {
		return traceID
	}
	return traceID[:TraceIDLength]
}

// TraceVisibility controls how trace information is included in the response message.
// - 1: Includes both event ID and response code.
// - 2: Includes only the Code.
// - 3: Includes only the event ID.
var TraceVisibility = 1

// BlankSuccess contols if success include user feeback information
var BlankSuccess = true

// Format the response body information.
func (formatter ResponseFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	msg := data.Message
	title := data.Title

	if TraceVisibility == 1 {
		msg = fmt.Sprintf("%s (%s-%s)", data.Message, shortID(data.EventID), data.ResponseCode)
	}
	if TraceVisibility == 2 {
		msg = fmt.Sprintf("%s (%s)", data.Message, data.ResponseCode)
	}
	if TraceVisibility == 3 {
		msg = fmt.Sprintf("%s (%s)", data.Message, shortID(data.EventID))
	}
	if BlankSuccess {
		msg = ""
		title = ""
	}

	return &core.ResponseFormatted{
		HTTPStatusCode: data.HTTPStatusCode,
		Headers:        data.Headers,
		Body: JSONResponse{
			Content: data.Content,
			Header: JSONResponseInfo{
				Title:   title,
				Message: msg,
				Type:    data.ResponseType,
				Action:  data.Actions,
				Token:   data.SecurityToken,
				Code:    string(data.ResponseCode),
				EventID: data.EventID,
				Info:    data.Info,
			},
		},
	}
}
