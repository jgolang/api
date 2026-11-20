package doc

import (
	"reflect"
	"testing"
)

func generatedParameter(t *testing.T, path string, opts ...RouteOption) OpenAPIParameter {
	t.Helper()
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("GET", path, opts...)

	operation := GenerateOpenAPI(docs).Paths[path]["get"]
	if len(operation.Parameters) != 1 {
		t.Fatalf("expected one parameter, got %d", len(operation.Parameters))
	}
	return operation.Parameters[0]
}

func TestQueryEnumWritesTypedValues(t *testing.T) {
	tests := []struct {
		name     string
		typ      DataType
		values   []interface{}
		wantType string
	}{
		{name: "bool", typ: Bool, values: []interface{}{true}, wantType: "boolean"},
		{name: "int", typ: Int, values: []interface{}{10, 20, 50}, wantType: "integer"},
		{name: "string", typ: String, values: []interface{}{"open", "done"}, wantType: "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := generatedParameter(t, "/tasks", QueryEnum("filter", tt.typ, false, "Filter.", tt.values...))

			if param.In != "query" || param.Required || param.Description != "Filter." {
				t.Fatalf("unexpected parameter %+v", param)
			}
			if param.Schema == nil {
				t.Fatal("expected a schema")
			}
			if param.Schema.Type != tt.wantType || !reflect.DeepEqual(param.Schema.Enum, tt.values) {
				t.Fatalf("expected %s enum %v, got %+v", tt.wantType, tt.values, param.Schema)
			}
		})
	}
}

func TestPathEnumIsRequired(t *testing.T) {
	param := generatedParameter(t, "/tasks/{status}", PathEnum("status", String, true, "Task status.", "open", "done"))

	if param.In != "path" || !param.Required || param.Description != "Task status." {
		t.Fatalf("expected a required path parameter, got %+v", param)
	}
	if param.Schema == nil || param.Schema.Type != "string" || !reflect.DeepEqual(param.Schema.Enum, []interface{}{"open", "done"}) {
		t.Fatalf("expected string enum [open done], got %+v", param.Schema)
	}
}

func TestQueryEnumWritesRepeatedValuesOnce(t *testing.T) {
	param := generatedParameter(t, "/tasks", QueryEnum("status", String, false, "", "open", "open", "done"))

	if param.Schema == nil || !reflect.DeepEqual(param.Schema.Enum, []interface{}{"open", "done"}) {
		t.Fatalf("expected enum [open done], got %+v", param.Schema)
	}
}

func TestQueryEnumWithoutValuesHasNoEnum(t *testing.T) {
	param := generatedParameter(t, "/tasks", QueryEnum("page", Int, false, "Page."))

	if param.Schema == nil || param.Schema.Type != "integer" || param.Schema.Enum != nil {
		t.Fatalf("expected a plain integer schema, got %+v", param.Schema)
	}
}

func TestQueryEnumRoutesDoNotShareValues(t *testing.T) {
	option := QueryEnum("status", String, false, "", "open", "done")
	docs := New(API{Title: "test", Version: "1"})
	first := docs.Register("GET", "/a", option)

	first.Parameters[0].Schema.Enum[0] = "changed"
	docs.Register("GET", "/b", option)

	got := GenerateOpenAPI(docs).Paths["/b"]["get"].Parameters[0].Schema.Enum
	if !reflect.DeepEqual(got, []interface{}{"open", "done"}) {
		t.Fatalf("expected the second route to keep [open done], got %v", got)
	}
}
