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
	Username = "admin"
	// Password basic authentication
	// Default: admin
	Password = "admin"
)

// Security core interface implement
type Security struct{}

// ValidateBasicToken doc ...
func (s *Security) ValidateBasicToken(token string) (client, secret string, valid bool) {
	payload, _ := base64.StdEncoding.DecodeString(token)
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 || !ValidateBasicAuthCredentialsFunc(pair[0], pair[1]) {
		return "", "", false
	}
	return pair[0], pair[1], true
}

// ValidateCustomToken doc ...
func (s *Security) ValidateCustomToken(token string, validator core.CustomTokenValidator) (json.RawMessage, bool) {
	return validator(token)
}

func validateCredentials(username, password string) bool {
	if username == Username && password == Password {
		return true
	}
	return false
}

// ValidateCustomToken doc ...
func ValidateCustomToken(token string) (json.RawMessage, bool) {
	return api.ValidateCustomToken(token, CustomTokenValidatorFunc)
}

// ValidateCredentials func doc ...
type ValidateCredentials func(string, string) bool

// CustomTokenValidatorFunc define custom function to validate custom token
var CustomTokenValidatorFunc core.CustomTokenValidator

// ValidateBasicAuthCredentialsFunc define custom function for validate basic authenteication credential
var ValidateBasicAuthCredentialsFunc ValidateCredentials = validateCredentials

// ValidateBasicToken doc ...
func ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.ValidateBasicToken(token)
}
