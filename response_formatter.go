package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/envelope"
)

// ResponseFormatter implementation of core.APIResponseFormatter interface.
// The zero value formats with CurrentResponseMode, TraceVisibility,
// TraceIDLength and BlankSuccess and lowercases response codes, as v0 does;
// New binds an instance's mode and code case to it, and the v0 defaults of
// the three formatter settings.
type ResponseFormatter struct {
	// mode is set by New; empty means the default instance's package variable.
	mode ResponseMode
	// declaredCodes is set by New; false lowercases codes, as v0 does.
	declaredCodes bool
	// v0Settings is set by New: the formatter uses the v0 defaults of
	// TraceVisibility, TraceIDLength and BlankSuccess instead of reading them.
	v0Settings bool
}

// TraceIDLength specifies the number of characters to return from the beginning of the input trace ID.
// It affects only the default instance; an instance from New uses 6.
var TraceIDLength = 6

// shortID returns the first length characters of the input traceID.
func shortID(traceID string, length int) string {
	if len(traceID) < length {
		return traceID
	}
	return traceID[:length]
}

// TraceVisibility controls how trace information is included in the response message.
// No trace is added to an empty message.
// - 1: Includes both event ID and response code.
// - 2: Includes only the Code.
// - 3: Includes only the event ID.
// It affects only the default instance; an instance from New uses 1.
var TraceVisibility = 1

// BlankSuccess controls whether success responses leave out their title and
// message. Responses of any other type (error, warning, informative or a
// custom type) keep them, since that is the feedback a client shows.
// It affects only the default instance; an instance from New uses true.
var BlankSuccess = true

// Format the response body information.
func (formatter ResponseFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	// Only the mode New binds selects problem details; the package variable
	// never does, so the default instance keeps its v0 modes.
	if formatter.mode == ResponseModeProblem {
		return formatter.formatProblem(data)
	}
	msg := data.Message
	title := data.Title
	code := formatter.responseCode(data.ResponseCode)

	// A trace suffix on an empty message would show clients a bare " (id-code)".
	if data.Message != "" {
		visibility, idLength := formatter.traceSettings()
		switch visibility {
		case 1:
			msg = fmt.Sprintf("%s (%s-%s)", data.Message, shortID(data.EventID, idLength), code)
		case 2:
			msg = fmt.Sprintf("%s (%s)", data.Message, code)
		case 3:
			msg = fmt.Sprintf("%s (%s)", data.Message, shortID(data.EventID, idLength))
		}
	}
	if formatter.blankSuccess() && data.ResponseType == SuccessType {
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

// problemContentType is the RFC 9457 media type of a problem details body.
const problemContentType = "application/problem+json"

// problemDetails is an RFC 9457 problem details body. Code and EventID are
// extension members (RFC 9457 section 3.2).
type problemDetails struct {
	Type    string `json:"type"`
	Title   string `json:"title,omitempty"`
	Status  int    `json:"status,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Code    string `json:"code,omitempty"`
	EventID string `json:"event_id,omitempty"`
}

// formatProblem formats data in ResponseModeProblem: an error as problem
// details, any other response as its content alone. The title is the HTTP
// status text unless a service set it (see problemTitle). Content, token, actions
// and info never reach a problem body. Detail has no trace suffix, since
// code and event_id carry it.
func (formatter ResponseFormatter) formatProblem(data core.ResponseData) *core.ResponseFormatted {
	if data.ResponseType != ErrorType {
		return &core.ResponseFormatted{
			HTTPStatusCode: data.HTTPStatusCode,
			Headers:        data.Headers,
			Body:           data.Content,
		}
	}
	return &core.ResponseFormatted{
		HTTPStatusCode: data.HTTPStatusCode,
		Headers:        problemHeaders(data.Headers),
		Body: problemDetails{
			Type:    "about:blank",
			Title:   problemTitle(data.Title, data.HTTPStatusCode),
			Status:  data.HTTPStatusCode,
			Detail:  data.Message,
			Code:    formatter.responseCode(data.ResponseCode),
			EventID: data.EventID,
		},
	}
}

// libraryErrorTitles are the titles the library puts on error responses on
// its own: the shipped values of DefaultErrorTitle, InternalServerTitle and
// UnauthorizedTitle, and the titles of its helpers and middlewares. A title
// reassigned through those variables is the service's.
var libraryErrorTitles = map[string]bool{
	"Error response!":                          true,
	"Sorry!":                                   true,
	"Unauthorized!":                            true,
	"Invalid request content":                  true,
	"Request info error!":                      true,
	"Not unmarshal JSON struct!":               true,
	"Error getting header value!":              true,
	"Error getting header value type Int!":     true,
	"Error getting header value type Int64!":   true,
	"Error getting header value type Float64!": true,
	"Error getting header value type Bool!":    true,
	"Error getting route var!":                 true,
	"Error getting route var type Int":         true,
	"Error getting route var type Int64":       true,
	"Error getting route var type Float64":     true,
	"Error getting route var type Bool":        true,
	"Error getting query param!":               true,
	"Error getting query param type Int!":      true,
	"Error getting query param type Int64!":    true,
	"Error getting query param type Float64!":  true,
	"Error getting query param type Bool!":     true,
}

// problemTitle returns the problem title for an error with title and
// status: http.StatusText(status), as RFC 9457 section 4.2.1 advises for
// about:blank, when title is empty or one the library set on its own
// (owner answer Q39); title otherwise, or when status has no text.
func problemTitle(title string, status int) string {
	if title != "" && !libraryErrorTitles[title] {
		return title
	}
	if text := http.StatusText(status); text != "" {
		return text
	}
	return title
}

// problemHeaders returns a copy of headers with the problem Content-Type,
// unless headers already set a non-empty one (a set Content-Type always
// wins; an empty value counts as unset, as in the writers). It copies so the
// caller's map is never changed.
func problemHeaders(headers map[string]string) map[string]string {
	problem := make(map[string]string, len(headers)+1)
	for key, value := range headers {
		if value == "" && strings.EqualFold(key, "Content-Type") {
			continue
		}
		problem[key] = value
	}
	if formattedContentType(problem) == "" {
		problem["Content-Type"] = problemContentType
	}
	return problem
}

// responseCode returns code as the body and the trace suffix show it.
func (formatter ResponseFormatter) responseCode(code core.ResponseCode) string {
	if formatter.declaredCodes {
		return string(code)
	}
	return strings.ToLower(string(code))
}

// traceSettings returns the trace visibility and ID length: the v0 defaults
// on an instance, the package variables on the default instance.
func (formatter ResponseFormatter) traceSettings() (visibility, idLength int) {
	if formatter.v0Settings {
		return 1, 6
	}
	return TraceVisibility, TraceIDLength
}

func (formatter ResponseFormatter) blankSuccess() bool {
	if formatter.v0Settings {
		return true
	}
	return BlankSuccess
}

// withMode returns formatter bound to mode.
func (formatter ResponseFormatter) withMode(mode ResponseMode) ResponseFormatter {
	formatter.mode = mode
	return formatter
}

func (formatter ResponseFormatter) responseMode() ResponseMode {
	if formatter.mode == "" {
		return CurrentResponseMode
	}
	return formatter.mode
}
