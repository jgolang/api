package api

import "github.com/jgolang/api/core"

const defaultInternalServerErrorCode core.ResponseCode = "E300"

// ResponseCodes catalog
var ResponseCodes = struct {
	Success             core.ResponseCode
	Informative         core.ResponseCode
	Warning             core.ResponseCode
	DefaultError        core.ResponseCode
	InvalidJSON         core.ResponseCode
	InvalidRequestURL   core.ResponseCode
	ValidationError     core.ResponseCode
	MissingVersionError core.ResponseCode
	Unauthorized        core.ResponseCode
	ObjectNotFound      core.ResponseCode
	RestrictResource    core.ResponseCode
	// InternalServerEerror is the misspelled former name of
	// InternalServerError.
	//
	// Deprecated: use InternalServerError. A program that still sets only
	// this field keeps its code until InternalServerError is changed.
	InternalServerEerror core.ResponseCode
	// InternalServerError is the code of a 500 response that carries no
	// coded error (Error500 without one, WriteError with a nil or uncoded
	// error). When it is changed from its default it takes precedence over
	// the deprecated InternalServerEerror.
	InternalServerError core.ResponseCode
	ServiceUnavailable  core.ResponseCode
	InvalidParams       core.ResponseCode
}{
	Success:              "0000",
	Informative:          "0000",
	Warning:              "0000",
	DefaultError:         "E001",
	InvalidJSON:          "E002",
	InvalidRequestURL:    "E003",
	ValidationError:      "E004",
	MissingVersionError:  "E005",
	Unauthorized:         "B001",
	RestrictResource:     "B002",
	ObjectNotFound:       "E006",
	InternalServerEerror: defaultInternalServerErrorCode,
	InternalServerError:  defaultInternalServerErrorCode,
	ServiceUnavailable:   "E301",
	InvalidParams:        "A101",
}

// internalServerErrorCode returns the code of a 500 response without a coded
// error. The new field wins once changed from its default, so programs that
// set only the deprecated field keep working.
func internalServerErrorCode() core.ResponseCode {
	if ResponseCodes.InternalServerError == defaultInternalServerErrorCode {
		return ResponseCodes.InternalServerEerror
	}
	return ResponseCodes.InternalServerError
}
