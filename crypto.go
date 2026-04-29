package api

import (
	"context"
	"fmt"
	"sync"
)

// CryptoProvider encrypts and decrypts payload bytes.
type CryptoProvider interface {
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

var cryptoRegistry = struct {
	sync.RWMutex
	providers map[string]CryptoProvider
}{
	providers: make(map[string]CryptoProvider),
}

// RegisterCrypto registers an encryption/decryption provider.
func RegisterCrypto(name string, provider CryptoProvider) {
	if name == "" || provider == nil {
		return
	}
	cryptoRegistry.Lock()
	defer cryptoRegistry.Unlock()
	cryptoRegistry.providers[name] = provider
}

// CryptoByName returns a registered encryption/decryption provider.
func CryptoByName(name string) (CryptoProvider, error) {
	cryptoRegistry.RLock()
	provider, ok := cryptoRegistry.providers[name]
	cryptoRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("api crypto provider %q is not registered", name)
	}
	return provider, nil
}
