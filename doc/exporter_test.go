package doc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type testExporter struct{}

func (exporter testExporter) Handler(openAPIURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(openAPIURL))
	}
}

func TestDefaultSwaggerExporterIsRegistered(t *testing.T) {
	exporter, err := ExporterByName("swagger")
	if err != nil {
		t.Fatalf("expected swagger exporter, got error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	res := httptest.NewRecorder()
	exporter.Handler("/openapi.json").ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if res.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", res.Header().Get("Content-Type"))
	}
}

func TestRegisteredExporterCanBeResolved(t *testing.T) {
	RegisterExporter("test-exporter", testExporter{})

	exporter, err := ExporterByName("test-exporter")
	if err != nil {
		t.Fatalf("expected registered exporter, got error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	res := httptest.NewRecorder()
	exporter.Handler("/openapi.json").ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	if res.Body.String() != "/openapi.json" {
		t.Fatalf("unexpected body: %q", res.Body.String())
	}
}

func TestExporterByNameReturnsErrorForMissingExporter(t *testing.T) {
	exporter, err := ExporterByName("missing-exporter")
	if err == nil {
		t.Fatalf("expected error")
	}
	if exporter != nil {
		t.Fatalf("expected nil exporter, got %#v", exporter)
	}
}

func TestRegisterExporterIgnoresInvalidInput(t *testing.T) {
	RegisterExporter("", testExporter{})
	RegisterExporter("nil-exporter", nil)

	if _, err := ExporterByName(""); err == nil {
		t.Fatalf("empty exporter name should not be registered")
	}
	if _, err := ExporterByName("nil-exporter"); err == nil {
		t.Fatalf("nil exporter should not be registered")
	}
}
