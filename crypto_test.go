package api

import (
	"context"
	"errors"
	"testing"
)

type testCryptoProvider struct {
	err error
}

func (provider testCryptoProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	return append([]byte("enc:"), plaintext...), nil
}

func (provider testCryptoProvider) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	return append([]byte("dec:"), ciphertext...), nil
}

func TestRegisteredCryptoProviderCanBeResolved(t *testing.T) {
	RegisterCrypto("test-crypto", testCryptoProvider{})

	provider, err := CryptoByName("test-crypto")
	if err != nil {
		t.Fatalf("expected registered provider, got error: %v", err)
	}
	encrypted, err := provider.Encrypt(context.Background(), []byte("payload"))
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if string(encrypted) != "enc:payload" {
		t.Fatalf("unexpected encrypted payload: %q", encrypted)
	}
	decrypted, err := provider.Decrypt(context.Background(), []byte("payload"))
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if string(decrypted) != "dec:payload" {
		t.Fatalf("unexpected decrypted payload: %q", decrypted)
	}
}

func TestRegisteredCryptoProviderReturnsProviderError(t *testing.T) {
	expected := errors.New("crypto failed")
	RegisterCrypto("failing-crypto", testCryptoProvider{err: expected})

	provider, err := CryptoByName("failing-crypto")
	if err != nil {
		t.Fatalf("expected registered provider, got error: %v", err)
	}
	if _, err := provider.Encrypt(context.Background(), []byte("payload")); !errors.Is(err, expected) {
		t.Fatalf("expected crypto error %v, got %v", expected, err)
	}
}

func TestCryptoByNameReturnsErrorForMissingProvider(t *testing.T) {
	provider, err := CryptoByName("missing-crypto")
	if err == nil {
		t.Fatalf("expected error")
	}
	if provider != nil {
		t.Fatalf("expected nil provider, got %#v", provider)
	}
}

func TestRegisterCryptoIgnoresInvalidInput(t *testing.T) {
	RegisterCrypto("", testCryptoProvider{})
	RegisterCrypto("nil-crypto", nil)

	if _, err := CryptoByName(""); err == nil {
		t.Fatalf("empty crypto name should not be registered")
	}
	if _, err := CryptoByName("nil-crypto"); err == nil {
		t.Fatalf("nil crypto provider should not be registered")
	}
}
