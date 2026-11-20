package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

func queryRequest(rawQuery string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/tasks?"+rawQuery, nil)
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantValue int
		wantOK    bool
		wantErr   bool
	}{
		{name: "present", query: "limit=20", wantValue: 20, wantOK: true},
		{name: "absent", query: "", wantOK: false},
		{name: "empty", query: "limit=", wantOK: true, wantErr: true},
		{name: "invalid", query: "limit=abc", wantOK: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := QueryInt(queryRequest(tt.query), "limit")

			if value != tt.wantValue || ok != tt.wantOK || (err != nil) != tt.wantErr {
				t.Fatalf("QueryInt = %d, %v, %v; want %d, %v, err %v", value, ok, err, tt.wantValue, tt.wantOK, tt.wantErr)
			}
		})
	}
}

func TestQueryBool(t *testing.T) {
	value, ok, err := QueryBool(queryRequest("in_inbox=true"), "in_inbox")
	if !value || !ok || err != nil {
		t.Fatalf("QueryBool = %v, %v, %v", value, ok, err)
	}
	if _, ok, err := QueryBool(queryRequest("in_inbox=maybe"), "in_inbox"); !ok || err == nil {
		t.Fatalf("expected an invalid bool error, got ok=%v err=%v", ok, err)
	}
}

func TestQueryEnum(t *testing.T) {
	value, ok, err := QueryEnum(queryRequest("in_inbox=true"), "in_inbox", []string{"true"})
	if value != "true" || !ok || err != nil {
		t.Fatalf("QueryEnum = %q, %v, %v", value, ok, err)
	}
	if _, ok, err := QueryEnum(queryRequest("in_inbox=false"), "in_inbox", []string{"true"}); !ok || err == nil {
		t.Fatalf("expected a value outside the enum to be invalid, got ok=%v err=%v", ok, err)
	}
	if _, ok, err := QueryEnum(queryRequest(""), "in_inbox", []string{"true"}); ok || err != nil {
		t.Fatalf("expected absent, got ok=%v err=%v", ok, err)
	}
}

func TestQueryString(t *testing.T) {
	tests := []struct {
		query     string
		wantValue string
		wantOK    bool
	}{
		{query: "q=tasks", wantValue: "tasks", wantOK: true},
		{query: "q=", wantValue: "", wantOK: true},
		{query: "", wantValue: "", wantOK: false},
	}
	for _, tt := range tests {
		value, ok := QueryString(queryRequest(tt.query), "q")
		if value != tt.wantValue || ok != tt.wantOK {
			t.Fatalf("QueryString(%q) = %q, %v", tt.query, value, ok)
		}
	}
}

func TestQueryErrorIsAValidationCode(t *testing.T) {
	_, _, err := QueryInt(queryRequest("limit=abc"), "limit")

	if errors.KindOf(err) != codes.KindValidation || StatusForKind(errors.KindOf(err)) != http.StatusBadRequest {
		t.Fatalf("expected a validation-kind error answering 400, got %v", err)
	}
	if errors.CodeOf(err) != codes.ValFormat {
		t.Fatalf("expected the default ValFormat code, got %v", errors.CodeOf(err))
	}
}

func TestQueryErrorCodeCanBeOverriddenPerCall(t *testing.T) {
	limitInvalid := codes.New("PLN-050", "limit must be a number").WithKind(codes.KindValidation)

	_, _, err := QueryInt(queryRequest("limit=abc"), "limit", WithQueryCode(limitInvalid))

	if errors.CodeOf(err) != limitInvalid {
		t.Fatalf("expected the override code, got %v", errors.CodeOf(err))
	}
}
