package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProcessRequestLogsPanicValueWithoutRequest(t *testing.T) {
	previousPrintError := PrintError
	var logged string
	PrintError = func(args ...interface{}) { logged += fmt.Sprintln(args...) }
	defer func() { PrintError = previousPrintError }()

	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		panic("handler panic value")
	})
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Authorization", "Bearer bearer-placeholder")
	req.Header.Set("Cookie", "session=cookie-placeholder")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", res.Code)
	}
	if !strings.Contains(logged, "handler panic value") {
		t.Fatalf("expected the panic value to be logged, got %q", logged)
	}
	for _, secret := range []string{"bearer-placeholder", "cookie-placeholder"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("expected %q to stay out of the error log, got %q", secret, logged)
		}
	}
}
