package doc

import (
	"encoding/json"
	"testing"
)

func TestGenerateOpenAPIWritesServersContactAndLicense(t *testing.T) {
	docs := New(API{
		Title:   "Tasks",
		Version: "1.0.0",
		Contact: &Contact{Name: "Tasks team", URL: "https://example.com/support", Email: "tasks@example.com"},
		License: &License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
	})
	docs.SetServers(
		Server{URL: "https://api.example.com", Description: "Production"},
		Server{URL: "http://localhost:8080"},
	)

	encoded, err := json.Marshal(GenerateOpenAPI(docs))

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"openapi":"3.0.3","info":{"title":"Tasks","version":"1.0.0",` +
		`"contact":{"name":"Tasks team","url":"https://example.com/support","email":"tasks@example.com"},` +
		`"license":{"name":"MIT","url":"https://opensource.org/licenses/MIT"}},` +
		`"servers":[{"url":"https://api.example.com","description":"Production"},{"url":"http://localhost:8080"}],` +
		`"paths":{}}`
	if string(encoded) != want {
		t.Fatalf("got  %s\nwant %s", encoded, want)
	}
}

func TestGenerateOpenAPIWithoutServersContactOrLicenseIsUnchanged(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0", Description: "Task API"})

	encoded, err := json.Marshal(GenerateOpenAPI(docs))

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"openapi":"3.0.3","info":{"title":"Tasks","version":"1.0.0","description":"Task API"},"paths":{}}`
	if string(encoded) != want {
		t.Fatalf("got  %s\nwant %s", encoded, want)
	}
}

func TestSetServersReplacesPreviousServers(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.SetServers(Server{URL: "https://old.example.com"})

	docs.SetServers(Server{URL: "https://api.example.com"})

	servers := GenerateOpenAPI(docs).Servers
	if len(servers) != 1 || servers[0].URL != "https://api.example.com" {
		t.Fatalf("expected only the last servers, got %#v", servers)
	}
}

func TestSetServersWithNoneClearsServers(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	docs.SetServers(Server{URL: "https://api.example.com"})

	docs.SetServers()

	if servers := GenerateOpenAPI(docs).Servers; servers != nil {
		t.Fatalf("expected no servers, got %#v", servers)
	}
}

func TestSetServersCopiesItsArguments(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})
	servers := []Server{{URL: "https://api.example.com"}}
	docs.SetServers(servers...)

	servers[0].URL = "changed"

	if got := GenerateOpenAPI(docs).Servers[0].URL; got != "https://api.example.com" {
		t.Fatalf("expected stored server unchanged, got %q", got)
	}
}

func TestSetServersIgnoresEmptyURL(t *testing.T) {
	docs := New(API{Title: "Tasks", Version: "1.0.0"})

	docs.SetServers(Server{Description: "no URL"}, Server{URL: "https://api.example.com"})

	servers := GenerateOpenAPI(docs).Servers
	if len(servers) != 1 || servers[0].URL != "https://api.example.com" {
		t.Fatalf("expected the empty-URL server dropped, got %#v", servers)
	}
}

func TestAPIStaysComparable(t *testing.T) {
	// doc.API must stay comparable: callers may use == or map keys.
	_ = map[API]bool{{Title: "Tasks"}: true}
}

func TestDocsAPIReturnsACopy(t *testing.T) {
	docs := New(API{
		Title:   "Tasks",
		Contact: &Contact{Name: "Tasks team"},
		License: &License{Name: "MIT"},
	})
	info := docs.API()

	info.Contact.Name = "changed"
	info.License.Name = "changed"

	stored := docs.API()
	if stored.Contact.Name != "Tasks team" || stored.License.Name != "MIT" {
		t.Fatalf("expected stored metadata unchanged, got %#v", stored)
	}
}
