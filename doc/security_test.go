package doc

import (
	"encoding/json"
	"strings"
	"testing"
)

func generatedJSON(t *testing.T, docs *Docs) string {
	t.Helper()
	raw, err := json.Marshal(GenerateOpenAPI(docs))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func TestPublicRouteWritesEmptySecurity(t *testing.T) {
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("GET", "/health", Public())
	docs.Register("GET", "/tasks", Security("bearer"))
	docs.Register("GET", "/plain")

	operations := GenerateOpenAPI(docs).Paths
	if !operations["/health"]["get"].Public {
		t.Fatal("expected the public operation to be marked Public")
	}

	health, _ := json.Marshal(operations["/health"]["get"])
	if !strings.Contains(string(health), `"security":[]`) {
		t.Fatalf("expected security: [] for a public route, got %s", health)
	}
	plain, _ := json.Marshal(operations["/plain"]["get"])
	if strings.Contains(string(plain), `"security"`) {
		t.Fatalf("expected no security key for a route without options, got %s", plain)
	}
	tasks, _ := json.Marshal(operations["/tasks"]["get"])
	if !strings.Contains(string(tasks), `"security":[{"bearer":[]}]`) {
		t.Fatalf("expected the bearer requirement, got %s", tasks)
	}
}

func TestPublicWinsOverSecurity(t *testing.T) {
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("GET", "/health", Security("bearer"), Public())

	raw, _ := json.Marshal(GenerateOpenAPI(docs).Paths["/health"]["get"])
	if !strings.Contains(string(raw), `"security":[]`) {
		t.Fatalf("expected security: [] when Public is set, got %s", raw)
	}
}

func TestDefaultSecurityIsWrittenAtTheTopLevel(t *testing.T) {
	docs := New(API{Title: "test", Version: "1"})
	docs.AddSecurityScheme("bearer", BearerSecurity("JWT"))
	docs.SetDefaultSecurity("bearer")
	docs.Register("GET", "/tasks")

	spec := generatedJSON(t, docs)
	if !strings.Contains(spec, `"security":[{"bearer":[]}]`) {
		t.Fatalf("expected a top-level default security, got %s", spec)
	}
}

func TestNoDefaultSecurityByDefault(t *testing.T) {
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("GET", "/tasks")

	var spec map[string]any
	_ = json.Unmarshal([]byte(generatedJSON(t, docs)), &spec)
	if _, has := spec["security"]; has {
		t.Fatalf("expected no top-level security, got %v", spec["security"])
	}
}
