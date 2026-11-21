package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/jgolang/api/core"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// planValidation is a service-declared code with an upper-case prefix.
var planValidation = codes.New("PLN-001", "title is required").WithKind(codes.KindValidation)

var codeWriters = map[string]http.HandlerFunc{
	"WriteError": func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, errors.NewC(planValidation, "title"))
	},
	"Error400": func(w http.ResponseWriter, r *http.Request) {
		Error400(errors.NewC(planValidation, "title")).Write(w, r)
	},
	"Error400 default code": func(w http.ResponseWriter, r *http.Request) { Error400().Write(w, r) },
	"Success":               func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) },
	"Warning": func(w http.ResponseWriter, r *http.Request) {
		Warning{HTTPStatusCode: http.StatusOK}.Write(w, r)
	},
	"Informative": func(w http.ResponseWriter, r *http.Request) {
		Informative{HTTPStatusCode: http.StatusOK}.Write(w, r)
	},
}

// codeAndMessage decodes the code and message of a plain or envelope body. It
// returns empty values for any other body, and is safe to call from any
// goroutine.
func codeAndMessage(body []byte) (code, message string) {
	var fields struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Header  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"header"`
	}
	if err := json.Unmarshal(body, &fields); err != nil {
		return "", ""
	}
	if fields.Header != nil {
		return fields.Header.Code, fields.Header.Message
	}
	return fields.Code, fields.Message
}

// serveCode serves writer with a fixed event ID, wrapped by a unless a is nil.
func serveCode(t *testing.T, a *API, writer string) (code, message string) {
	t.Helper()
	var handler http.Handler = codeWriters[writer]
	if a != nil {
		handler = a.Handler(handler)
	}
	rec := serve(handler, http.MethodGet, "", map[string]string{EventIDHeaderKey: "evt-123456789"})
	return codeAndMessage(rec.Body.Bytes())
}

func TestInstanceCodeCase(t *testing.T) {
	tests := []struct {
		name        string
		instance    func(mode ResponseMode) *API
		writer      string
		wantCode    string
		wantMessage string
	}{
		{"declared WriteError", declaredInstance, "WriteError", "PLN-001", "title is required (evt-12-PLN-001)"},
		{"declared Error400", declaredInstance, "Error400", "PLN-001", "title is required (evt-12-PLN-001)"},
		{"lowercase WriteError", lowercaseInstance, "WriteError", "pln-001", "title is required (evt-12-pln-001)"},
		{"lowercase Error400", lowercaseInstance, "Error400", "pln-001", "title is required (evt-12-pln-001)"},
		{"default instance WriteError", noInstance, "WriteError", "pln-001", "title is required (evt-12-pln-001)"},
		{"default instance Error400", noInstance, "Error400", "pln-001", "title is required (evt-12-pln-001)"},
		{"declared default error code", declaredInstance, "Error400 default code", "E001", "The service has not completed the operation! (evt-12-E001)"},
		{"declared success", declaredInstance, "Success", "0000", ""},
		{"declared warning", declaredInstance, "Warning", "0000", "The application has been successful but with potential problems! (evt-12-0000)"},
		{"declared informative", declaredInstance, "Informative", "0000", "The request has been successful! (evt-12-0000)"},
	}

	for _, mode := range []ResponseMode{ResponseModePlain, ResponseModeEnvelope} {
		for _, tt := range tests {
			t.Run(string(mode)+"/"+tt.name, func(t *testing.T) {
				useFormatterSettings(t, true)
				usePackageModes(t, RequestModePlain, mode)
				a := tt.instance(mode)
				// An instance existing must not change the default instance.
				_ = New(WithResponseMode(mode))

				code, message := serveCode(t, a, tt.writer)

				if code != tt.wantCode || message != tt.wantMessage {
					t.Fatalf("code, message = %q, %q; want %q, %q", code, message, tt.wantCode, tt.wantMessage)
				}
			})
		}
	}
}

func declaredInstance(mode ResponseMode) *API { return New(WithResponseMode(mode)) }

func lowercaseInstance(mode ResponseMode) *API {
	return New(WithResponseMode(mode), WithLowercaseCodes())
}

func noInstance(ResponseMode) *API { return nil }

func TestBuiltInFormatterPassedAsOptionTakesTheInstanceCodeCase(t *testing.T) {
	tests := []struct {
		name      string
		formatter core.APIResponseFormatter
		lowercase bool
		wantCode  string
	}{
		{"value declared", ResponseFormatter{}, false, "PLN-001"},
		{"pointer declared", &ResponseFormatter{}, false, "PLN-001"},
		{"value lowercase", ResponseFormatter{}, true, "pln-001"},
		{"pointer lowercase", &ResponseFormatter{}, true, "pln-001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFormatterSettings(t, true)
			opts := []Option{WithResponseFormatter(tt.formatter)}
			if tt.lowercase {
				opts = append(opts, WithLowercaseCodes())
			}

			code, message := serveCode(t, New(opts...), "WriteError")

			if wantMessage := "title is required (evt-12-" + tt.wantCode + ")"; code != tt.wantCode || message != wantMessage {
				t.Fatalf("code, message = %q, %q; want %q, %q", code, message, tt.wantCode, wantMessage)
			}
		})
	}
}

func TestLowercaseCodesDoesNotApplyToACustomFormatter(t *testing.T) {
	formatter := &fakeFormatter{}
	a := New(WithResponseFormatter(formatter), WithLowercaseCodes())

	serve(a.Handler(codeWriters["WriteError"]), http.MethodGet, "", nil)

	if got := formatter.data.ResponseCode; got != "PLN-001" {
		t.Fatalf("custom formatter got code %q, want PLN-001 as declared", got)
	}
}

func TestInstancesServeConcurrentlyWithTheirOwnCodeCase(t *testing.T) {
	useFormatterSettings(t, true)
	handlers := map[string]http.Handler{
		"PLN-001": New().Handler(codeWriters["WriteError"]),
		"pln-001": New(WithLowercaseCodes()).Handler(codeWriters["WriteError"]),
	}
	// The default instance lowercases as in v0.
	defaultHandler := codeWriters["WriteError"]

	var wg sync.WaitGroup
	for range 20 {
		for want, handler := range handlers {
			wg.Go(func() {
				rec := serve(handler, http.MethodGet, "", nil)
				if code, _ := codeAndMessage(rec.Body.Bytes()); code != want {
					t.Errorf("code = %q, want %q", code, want)
				}
			})
		}
		wg.Go(func() {
			rec := serve(defaultHandler, http.MethodGet, "", nil)
			if code, _ := codeAndMessage(rec.Body.Bytes()); code != "pln-001" {
				t.Errorf("default instance code = %q, want pln-001", code)
			}
		})
	}
	wg.Wait()
}
