package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func encryptedBodyRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestProcessEncryptedBodyAcceptsCompletePayload(t *testing.T) {
	r := encryptedBodyRequest("data=cipher&metadata=meta")

	got, err := RequestReceiverV2{}.ProcessEncryptedBody(r)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Data != "cipher" || got.Metadata != "meta" {
		t.Fatalf("expected data and metadata, got %+v", got)
	}
}

func TestProcessEncryptedBodyRejectsIncompletePayload(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing metadata", body: "data=cipher"},
		{name: "missing data", body: "metadata=meta"},
		{name: "empty", body: ""},
		{name: "empty value", body: "data=&metadata=meta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RequestReceiverV2{}.ProcessEncryptedBody(encryptedBodyRequest(tt.body))

			if err == nil || err.Error() != "does not provide required information" {
				t.Fatalf("expected the missing-information error, got %v", err)
			}
		})
	}
}

func TestProcessEncryptedBodyReturnsParseError(t *testing.T) {
	r := encryptedBodyRequest("data=%zz&metadata=meta")

	if _, err := (RequestReceiverV2{}).ProcessEncryptedBody(r); err == nil || !strings.Contains(err.Error(), "invalid URL escape") {
		t.Fatalf("expected the form parse error, got %v", err)
	}
}
