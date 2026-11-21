package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomTokenAnswers401WithoutValidator(t *testing.T) {
	previousValidator := CustomTokenValidatorFunc
	CustomTokenValidatorFunc = nil
	defer func() { CustomTokenValidatorFunc = previousValidator }()

	var nextCalled bool
	handler := CustomToken(func(w http.ResponseWriter, r *http.Request) { nextCalled = true })
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", CustomTokenPrefix+" token-placeholder")
	res := httptest.NewRecorder()

	handler(res, req)

	if nextCalled {
		t.Fatal("expected the next handler not to run")
	}
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}
}

func TestCustomTokenRunsNextWithValidToken(t *testing.T) {
	previousValidator := CustomTokenValidatorFunc
	CustomTokenValidatorFunc = func(token string) (json.RawMessage, bool) {
		return json.RawMessage(`{"user":"u1"}`), token == "valid-token"
	}
	defer func() { CustomTokenValidatorFunc = previousValidator }()

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "valid token", token: "valid-token", wantStatus: http.StatusNoContent},
		{name: "invalid token", token: "other-token", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CustomToken(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", CustomTokenPrefix+" "+tt.token)
			res := httptest.NewRecorder()

			handler(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.Code)
			}
		})
	}
}
