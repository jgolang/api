package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProcessRequestLogsPanicTypeWithoutValueOrRequest(t *testing.T) {
	buf := captureDefaultLogger(t)
	failOnPackageLoggers(t)

	handler := ProcessRequest(func(w http.ResponseWriter, r *http.Request) {
		panic("handler panic value")
	})
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Authorization", "Bearer bearer-placeholder")
	req.Header.Set("Cookie", "session=cookie-placeholder")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	logged := buf.String()

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", res.Code)
	}
	if !strings.Contains(logged, `"panic":"[REDACTED]"`) || !strings.Contains(logged, `"panic_type":"string"`) {
		t.Fatalf("expected the panic value masked and its type logged, got %q", logged)
	}
	for _, secret := range []string{"handler panic value", "bearer-placeholder", "cookie-placeholder"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("expected %q to stay out of the error log, got %q", secret, logged)
		}
	}
}
