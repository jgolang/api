package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/jgolang/api/core"
)

// tokenWriters write each response kind without setting a token.
var tokenWriters = map[string]http.HandlerFunc{
	"success":     func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) },
	"error401":    func(w http.ResponseWriter, r *http.Request) { Error401().Write(w, r) },
	"writeError":  func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, nil) },
	"warning":     func(w http.ResponseWriter, r *http.Request) { Warning{HTTPStatusCode: http.StatusOK}.Write(w, r) },
	"informative": func(w http.ResponseWriter, r *http.Request) { Informative{HTTPStatusCode: http.StatusOK}.Write(w, r) },
}

// tokenRequests carry the request token in each place v0 read it from to echo it.
var tokenRequests = map[string]map[string]string{
	"bearer":       {"Authorization": "Bearer request-token"},
	"token header": {SecurityTokenHeaderKey: "request-token"},
}

func responseToken(t *testing.T, handler http.Handler, headers map[string]string) any {
	t.Helper()
	rec := serve(handler, http.MethodGet, "", headers)
	return decodeMembers(t, rec.Body.Bytes())["token"]
}

func TestInstanceDoesNotEchoTheRequestToken(t *testing.T) {
	a := New(WithResponseMode(ResponseModePlain))
	for writer, handler := range tokenWriters {
		for source, headers := range tokenRequests {
			t.Run(writer+"/"+source, func(t *testing.T) {
				if got := responseToken(t, a.Handler(handler), headers); got != "" {
					t.Fatalf("token = %v, want empty", got)
				}
			})
		}
	}
}

func TestInstanceEnvelopeOmitsTheRequestToken(t *testing.T) {
	a := New()
	for writer, handler := range tokenWriters {
		for source, headers := range tokenRequests {
			t.Run(writer+"/"+source, func(t *testing.T) {
				rec := serve(a.Handler(handler), http.MethodGet, "", headers)

				header, _ := decodeMembers(t, rec.Body.Bytes())["header"].(map[string]any)
				if token, ok := header["token"]; ok {
					t.Fatalf("header.token = %v, want no member", token)
				}
			})
		}
	}
}

func TestInstanceUnderProcessRequestDoesNotEchoTheEnvelopeToken(t *testing.T) {
	a := New(WithRequestMode(RequestModeEnvelope), WithResponseMode(ResponseModePlain))
	handler := a.Handler(ProcessRequest(tokenWriters["success"]))

	rec := serve(handler, http.MethodPost, `{"header":{"token":"body-token"},"content":{}}`, nil)

	if got := decodeMembers(t, rec.Body.Bytes())["token"]; got != "" {
		t.Fatalf("token = %v, want empty", got)
	}
}

// tokenFormatter is a custom formatter whose body is the response token.
var tokenFormatter = formatterFunc(func(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{HTTPStatusCode: http.StatusOK, Body: map[string]string{"token": data.SecurityToken}}
})

func TestATokenTheHandlerSetsIsSent(t *testing.T) {
	setToken := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Success{SecurityToken: "renewed-token"}.Write(w, r)
	})
	tests := []struct {
		name      string
		handler   http.Handler
		tokenPath func(members map[string]any) any
	}{
		{
			name:      "instance plain",
			handler:   New(WithResponseMode(ResponseModePlain)).Handler(setToken),
			tokenPath: func(m map[string]any) any { return m["token"] },
		},
		{
			name:    "instance envelope",
			handler: New().Handler(setToken),
			tokenPath: func(m map[string]any) any {
				header, _ := m["header"].(map[string]any)
				return header["token"]
			},
		},
		{
			name:      "instance custom formatter",
			handler:   New(WithResponseFormatter(tokenFormatter)).Handler(setToken),
			tokenPath: func(m map[string]any) any { return m["token"] },
		},
		{
			name:      "default instance",
			handler:   setToken,
			tokenPath: func(m map[string]any) any { return m["token"] },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usePackageModes(t, RequestModePlain, ResponseModePlain)

			rec := serve(tt.handler, http.MethodGet, "", tokenRequests["bearer"])

			if got := tt.tokenPath(decodeMembers(t, rec.Body.Bytes())); got != "renewed-token" {
				t.Fatalf("token = %v, want renewed-token", got)
			}
		})
	}
}

func TestInstanceCustomFormatterGetsNoRequestToken(t *testing.T) {
	a := New(WithResponseFormatter(tokenFormatter))

	if got := responseToken(t, a.Handler(tokenWriters["success"]), tokenRequests["bearer"]); got != "" {
		t.Fatalf("token = %v, want empty", got)
	}
}

func TestDefaultInstanceDoesNotEchoTheRequestTokenFromAnySource(t *testing.T) {
	usePackageModes(t, RequestModePlain, ResponseModePlain)
	for writer, handler := range tokenWriters {
		for source, headers := range tokenRequests {
			t.Run(writer+"/"+source, func(t *testing.T) {
				if got := responseToken(t, handler, headers); got != "" {
					t.Fatalf("token = %v, want empty", got)
				}
			})
		}
	}
}

func TestRequestHeaderSessionResponseHeader(t *testing.T) {
	writeSuccess := func(w http.ResponseWriter, r *http.Request) { Success200().Write(w, r) }
	setRenewedToken := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(SecurityTokenHeaderKey, "renewed-token")
		Success200().Write(w, r)
	}
	tests := []struct {
		name    string
		handler http.Handler
		want    []string
	}{
		{name: "instance does not copy the token", handler: New().Handler(RequestHeaderSession(writeSuccess)), want: nil},
		{name: "default instance does not copy the token", handler: RequestHeaderSession(writeSuccess), want: nil},
		// A guard for the documented way to return a token on an instance.
		{name: "instance keeps a token the handler sets", handler: New().Handler(RequestHeaderSession(setRenewedToken)), want: []string{"renewed-token"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.handler, http.MethodGet, "", map[string]string{SecurityTokenHeaderKey: "request-token"})

			if got := rec.Header().Values(SecurityTokenHeaderKey); fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Fatalf("%s response header = %q, want %q", SecurityTokenHeaderKey, got, tt.want)
			}
		})
	}
}
