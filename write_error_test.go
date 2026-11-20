package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

func TestStatusForKind(t *testing.T) {
	tests := []struct {
		kind codes.Kind
		want int
	}{
		{codes.KindValidation, http.StatusBadRequest},
		{codes.KindUnauthorized, http.StatusUnauthorized},
		{codes.KindForbidden, http.StatusForbidden},
		{codes.KindNotFound, http.StatusNotFound},
		{codes.KindConflict, http.StatusConflict},
		{codes.KindInfrastructure, http.StatusInternalServerError},
		{"", http.StatusInternalServerError},
		{codes.Kind("unknown"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		name := string(tt.kind)
		if name == "" {
			name = "no kind"
		}
		t.Run(name, func(t *testing.T) {
			if got := StatusForKind(tt.kind); got != tt.want {
				t.Fatalf("StatusForKind(%q) = %d, want %d", tt.kind, got, tt.want)
			}
		})
	}
}

func TestWriteErrorUsesTheKindOfTheCode(t *testing.T) {
	useFormatterSettings(t, true)
	notFound := codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound)

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "not found behind fmt.Errorf", err: fmt.Errorf("loading: %w", errors.NewC(notFound, "task 42")), wantStatus: http.StatusNotFound, wantCode: "pln-111"},
		{name: "validation built-in", err: errors.NewC(codes.ValRequired, "title"), wantStatus: http.StatusBadRequest, wantCode: "b001"},
		{name: "unauthorized", err: errors.NewC(codes.AuthFailed, "bad token"), wantStatus: http.StatusUnauthorized, wantCode: "a003"},
		{name: "forbidden", err: errors.NewC(codes.AuthForbidden, "no access"), wantStatus: http.StatusForbidden, wantCode: "f001"},
		{name: "conflict", err: errors.NewC(codes.New("PLN-200", "task is locked").WithKind(codes.KindConflict), "locked"), wantStatus: http.StatusConflict, wantCode: "pln-200"},
		{name: "infrastructure keeps its code", err: errors.NewC(codes.ConnFailed, "db down"), wantStatus: http.StatusInternalServerError, wantCode: "d001"},
		{name: "nil error", err: nil, wantStatus: http.StatusInternalServerError, wantCode: strings.ToLower(string(ResponseCodes.InternalServerEerror))},
		{name: "code without kind", err: errors.NewC(codes.New("X-1", "x"), "x"), wantStatus: http.StatusInternalServerError, wantCode: "x-1"},
		{name: "plain error", err: fmt.Errorf("boom"), wantStatus: http.StatusInternalServerError, wantCode: strings.ToLower(string(ResponseCodes.InternalServerEerror))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			WriteError(recorder, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			var body struct {
				Header struct {
					Code string `json:"code"`
				} `json:"header"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %q: %v", recorder.Body.String(), err)
			}
			if body.Header.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", body.Header.Code, tt.wantCode)
			}
		})
	}
}

func TestErrorResponseForUsesTheUnauthorizedTitle(t *testing.T) {
	for _, code := range []codes.Code{codes.AuthFailed, codes.AuthForbidden} {
		if got := errorResponseFor(errors.NewC(code, "x")).Title; got != UnauthorizedTitle {
			t.Fatalf("%s: title = %q, want %q", code.Str(), got, UnauthorizedTitle)
		}
	}
}

func TestErrorResponseForInfrastructureKeepsCodeMessage(t *testing.T) {
	response := errorResponseFor(errors.NewC(codes.ConnFailed, "db down"))

	if response.Message != codes.ConnFailed.Msg() || response.Title != InternalServerTitle {
		t.Fatalf("got title %q message %q", response.Title, response.Message)
	}
}
