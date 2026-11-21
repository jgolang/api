package api

import (
	"net/http"

	"github.com/jgolang/api/core"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// StatusForKind returns the HTTP status for a jgolang/errors code kind:
// validation 400, unauthorized 401, forbidden 403, not_found 404, conflict 409,
// and 500 for infrastructure, no kind or an unknown kind.
func StatusForKind(kind codes.Kind) int {
	switch kind {
	case codes.KindValidation:
		return http.StatusBadRequest
	case codes.KindUnauthorized:
		return http.StatusUnauthorized
	case codes.KindForbidden:
		return http.StatusForbidden
	case codes.KindNotFound:
		return http.StatusNotFound
	case codes.KindConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// WriteError writes err as an error response. The response code, message
// and status all come from errors.CodeOf(err): the status is the request
// instance's WithKindStatus override for the code's kind, else StatusForKind
// of that kind, with the internal title for any 5xx and the unauthorized
// title for 401 and 403 like Error500, Error501, Error401 and Error403. An error without a
// code, and a nil err, answer 500 with the Error500 defaults. When the
// instance has a WithErrorHook, WriteError calls it with err and the status
// before writing.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	a := instanceFrom(r)
	response := errorResponseFor(err, a.statusFor)
	if a.errorHook != nil {
		a.errorHook(r, err, response.HTTPStatusCode)
	}
	response.Write(w, r)
}

// statusFor returns the instance's override for kind, else StatusForKind.
func (a *API) statusFor(kind codes.Kind) int {
	if status, ok := a.kindStatus[kind]; ok {
		return status
	}
	return StatusForKind(kind)
}

// errorResponseFor builds the response WriteError writes, reading err's code
// once so the status, code and message always agree. statusFor decides the
// status from the code's kind.
func errorResponseFor(err error, statusFor func(codes.Kind) int) Error {
	coder := errors.CodeOf(err)

	var kind codes.Kind
	if kinded, ok := coder.(codes.Kinded); ok {
		kind = kinded.Kind()
	}
	status := statusFor(kind)

	response := Error{HTTPStatusCode: status}
	switch {
	case status == http.StatusInternalServerError:
		response.Title = InternalServerTitle
		response.Message = InternalServerMessage
		response.ResponseCode = internalServerErrorCode()
	case status > http.StatusInternalServerError:
		// Only a WithKindStatus override reaches another 5xx; titled like Error501.
		response.Title = InternalServerTitle
		response.Message = InternalServerMessage
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		response.Title = UnauthorizedTitle
	}
	if coder != nil {
		response.ResponseCode = core.ResponseCode(coder.Str())
		if coder.Msg() != "" {
			response.Message = coder.Msg()
		}
	}
	return response
}
