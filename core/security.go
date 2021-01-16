package core

import (
	"encoding/json"
)

// APISecurity doc ...
type APISecurity interface {
	ValidateBasicToken(token string) (client, secret string, valid bool)
	ValidateCustomToken(token string, validator CustomTokenValidator) (json.RawMessage, bool)
}

// CustomTokenValidator validator custom token func
type CustomTokenValidator func(string) (json.RawMessage, bool)
