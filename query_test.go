package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

func queryRequest(rawQuery string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	r.URL.RawQuery = rawQuery
	return r
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantValue int
		wantOK    bool
		wantCode  codes.Coder
	}{
		{name: "present", query: "limit=20", wantValue: 20, wantOK: true},
		{name: "signed", query: "limit=-5", wantValue: -5, wantOK: true},
		{name: "absent", query: "", wantOK: false},
		{name: "empty", query: "limit=", wantOK: true, wantCode: codes.ValFormat},
		{name: "not a number", query: "limit=abc", wantOK: true, wantCode: codes.ValFormat},
		{name: "first value wins", query: "limit=abc&limit=5", wantOK: true, wantCode: codes.ValFormat},
		{name: "malformed escape", query: "limit=%zz", wantOK: true, wantCode: codes.ValFormat},
		{name: "semicolon", query: "a=1;limit=5", wantOK: true, wantCode: codes.ValFormat},
		{name: "malformed elsewhere", query: "a=%zz&limit=7", wantValue: 7, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := QueryInt(queryRequest(tt.query), "limit")

			if value != tt.wantValue || ok != tt.wantOK || errors.CodeOf(err) != tt.wantCode {
				t.Fatalf("QueryInt = %d, %v, %v; want %d, %v, code %v", value, ok, err, tt.wantValue, tt.wantOK, tt.wantCode)
			}
		})
	}
}

func TestQueryBool(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantValue bool
		wantOK    bool
		wantErr   bool
	}{
		{name: "true", query: "in_inbox=true", wantValue: true, wantOK: true},
		{name: "absent", query: "", wantOK: false},
		{name: "empty", query: "in_inbox=", wantOK: true, wantErr: true},
		{name: "invalid", query: "in_inbox=maybe", wantOK: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := QueryBool(queryRequest(tt.query), "in_inbox")

			if value != tt.wantValue || ok != tt.wantOK || (err != nil) != tt.wantErr {
				t.Fatalf("QueryBool = %v, %v, %v", value, ok, err)
			}
		})
	}
}

func TestQueryEnum(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantValue string
		wantOK    bool
		wantErr   bool
	}{
		{name: "allowed", query: "in_inbox=true", wantValue: "true", wantOK: true},
		{name: "outside the enum", query: "in_inbox=false", wantOK: true, wantErr: true},
		{name: "absent", query: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := QueryEnum(queryRequest(tt.query), "in_inbox", []string{"true"})

			if value != tt.wantValue || ok != tt.wantOK || (err != nil) != tt.wantErr {
				t.Fatalf("QueryEnum = %q, %v, %v", value, ok, err)
			}
		})
	}
}

func TestQueryEnumErrorNamesTheAllowedValues(t *testing.T) {
	_, _, err := QueryEnum(queryRequest("status=x"), "status", []string{"open", "done"})

	if err == nil || err.Error() != `[b002](Invalid format.): query parameter "status" must be one of open, done` {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestQueryString(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantValue string
		wantOK    bool
	}{
		{name: "present", query: "q=tasks", wantValue: "tasks", wantOK: true},
		{name: "empty", query: "q=", wantValue: "", wantOK: true},
		{name: "absent", query: "", wantValue: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := QueryString(queryRequest(tt.query), "q")

			if value != tt.wantValue || ok != tt.wantOK || err != nil {
				t.Fatalf("QueryString = %q, %v, %v", value, ok, err)
			}
		})
	}
}

func TestQueryErrorAnswers400(t *testing.T) {
	_, _, err := QueryInt(queryRequest("limit=abc"), "limit")

	if StatusForKind(errors.KindOf(err)) != http.StatusBadRequest {
		t.Fatalf("expected a validation kind answering 400, got %v", errors.KindOf(err))
	}
}

func TestWithQueryCodeOverridesTheCode(t *testing.T) {
	limitInvalid := codes.New("PLN-050", "limit must be a number").WithKind(codes.KindValidation)

	tests := []struct {
		name string
		call func(QueryOption) error
	}{
		{name: "QueryInt", call: func(o QueryOption) error { _, _, err := QueryInt(queryRequest("v=x"), "v", o); return err }},
		{name: "QueryBool", call: func(o QueryOption) error { _, _, err := QueryBool(queryRequest("v=x"), "v", o); return err }},
		{name: "QueryEnum", call: func(o QueryOption) error {
			_, _, err := QueryEnum(queryRequest("v=x"), "v", []string{"y"}, o)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.CodeOf(tt.call(WithQueryCode(limitInvalid))); got != limitInvalid {
				t.Fatalf("expected the override code, got %v", got)
			}
		})
	}
}

func TestNilQueryOptionsAreIgnored(t *testing.T) {
	tests := []struct {
		name string
		opt  QueryOption
	}{
		{name: "nil option", opt: nil},
		{name: "nil code", opt: WithQueryCode(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := QueryInt(queryRequest("limit=abc"), "limit", tt.opt)

			if errors.CodeOf(err) != codes.ValFormat {
				t.Fatalf("expected the default code, got %v", err)
			}
		})
	}
}

func BenchmarkQueryInt(b *testing.B) {
	r := queryRequest("limit=20&offset=40&in_inbox=true")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := QueryInt(r, "limit"); err != nil {
			b.Fatal(err)
		}
	}
}
