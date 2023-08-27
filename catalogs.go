package api

import "github.com/jgolang/api/core"

// ResponseCodes catalog
var ResponseCodes = struct {
	Success              core.ResponseCode
	Informative          core.ResponseCode
	Warning              core.ResponseCode
	DefaultError         core.ResponseCode
	InvalidJSON          core.ResponseCode
	InvalidRequestURL    core.ResponseCode
	ValidationError      core.ResponseCode
	MissingVersionError  core.ResponseCode
	Unauthorized         core.ResponseCode
	ObjectNotFound       core.ResponseCode
	RestrictResource     core.ResponseCode
	InternalServerEerror core.ResponseCode
	ServiceUnavailable   core.ResponseCode
	AfterHours           core.ResponseCode
}{
	Success:              "success",
	Informative:          "success",
	Warning:              "warning",
	DefaultError:         "error",
	InvalidJSON:          "invalid_json",
	InvalidRequestURL:    "invalid_request_url",
	ValidationError:      "validation_error",
	MissingVersionError:  "missing_version",
	Unauthorized:         "unauthorized",
	RestrictResource:     "restricted_resource",
	ObjectNotFound:       "object_not_found",
	InternalServerEerror: "internal_server_error",
	ServiceUnavailable:   "service_unauvailable",
	AfterHours:           "after_hours",
}
