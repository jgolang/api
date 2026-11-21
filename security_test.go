package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateCredentialsMatchesUsernameAndPassword(t *testing.T) {
	previousUsername, previousPassword := Username, Password
	Username, Password = "service-user", "service-password"
	defer func() { Username, Password = previousUsername, previousPassword }()

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{name: "both match", username: "service-user", password: "service-password", want: true},
		{name: "wrong password", username: "service-user", password: "service-passwore", want: false},
		{name: "wrong username", username: "service-usex", password: "service-password", want: false},
		{name: "shorter password", username: "service-user", password: "service", want: false},
		{name: "longer username", username: "service-user-x", password: "service-password", want: false},
		{name: "empty values", username: "", password: "", want: false},
		{name: "swapped values", username: "service-password", password: "service-user", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCredentials(tt.username, tt.password); got != tt.want {
				t.Fatalf("validateCredentials(%q, %q) = %v, want %v", tt.username, tt.password, got, tt.want)
			}
		})
	}
}

func TestBasicTokenAcceptsConfiguredCredentials(t *testing.T) {
	previousUsername, previousPassword := Username, Password
	Username, Password = "service-user", "service-password"
	defer func() { Username, Password = previousUsername, previousPassword }()

	tests := []struct {
		name       string
		credential string
		wantStatus int
	}{
		{name: "valid", credential: "service-user:service-password", wantStatus: http.StatusNoContent},
		{name: "invalid password", credential: "service-user:other", wantStatus: http.StatusUnauthorized},
		{name: "no separator", credential: "service-user", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := BasicToken(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(tt.credential)))
			res := httptest.NewRecorder()

			handler(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.Code)
			}
		})
	}
}

// The README and doc comments warn that the defaults accept default:default.
func TestValidateCredentialsDefaultsAcceptDefaultCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{name: "default credentials", username: "default", password: "default", want: true},
		{name: "other credentials", username: "default", password: "other", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCredentials(tt.username, tt.password); got != tt.want {
				t.Fatalf("validateCredentials(%q, %q) = %v, want %v", tt.username, tt.password, got, tt.want)
			}
		})
	}
}
