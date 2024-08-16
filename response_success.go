package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

var (
	// DefaultSuccessTitle default success title.
	DefaultSuccessTitle = "Successful!"

	// DefaultSuccessMessage default success message.
	DefaultSuccessMessage = "The request has been successful!"

	// SuccessType success response type the value is "success".
	SuccessType core.ResponseType = "success"

	// DefaultSuccessCode default success code.
	DefaultSuccessCode = "0000"
)

// Success success response type the value is "success".
type Success core.ResponseData

// Write success response in screen.
func (success Success) Write(w http.ResponseWriter, r *http.Request) {
	success.EventID = getEventID(r)
	success.ResponseType = SuccessType
	if success.Title == "" {
		success.Title = DefaultSuccessTitle
	}
	if success.Message == "" {
		success.Message = DefaultSuccessMessage
	}
	if success.HTTPStatusCode == 0 {
		success.HTTPStatusCode = http.StatusOK
	}
	if success.ResponseCode == "" {
		success.ResponseCode = ResponseCodes.Success
	}
	if success.SecurityToken == "" {
		success.SecurityToken = getSecurityToken(r)
	}
	api.Write(core.ResponseData(success), w)
}

// Success200 returns a HTTP success response with status code 200.
func Success200() Success {
	return Success{}
}

// SuccessWithMsg returns a HTTP OK response with a custom message.
func SuccessWithMsg(msg string) Success {
	return Success{
		Message: msg,
	}
}

// SuccessWithContent returns a HTTP OK response with conttent response.
func SuccessWithContent(content interface{}) Success {
	return Success{
		Content: content,
	}
}

// Success201 returns a HTTP Created success response with status code 200.
func Success201() Success {
	return Success{
		HTTPStatusCode: http.StatusCreated,
	}
}
