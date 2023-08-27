package api

import "github.com/jgolang/api/core"

// Request contains all information to process the API request.
// Wrapper of core.RequestData
type Request struct {
	*core.RequestData
}

// EncryptedRequest documentation ...
type EncryptedRequest struct {
	*core.RequestEncryptedData
}
