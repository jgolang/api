package api

import "github.com/jgolang/api/core"

// Request contains all information to process the API request.
// Wrapper of core.RequestDataContext
type RequestContext struct {
	*core.RequestDataContext
}

// EncryptedRequest documentation ...
type EncryptedRequest struct {
	*core.RequestEncryptedData
}
