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

// Until Username and Password are both set to values other than "" and
// "default", the default validator rejects every credential pair.
func TestValidateCredentialsRejectsAllWhileNotConfigured(t *testing.T) {
	configurations := []struct {
		name     string
		username string
		password string
	}{
		{name: "defaults", username: "default", password: "default"},
		{name: "empty", username: "", password: ""},
		{name: "default password", username: "service-user", password: "default"},
		{name: "default username", username: "default", password: "service-password"},
		{name: "empty password", username: "service-user", password: ""},
		{name: "empty username", username: "", password: "service-password"},
	}
	for _, configuration := range configurations {
		t.Run(configuration.name, func(t *testing.T) {
			previousUsername, previousPassword := Username, Password
			Username, Password = configuration.username, configuration.password
			defer func() { Username, Password = previousUsername, previousPassword }()

			credentials := [][2]string{
				{configuration.username, configuration.password},
				{"default", "default"},
				{"", ""},
			}
			for _, credential := range credentials {
				if validateCredentials(credential[0], credential[1]) {
					t.Fatalf("validateCredentials(%q, %q) = true with Username %q and Password %q, want false", credential[0], credential[1], Username, Password)
				}
			}
		})
	}
}

func TestBasicTokenRejectsDefaultCredentialsWhileNotConfigured(t *testing.T) {
	previousUsername, previousPassword := Username, Password
	Username, Password = "default", "default"
	defer func() { Username, Password = previousUsername, previousPassword }()

	nextCalled := false
	handler := BasicToken(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("default:default")))
	res := httptest.NewRecorder()

	handler(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
	if nextCalled {
		t.Fatal("expected next handler not to run")
	}
}

// A replacement validator decides on its own; the configuration rule belongs
// to the default validator only.
func TestBasicTokenUsesCustomValidatorWhileNotConfigured(t *testing.T) {
	previousUsername, previousPassword, previousValidator := Username, Password, ValidateBasicAuthCredentialsFunc
	Username, Password = "default", "default"
	ValidateBasicAuthCredentialsFunc = func(username, password string) bool {
		return username == "custom-user" && password == "custom-password"
	}
	defer func() {
		Username, Password, ValidateBasicAuthCredentialsFunc = previousUsername, previousPassword, previousValidator
	}()

	handler := BasicToken(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("custom-user:custom-password")))
	res := httptest.NewRecorder()

	handler(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}
