package doc

import (
	"reflect"
	"testing"
)

func generatedParameter(t *testing.T, opts ...RouteOption) OpenAPIParameter {
	t.Helper()
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("GET", "/tasks/{status}", opts...)

	operation := GenerateOpenAPI(docs).Paths["/tasks/{status}"]["get"]
	if len(operation.Parameters) != 1 {
		t.Fatalf("expected one parameter, got %d", len(operation.Parameters))
	}
	return operation.Parameters[0]
}

func TestQueryEnumWritesAllowedValues(t *testing.T) {
	param := generatedParameter(t, QueryEnum("in_inbox", "Only true is accepted.", false, "true"))

	if param.In != "query" || param.Name != "in_inbox" || param.Required || param.Description != "Only true is accepted." {
		t.Fatalf("unexpected parameter %+v", param)
	}
	if param.Schema == nil || param.Schema.Type != "string" || !reflect.DeepEqual(param.Schema.Enum, []interface{}{"true"}) {
		t.Fatalf("expected a string schema with enum [true], got %+v", param.Schema)
	}
}

func TestPathEnumWritesAllowedValuesAndIsRequired(t *testing.T) {
	param := generatedParameter(t, PathEnum("status", "Task status.", "open", "done"))

	if param.In != "path" || !param.Required {
		t.Fatalf("expected a required path parameter, got %+v", param)
	}
	if !reflect.DeepEqual(param.Schema.Enum, []interface{}{"open", "done"}) {
		t.Fatalf("expected enum [open done], got %v", param.Schema.Enum)
	}
}
