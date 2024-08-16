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
	InvalidParams        core.ResponseCode
}{
	Success:              "000",
	Informative:          "000",
	Warning:              "000",
	DefaultError:         "E001",
	InvalidJSON:          "E002",
	InvalidRequestURL:    "E003",
	ValidationError:      "E004",
	MissingVersionError:  "E005",
	Unauthorized:         "S001",
	RestrictResource:     "S002",
	ObjectNotFound:       "E006",
	InternalServerEerror: "I300",
	ServiceUnavailable:   "I301",
	InvalidParams:        "A101",
}
