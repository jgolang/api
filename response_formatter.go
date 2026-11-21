package api

import (
	"fmt"
	"strings"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/envelope"
)

// ResponseFormatter implementation of core.APIResponseFormatter interface.
// The zero value formats with CurrentResponseMode and lowercases response
// codes, as v0 does; New binds an instance's mode and code case to it.
type ResponseFormatter struct {
	// mode is set by New; empty means the default instance's package variable.
	mode ResponseMode
	// declaredCodes is set by New; false lowercases codes, as v0 does.
	declaredCodes bool
}

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
// No trace is added to an empty message.
// - 1: Includes both event ID and response code.
// - 2: Includes only the Code.
// - 3: Includes only the event ID.
var TraceVisibility = 1

// BlankSuccess controls whether success responses leave out their title and
// message. Responses of any other type (error, warning, informative or a
// custom type) keep them, since that is the feedback a client shows.
var BlankSuccess = true

// Format the response body information.
func (formatter ResponseFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	msg := data.Message
	title := data.Title
	code := formatter.responseCode(data.ResponseCode)

	// A trace suffix on an empty message would show clients a bare " (id-code)".
	if data.Message != "" {
		switch TraceVisibility {
		case 1:
			msg = fmt.Sprintf("%s (%s-%s)", data.Message, shortID(data.EventID), code)
		case 2:
			msg = fmt.Sprintf("%s (%s)", data.Message, code)
		case 3:
			msg = fmt.Sprintf("%s (%s)", data.Message, shortID(data.EventID))
		}
	}
	if BlankSuccess && data.ResponseType == SuccessType {
		msg = ""
		title = ""
	}

	responseInfo := envelope.ResponseInfo{
		Title:   title,
		Message: msg,
		Type:    data.ResponseType,
		Action:  data.Actions,
		Token:   data.SecurityToken,
		Code:    code,
		EventID: data.EventID,
		Info:    data.Info,
	}

	body := any(envelope.Response{
		Content: data.Content,
		Header:  responseInfo,
	})
	switch formatter.responseMode() {
	case ResponseModeNone:
		body = data.Content
	case ResponseModePlain:
		body = map[string]any{
			"title":    responseInfo.Title,
			"message":  responseInfo.Message,
			"type":     responseInfo.Type,
			"code":     responseInfo.Code,
			"token":    responseInfo.Token,
			"action":   responseInfo.Action,
			"event_id": responseInfo.EventID,
			"info":     responseInfo.Info,
			"content":  data.Content,
		}
	}

	return &core.ResponseFormatted{
		HTTPStatusCode: data.HTTPStatusCode,
		Headers:        data.Headers,
		Body:           body,
	}
}

// responseCode returns code as the body and the trace suffix show it.
func (formatter ResponseFormatter) responseCode(code core.ResponseCode) string {
	if formatter.declaredCodes {
		return string(code)
	}
	return strings.ToLower(string(code))
}

// withMode returns formatter bound to mode unless it already has one.
func (formatter ResponseFormatter) withMode(mode ResponseMode) ResponseFormatter {
	if formatter.mode == "" {
		formatter.mode = mode
	}
	return formatter
}

func (formatter ResponseFormatter) responseMode() ResponseMode {
	if formatter.mode == "" {
		return CurrentResponseMode
	}
	return formatter.mode
}
