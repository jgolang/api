package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestBodyDecodeErrorsAreLoggedWithoutValues checks that the error record of
// a body that cannot be decoded names the field, type and offset but never a
// value of the body.
func TestBodyDecodeErrorsAreLoggedWithoutValues(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
		secret    string
	}{
		{
			name:      "number overflows its field",
			body:      `{"pin_code":123456789}`,
			wantError: `json: cannot unmarshal number into Go struct field pin_code of type int16 (offset 21)`,
			secret:    "123456789",
		},
		{
			name:      "negative number into an unsigned field",
			body:      `{"account":-4242}`,
			wantError: `json: cannot unmarshal number into Go struct field account of type uint32 (offset 16)`,
			secret:    "4242",
		},
		{
			name:      "syntax error",
			body:      `{"pin_code":9x}`,
			wantError: `json: syntax error at offset 14`,
			secret:    "'x'",
		},
		{
			name:      "bad value for a string-tagged number",
			body:      `{"amount":"98765x"}`,
			wantError: "json: cannot unmarshal number into Go struct field amount of type int (offset 18)",
			secret:    "98765x",
		},
		{
			name:      "bad time",
			body:      `{"at":"secret-4711"}`,
			wantError: "request body cannot be read or decoded",
			secret:    "secret-4711",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newRecordingLogger()
			handler := New(WithLogger(logger)).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var in struct {
					PIN     int16     `json:"pin_code"`
					Account uint32    `json:"account"`
					Amount  int       `json:"amount,string"`
					At      time.Time `json:"at"`
				}
				if response := UnmarshalBody(&in, r); response != nil {
					response.Write(w, r)
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(tt.body)))

			record := recordNamed(t, decodeRecords(t, buf), "request body not unmarshaled")
			if record["error"] != tt.wantError {
				t.Fatalf("error = %q, want %q", record["error"], tt.wantError)
			}
			if strings.Contains(buf.String(), tt.secret) {
				t.Fatalf("log %q contains body value %q", buf.String(), tt.secret)
			}
		})
	}
}

// TestBodyTooLargeErrorIsLoggedAsIs checks that an error without body values,
// such as the body limit, keeps its text.
func TestBodyTooLargeErrorIsLoggedAsIs(t *testing.T) {
	logger, buf := newRecordingLogger()
	handler := New(WithLogger(logger), WithMaxBodyBytes(4)).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct{}
		if response := UnmarshalBody(&in, r); response != nil {
			response.Write(w, r)
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{"a":1}`)))

	record := recordNamed(t, decodeRecords(t, buf), "request body not unmarshaled")
	if record["error"] != "http: request body too large" {
		t.Fatalf("error = %q, want the body limit error", record["error"])
	}
}

// TestRejectedBodyIsLoggedWithoutValues checks the ProcessRequest and
// RequestBody records of a body that is not valid JSON.
func TestRejectedBodyIsLoggedWithoutValues(t *testing.T) {
	for name, middleware := range map[string]func(http.HandlerFunc) http.HandlerFunc{
		"ProcessRequest": ProcessRequest,
		"RequestBody":    RequestBody,
	} {
		t.Run(name, func(t *testing.T) {
			logger, buf := newRecordingLogger()
			handler := New(WithLogger(logger), WithRequestMode(RequestModeEnvelope)).Handler(middleware(okHandler))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{"content":7x}`)))

			record := recordNamed(t, decodeRecords(t, buf), "request body rejected")
			if record["error"] != "json: syntax error at offset 13" {
				t.Fatalf("error = %q, want the offset only", record["error"])
			}
			if strings.Contains(buf.String(), "'x'") {
				t.Fatalf("log %q contains the body byte", buf.String())
			}
		})
	}
}

// TestTopLevelTypeErrorIsLoggedWithoutValue checks the description of a type
// error outside a struct field.
func TestTopLevelTypeErrorIsLoggedWithoutValue(t *testing.T) {
	logger, buf := newRecordingLogger()
	handler := New(WithLogger(logger)).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pin int16
		if response := UnmarshalBody(&pin, r); response != nil {
			response.Write(w, r)
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`123456789`)))

	record := recordNamed(t, decodeRecords(t, buf), "request body not unmarshaled")
	if want := "json: cannot unmarshal number into Go value of type int16 (offset 9)"; record["error"] != want {
		t.Fatalf("error = %q, want %q", record["error"], want)
	}
	if strings.Contains(buf.String(), "123456789") {
		t.Fatalf("log %q contains the body value", buf.String())
	}
}
