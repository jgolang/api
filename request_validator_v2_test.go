package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
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

			if errors.CodeOf(err) != codes.ValRequired {
				t.Fatalf("expected the ValRequired code, got %v", err)
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

func TestProcessBodyDeviceFields(t *testing.T) {
	tests := []struct {
		name    string
		request func() *http.Request
	}{
		{name: "envelope header", request: func() *http.Request {
			body := `{"header":{"device_serial":"serial-1","device_id":"id-1","lat":"19.4","lon":"-99.1"},"content":{}}`
			return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		}},
		{name: "HTTP headers", request: func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Header.Set("DeviceSerial", "serial-1")
			r.Header.Set("DeviceID", "id-1")
			r.Header.Set("Latitude", "19.4")
			r.Header.Set("Longitude", "-99.1")
			return r
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := RequestReceiver{mode: RequestModeEnvelope}.ProcessBody(tt.request())
			if err != nil {
				t.Fatalf("v1: unexpected error %v", err)
			}
			v2, err := RequestReceiverV2{mode: RequestModeEnvelope}.ProcessBody(tt.request())
			if err != nil {
				t.Fatalf("v2: unexpected error %v", err)
			}

			if v1.DeviceSerial != "serial-1" || v1.DeviceId != "id-1" || v1.Latitude != "19.4" || v1.Longitude != "-99.1" {
				t.Fatalf("v1: expected the four device fields, got %+v", v1)
			}
			if v2.DeviceSerial != "" || v2.DeviceId != "" || v2.Latitude != "" || v2.Longitude != "" {
				t.Fatalf("v2: expected empty device fields, got %+v", v2)
			}
		})
	}
}
