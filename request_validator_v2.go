package api

import (
	"net/http"

	"github.com/jgolang/api/core"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// RequestReceiverV2 implementation of core.APIRequestReceiver interface.
// The zero value parses with CurrentRequestMode.
type RequestReceiverV2 struct {
	// mode is set by New; empty means the default instance's package variable.
	mode RequestMode
}

// ProcessEncryptedBody reads the encrypted payload from the values data and
// metadata, taken from the form body or the URL query (r.FormValue). Both are
// required; a missing one returns an error with the validation code
// codes.ValRequired (400 through WriteError). A body that cannot be parsed
// returns the parse error. On any error
// the returned value is empty.
func (receiver RequestReceiverV2) ProcessEncryptedBody(r *http.Request) (*core.RequestEncryptedData, error) {
	var requestData core.RequestEncryptedData
	if err := r.ParseForm(); err != nil {
		return &requestData, err
	}

	data := r.FormValue("data")
	metadata := r.FormValue("metadata")

	if data == "" || metadata == "" {
		return &requestData, errors.NewC(codes.ValRequired, "does not provide required information")
	}

	requestData.Data = data
	requestData.Metadata = metadata

	return &requestData, nil
}

// ProcessBody API request body information.
func (receiver RequestReceiverV2) ProcessBody(r *http.Request) (*core.RequestDataContext, error) {
	return processRequestBody(r, receiver.requestMode(), false)
}

// withMode returns receiver bound to mode.
func (receiver RequestReceiverV2) withMode(mode RequestMode) RequestReceiverV2 {
	receiver.mode = mode
	return receiver
}

func (receiver RequestReceiverV2) requestMode() RequestMode {
	if receiver.mode == "" {
		return CurrentRequestMode
	}
	return receiver.mode
}

// GetRouteVar returns the route variables for the current request, if any
func (receiver RequestReceiverV2) GetRouteVar(key string, r *http.Request) string {
	return GetRouteVar(key, r)
}
