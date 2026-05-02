package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProcessRequestAllowsGETWithoutBodyInPlainMode(t *testing.T) {
	previousMode := CurrentRequestMode
	SetRequestMode(RequestModePlain)
	defer SetRequestMode(previousMode)

	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		requestData, err := GetRequestContext(r)
		if err != nil {
			t.Fatalf("expected request context, got error: %v", err)
		}
		if len(requestData.Body()) != 0 {
			t.Fatalf("expected empty body for GET request, got %q", string(requestData.Body()))
		}
		Success{Content: map[string]string{"status": "ok"}}.Write(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", res.Code)
	}
}

func TestProcessRequestParsesEnvelopeWhenConfigured(t *testing.T) {
	previousMode := CurrentRequestMode
	SetRequestMode(RequestModeEnvelope)
	defer SetRequestMode(previousMode)

	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		requestData, err := GetRequestContext(r)
		if err != nil {
			t.Fatalf("expected request context, got error: %v", err)
		}
		var payload struct {
			Title string `json:"title"`
		}
		if err := requestData.DecodeContent(&payload); err != nil {
			t.Fatalf("expected envelope content to decode, got error: %v", err)
		}
		if payload.Title != "task" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if requestData.SecurityToken != "header-token" {
			t.Fatalf("expected HTTP header metadata to win, got %q", requestData.SecurityToken)
		}
		Success{}.Write(w, r)
	})

	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"header":{"token":"body-token"},"content":{"title":"task"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SecurityTokenHeaderKey, "header-token")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", res.Code)
	}
}

func TestRequestHeaderJSONAcceptsCharset(t *testing.T) {
	handler := RequestHeaderJSON(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"title":"task"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", res.Code)
	}
}

func TestCustomTokenReturnsUnauthorizedForMalformedHeader(t *testing.T) {
	previousResponseMode := CurrentResponseMode
	SetResponseMode(ResponseModePlain)
	defer SetResponseMode(previousResponseMode)

	handler := CustomToken(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Authorization", "invalid")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: %d", res.Code)
	}
	if res.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected WWW-Authenticate header")
	}
}

func TestRequestBodySkipsGETByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tasks", bytes.NewBufferString(`not-json`))
	res := httptest.NewRecorder()

	RequestBody(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", res.Code)
	}
}

func TestResponseModeNoneWritesRawContent(t *testing.T) {
	previousMode := CurrentResponseMode
	SetResponseMode(ResponseModeNone)
	defer SetResponseMode(previousMode)

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	res := httptest.NewRecorder()

	Success{Content: map[string]string{"status": "ok"}}.Write(res, req)

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("expected JSON body, got error: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected response body: %#v", body)
	}
}
