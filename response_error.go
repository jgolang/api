package api

import (
	"net/http"

	"github.com/jgolang/api/core"
	"github.com/jgolang/errors"
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

// getErrCode returns the code and public message of the first coded error in
// errs: for each error, the first jgolang/errors error in its chain (also
// behind wrappers such as fmt.Errorf) whose code is not nil. A nil element in
// errs ends the search. Without a code it returns empty values, and the caller
// applies its defaults.
func getErrCode(errs ...error) (code core.ResponseCode, msg string) {
	for _, e := range errs {
		if e == nil {
			return "", ""
		}
		if coder := errors.CodeOf(e); coder != nil {
			return core.ResponseCode(coder.Str()), coder.Msg()
		}
	}
	return "", ""
}

// Error400 returns a new HTTP Bad Request error code. Its response code and
// message come from the first coded error in errs (see getErrCode); without
// one they stay empty and Write applies the default code and message.
func Error400(errs ...error) Error {
	code, msg := getErrCode(errs...)
	return Error{
		Message:      msg,
		ResponseCode: code,
	}
}

// ErrorWithMsg return a new HTTP Bad Request with custom message. Its response
// code comes from the first coded error in errs (see getErrCode).
func ErrorWithMsg(msg string, errs ...error) Error {
	code, _ := getErrCode(errs...)
	return Error{
		Message:      msg,
		ResponseCode: code,
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
// and internal server error default title and default mesage. The code and
// message come from the first coded error in errs (see getErrCode) when there
// is one.
func Error500(errs ...error) Error {
	code, msg := getErrCode(errs...)
	if code == "" {
		code = ResponseCodes.InternalServerEerror
	}
	if msg == "" {
		msg = InternalServerMessage
	}
	return Error{
		Title:          InternalServerTitle,
		Message:        msg,
		ResponseCode:   code,
		HTTPStatusCode: http.StatusInternalServerError,
	}
}

// Error501 returns a new HTTP 501 Not Implemented error with the internal
// server error default title and message. The code and message come from the
// first coded error in errs (see getErrCode) when there is one. Before v0.10.0
// it answered 503.
func Error501(errs ...error) Error {
	code, msg := getErrCode(errs...)
	if code == "" {
		code = ResponseCodes.DefaultError
	}
	if msg == "" {
		msg = InternalServerMessage
	}
	return Error{
		Title:          InternalServerTitle,
		Message:        msg,
		ResponseCode:   code,
		HTTPStatusCode: http.StatusNotImplemented,
	}
}

// Error500WithMsg returns a new HTTP internal server error code with custom message.
func Error500WithMsg(msg string, errs ...error) Error {
	err := Error500(errs...)
	err.Message = msg
	return err
}
