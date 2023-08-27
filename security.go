package api

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/jgolang/api/core"
)

var (
	// Username basic authentication
	// Default: admin
	// Change this, it's insecure.
	Username = "default"
	// Password basic authentication
	// Default: admin
	// Change this, it's insecure.
	Password = "default"
)

// SecurityGuaranter implementation of core.APISecurityGuaranter interface.
type SecurityGuaranter struct{}

// ValidateBasicToken validate token with a basic auth token validation method.
func (guaranter *SecurityGuaranter) ValidateBasicToken(token string) (client, secret string, valid bool) {
	payload, _ := base64.StdEncoding.DecodeString(token)
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 || !ValidateBasicAuthCredentialsFunc(pair[0], pair[1]) {
		return "", "", false
	}
	return pair[0], pair[1], true
}

// ValidateCustomToken validate token with a custom method.
func (guaranter *SecurityGuaranter) ValidateCustomToken(token string, validator core.CustomTokenValidator) (json.RawMessage, bool) {
	return validator(token)
}

func validateCredentials(username, password string) bool {
	if username == Username && password == Password {
		return true
	}
	return false
}

// ValidateCustomToken validate token with a custom method.
func ValidateCustomToken(token string) (json.RawMessage, bool) {
	return api.ValidateCustomToken(token, CustomTokenValidatorFunc)
}

// ValidateCredentials func type.
type ValidateCredentials func(string, string) bool

// CustomTokenValidatorFunc define custom function to validate custom token.
var CustomTokenValidatorFunc core.CustomTokenValidator

// ValidateBasicAuthCredentialsFunc define custom function for validate basic authentication credential.
var ValidateBasicAuthCredentialsFunc ValidateCredentials = validateCredentials

// ValidateBasicToken validate token with a basic auth token validation method.
func ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.ValidateBasicToken(token)
}
