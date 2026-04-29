package api

import (
	"net/http"
	"testing"
)

type testAuthProvider struct{}

func (provider testAuthProvider) Authenticate(r *http.Request) (*AuthResult, error) {
	return &AuthResult{
		Subject: "user-123",
		Token:   r.Header.Get("Authorization"),
		Claims: map[string]any{
			"role": "admin",
		},
	}, nil
}

func TestRegisteredAuthProviderCanBeResolved(t *testing.T) {
	RegisterAuth("test-auth", testAuthProvider{})

	provider, err := AuthByName("test-auth")
	if err != nil {
		t.Fatalf("expected registered provider, got error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	result, err := provider.Authenticate(req)
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if result.Subject != "user-123" || result.Claims["role"] != "admin" {
		t.Fatalf("unexpected auth result: %#v", result)
	}
}

func TestAuthByNameReturnsErrorForMissingProvider(t *testing.T) {
	provider, err := AuthByName("missing-auth")
	if err == nil {
		t.Fatalf("expected error")
	}
	if provider != nil {
		t.Fatalf("expected nil provider, got %#v", provider)
	}
}

func TestRegisterAuthIgnoresInvalidInput(t *testing.T) {
	RegisterAuth("", testAuthProvider{})
	RegisterAuth("nil-auth", nil)

	if _, err := AuthByName(""); err == nil {
		t.Fatalf("empty auth name should not be registered")
	}
	if _, err := AuthByName("nil-auth"); err == nil {
		t.Fatalf("nil auth provider should not be registered")
	}
}
