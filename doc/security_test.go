package doc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func newTestDocs() *Docs {
	return New(API{Title: "test", Version: "1"})
}

func marshalString(t *testing.T, value interface{}) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func decodeMap(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return decoded
}

func TestPublicOperationWritesEmptySecurity(t *testing.T) {
	docs := newTestDocs()
	docs.Register("GET", "/health", Public())

	operation := GenerateOpenAPI(docs).Paths["/health"]["get"]
	security, has := decodeMap(t, marshalString(t, operation))["security"]

	if !operation.Public || !has || !reflect.DeepEqual(security, []interface{}{}) {
		t.Fatalf("expected Public with security [], got %v (present %v)", security, has)
	}
}

func TestOperationWithoutOptionsHasNoSecurity(t *testing.T) {
	docs := newTestDocs()
	docs.Register("GET", "/plain")

	if _, has := decodeMap(t, marshalString(t, GenerateOpenAPI(docs).Paths["/plain"]["get"]))["security"]; has {
		t.Fatal("expected no security key")
	}
}

func TestPublicIgnoresSecurityOptionsAndAddsNoScheme(t *testing.T) {
	docs := newTestDocs()
	docs.Register("GET", "/health", Security("bearer"), Public())

	spec := GenerateOpenAPI(docs)

	if security := decodeMap(t, marshalString(t, spec.Paths["/health"]["get"]))["security"]; !reflect.DeepEqual(security, []interface{}{}) {
		t.Fatalf("expected security [], got %v", security)
	}
	if spec.Components != nil && len(spec.Components.SecuritySchemes) != 0 {
		t.Fatalf("expected no invented scheme, got %v", spec.Components.SecuritySchemes)
	}
}

func TestDefaultSecurityWithAPublicRoute(t *testing.T) {
	docs := newTestDocs()
	docs.AddSecurityScheme("bearer", BearerSecurity("JWT"))
	docs.SetDefaultSecurity("bearer")
	docs.Register("GET", "/health", Public())
	docs.Register("GET", "/tasks")

	spec := decodeMap(t, marshalString(t, GenerateOpenAPI(docs)))
	paths := spec["paths"].(map[string]interface{})
	health := paths["/health"].(map[string]interface{})["get"].(map[string]interface{})
	tasks := paths["/tasks"].(map[string]interface{})["get"].(map[string]interface{})

	if !reflect.DeepEqual(spec["security"], []interface{}{map[string]interface{}{"bearer": []interface{}{}}}) {
		t.Fatalf("expected top-level bearer, got %v", spec["security"])
	}
	if !reflect.DeepEqual(health["security"], []interface{}{}) {
		t.Fatalf("expected the public route to opt out, got %v", health["security"])
	}
	if _, has := tasks["security"]; has {
		t.Fatalf("expected the other route to inherit the default, got %v", tasks["security"])
	}
}

func TestSetDefaultSecurityIgnoresEmptyAndRepeatedNames(t *testing.T) {
	docs := newTestDocs()
	docs.SetDefaultSecurity("", "bearer", "bearer", "apiKey")

	if got := docs.DefaultSecurity(); !reflect.DeepEqual(got, []string{"bearer", "apiKey"}) {
		t.Fatalf("expected [bearer apiKey], got %v", got)
	}
}

func TestDefaultSecurityReturnsACopy(t *testing.T) {
	docs := newTestDocs()
	docs.SetDefaultSecurity("bearer")

	docs.DefaultSecurity()[0] = "changed"

	if got := docs.DefaultSecurity(); got[0] != "bearer" {
		t.Fatalf("expected the stored value to stay bearer, got %v", got)
	}
}

func TestDefaultSecurityOnNilDocs(t *testing.T) {
	var docs *Docs
	docs.SetDefaultSecurity("bearer")

	if docs.DefaultSecurity() != nil {
		t.Fatal("expected nil for nil docs")
	}
}

func TestOperationJSONRoundTripKeepsPublic(t *testing.T) {
	original := Operation{Summary: "health", Responses: map[string]ResponseObject{}, Public: true}

	var decoded Operation
	if err := json.Unmarshal([]byte(marshalString(t, original)), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !decoded.Public || marshalString(t, decoded) != marshalString(t, original) {
		t.Fatalf("expected the round trip to keep security [], got %+v", decoded)
	}
}

func TestOperationMarshalLeavesEscapingToTheEncoder(t *testing.T) {
	operation := Operation{Summary: "a < b", Responses: map[string]ResponseObject{}}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(&operation); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if !bytes.Contains(buffer.Bytes(), []byte(`"summary":"a < b"`)) {
		t.Fatalf("expected unescaped summary, got %s", buffer.String())
	}
}

func TestRouteSecurityOverridesTheDefault(t *testing.T) {
	docs := newTestDocs()
	docs.SetDefaultSecurity("bearer")
	docs.Register("GET", "/admin", Security("apiKey"))

	security := decodeMap(t, marshalString(t, GenerateOpenAPI(docs).Paths["/admin"]["get"]))["security"]

	if !reflect.DeepEqual(security, []interface{}{map[string]interface{}{"apiKey": []interface{}{}}}) {
		t.Fatalf("expected the route's own apiKey requirement, got %v", security)
	}
}

func TestOperationMarshalStillEscapesWithPlainMarshal(t *testing.T) {
	raw := marshalString(t, Operation{Summary: "a < b", Responses: map[string]ResponseObject{}})

	if !strings.Contains(raw, `a \u003c b`) {
		t.Fatalf("expected json.Marshal to escape <, got %s", raw)
	}
}
