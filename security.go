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
	// validator compares against. Default: "default".
	//
	// While Username or Password is empty or "default", the default
	// validator rejects every request, so BasicToken answers 401. Set both
	// (or replace ValidateBasicAuthCredentialsFunc) before using BasicToken.
	Username = "default"
	// Password is the password that the default basic authentication
	// validator compares against. Default: "default". See Username.
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

// ValidateCustomToken validate token with a custom method. A nil validator
// accepts no token.
func (guaranter *SecurityGuaranter) ValidateCustomToken(token string, validator core.CustomTokenValidator) (json.RawMessage, bool) {
	if validator == nil {
		return nil, false
	}
	return validator(token)
}

// validateCredentials rejects every pair while the credentials are not
// configured, so the publicly known defaults never open a route. Otherwise
// it compares both values in constant time. Hashing first gives both sides
// the same length, so the compare does not return early on a length
// mismatch, and both compares always run.
func validateCredentials(username, password string) bool {
	if !credentialsConfigured() {
		return false
	}
	usernameMatch := constantTimeEqual(username, Username)
	passwordMatch := constantTimeEqual(password, Password)
	return usernameMatch&passwordMatch == 1
}

func credentialsConfigured() bool {
	return isConfiguredCredential(Username) && isConfiguredCredential(Password)
}

func isConfiguredCredential(value string) bool {
	return value != "" && value != "default"
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
// While it is nil (the default), every token is invalid and CustomToken
// answers 401.
var CustomTokenValidatorFunc core.CustomTokenValidator

// ValidateBasicAuthCredentialsFunc define custom function for validate basic authentication credential.
// The default rejects every pair while Username or Password is empty or
// "default", and otherwise compares against them in constant time; a
// replacement should compare secrets in constant time as well (crypto/subtle).
var ValidateBasicAuthCredentialsFunc ValidateCredentials = validateCredentials

// ValidateBasicToken validate token with a basic auth token validation method.
func ValidateBasicToken(token string) (client, secret string, valid bool) {
	return api.ValidateBasicToken(token)
}
