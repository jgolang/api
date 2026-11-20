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
// and status all come from errors.CodeOf(err): the status is StatusForKind of
// the code's kind, with the unauthorized title for 401 and 403 like Error401
// and Error403. An error without a code, and a nil err, answer 500 with the
// Error500 defaults.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	errorResponseFor(err).Write(w, r)
}

// errorResponseFor builds the response WriteError writes, reading err's code
// once so the status, code and message always agree.
func errorResponseFor(err error) Error {
	coder := errors.CodeOf(err)

	var kind codes.Kind
	if kinded, ok := coder.(codes.Kinded); ok {
		kind = kinded.Kind()
	}
	status := StatusForKind(kind)

	response := Error{HTTPStatusCode: status}
	switch status {
	case http.StatusInternalServerError:
		response.Title = InternalServerTitle
		response.Message = InternalServerMessage
		response.ResponseCode = ResponseCodes.InternalServerEerror
	case http.StatusUnauthorized, http.StatusForbidden:
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
