package core

import (
	"encoding/json"
)

// APISecurityGuarantor you can to implement this interface to implement
// authentication methos.
type APISecurityGuarantor interface {
	// ValidateBasicToken validate a token with a basic authentication method.
	ValidateBasicToken(token string) (client, secret string, valid bool)
	// ValidateBasicToken validate a token with a custmo authentication method.
	ValidateCustomToken(token string, validator CustomTokenValidator) (json.RawMessage, bool)
}

// CustomTokenValidator validator custom token function type.
// Implement this type to creat a custom token validation method
// as bearer authentication method or specific company methods.
type CustomTokenValidator func(string) (json.RawMessage, bool)
