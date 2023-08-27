package api

import (
	"net/http"

	"github.com/jgolang/api/core"
)

var (
	// DefaultErrorTitle default error title.
	DefaultErrorTitle = "Error response!"

	// DefaultErrorMessage default error message.
	DefaultErrorMessage = "The service has not completed the operation!"

	// ErrorType error response type the value is "error".
	ErrorType core.ResponseType = "error"

	// InternalServerTitle internal server error title.
	InternalServerTitle = "Sorry!"

	// InternalServerMessage internal server error message.
	InternalServerMessage = "An error has occurred please try again later."

	// UnauthorizedTitle unauthorized error title.
	UnauthorizedTitle = "Unauthorized!"

	// UnauthorizedMessage unauthorized error message.
	UnauthorizedMessage = "User not authorized."
)

// Error error response type the value is "error".
type Error core.ResponseData

// Write response error in screen.
func (err Error) Write(w http.ResponseWriter, r *http.Request) {
	err.EventID = getEventID(r)
	err.ResponseType = ErrorType
	if err.Title == "" {
		err.Title = DefaultErrorTitle
	}
	if err.Message == "" {
		err.Message = DefaultErrorMessage
	}
	if err.HTTPStatusCode == 0 {
		err.HTTPStatusCode = http.StatusBadRequest
	}
	if err.ResponseCode == "" {
		err.ResponseCode = ResponseCodes.DefaultError
	}
	if err.SecurityToken == "" {
		err.SecurityToken = getSecurityToken(r)
	}
	api.Write(core.ResponseData(err), w)
}

// Error400 returns a new HTTP Bad Request error code.
func Error400() Error {
	return Error{}
}

// ErrorWithMsg return a new HTTP Bad Request with custom message.
func ErrorWithMsg(msg string) Error {
	return Error{
		Message: msg,
	}
}

// Error401 returns a new  HTTP Unauthorized error cod.
// and unauthorized error defalut title and default message.
func Error401() Error {
	return Error{
		Title:          UnauthorizedTitle,
		Message:        UnauthorizedMessage,
		ResponseCode:   ResponseCodes.Unauthorized,
		HTTPStatusCode: http.StatusUnauthorized,
	}
}

// Error403 returns a new HTTP Forbiden error code
// and unauthorized error defalut title and default message.
func Error403() Error {
	return Error{
		Title:          UnauthorizedTitle,
		Message:        UnauthorizedMessage,
		ResponseCode:   ResponseCodes.RestrictResource,
		HTTPStatusCode: http.StatusForbidden,
	}
}

// Error500 returns a new HTTP Internal Server Error code.
// and internal server error default title and default mesage.
func Error500() Error {
	return Error{
		Title:          InternalServerTitle,
		Message:        InternalServerMessage,
		ResponseCode:   ResponseCodes.InternalServerEerror,
		HTTPStatusCode: http.StatusInternalServerError,
	}
}

// Error501 returns a new HTTP Not Implement Error code.
// and internal server error default title and default mesage.
func Error501() Error {
	return Error{
		Title:          InternalServerTitle,
		Message:        InternalServerMessage,
		ResponseCode:   ResponseCodes.DefaultError,
		HTTPStatusCode: http.StatusServiceUnavailable,
	}
}

// Error500WithMsg returns a new HTTP internal server error code with custom message.
func Error500WithMsg(msg string) Error {
	err := Error500()
	err.Message = msg
	return err
}
