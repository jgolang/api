package api

import (
	"fmt"
	"net/http"
	"sync"
)

// AuthResult contains normalized authentication data returned by an AuthProvider.
type AuthResult struct {
	Subject string
	Token   string
	Claims  map[string]any
}

// AuthProvider authenticates an HTTP request.
type AuthProvider interface {
	Authenticate(r *http.Request) (*AuthResult, error)
}

var authRegistry = struct {
	sync.RWMutex
	providers map[string]AuthProvider
}{
	providers: make(map[string]AuthProvider),
}

// RegisterAuth registers an authentication provider.
func RegisterAuth(name string, provider AuthProvider) {
	if name == "" || provider == nil {
		return
	}
	authRegistry.Lock()
	defer authRegistry.Unlock()
	authRegistry.providers[name] = provider
}

// AuthByName returns a registered authentication provider.
func AuthByName(name string) (AuthProvider, error) {
	authRegistry.RLock()
	provider, ok := authRegistry.providers[name]
	authRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("api auth provider %q is not registered", name)
	}
	return provider, nil
}
