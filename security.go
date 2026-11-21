package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/jgolang/api/core"
)

var (
	// Username is the user name that the default basic authentication
	// validator accepts. Default: "default".
	//
	// Warning: with the defaults, BasicToken accepts the publicly known
	// credentials "default:default". Set Username and Password (or
	// ValidateBasicAuthCredentialsFunc) before using BasicToken.
	Username = "default"
	// Password is the password that the default basic authentication
	// validator accepts. Default: "default". See the warning on Username.
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

// validateCredentials compares both values in constant time. Hashing first
// gives both sides the same length, so the compare does not return early on
// a length mismatch, and both compares always run.
func validateCredentials(username, password string) bool {
	usernameMatch := constantTimeEqual(username, Username)
	passwordMatch := constantTimeEqual(password, Password)
	return usernameMatch&passwordMatch == 1
}

func constantTimeEqual(got, want string) int {
	gotSum := sha256.Sum256([]byte(got))
	wantSum := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotSum[:], wantSum[:])
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
// The default compares against Username and Password in constant time; a
// replacement should compare secrets in constant time as well (crypto/subtle).
var ValidateBasicAuthCredentialsFunc ValidateCredentials = validateCredentials

// ValidateBasicToken validate token with a basic auth token validation method.
func ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.ValidateBasicToken(token)
}
