package api

import (
	"fmt"
	"testing"

	"github.com/jgolang/api/core"
	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

var errorHelpers = []struct {
	name        string
	build       func(error) Error
	wantMessage bool // false when the helper keeps its own message
}{
	{name: "Error400", build: func(err error) Error { return Error400(err) }, wantMessage: true},
	{name: "ErrorWithMsg", build: func(err error) Error { return ErrorWithMsg("custom", err) }, wantMessage: false},
	{name: "Error500", build: func(err error) Error { return Error500(err) }, wantMessage: true},
	{name: "Error501", build: func(err error) Error { return Error501(err) }, wantMessage: true},
}

func TestErrorHelpersKeepCodeOfWrappedErrors(t *testing.T) {
	notFound := codes.New("PLN-111", "task not found")
	coded := errors.NewC(notFound, "task 42 is missing")

	cases := []struct {
		name string
		err  error
	}{
		{name: "unwrapped", err: coded},
		{name: "behind fmt.Errorf", err: fmt.Errorf("loading board: %w", coded)},
		{name: "behind two wrappers", err: fmt.Errorf("handler: %w", fmt.Errorf("loading: %w", coded))},
		{name: "behind a nil-code error", err: errors.WrapC(coded, nil)},
	}

	for _, helper := range errorHelpers {
		for _, tc := range cases {
			t.Run(helper.name+"/"+tc.name, func(t *testing.T) {
				response := helper.build(tc.err)

				if response.ResponseCode != core.ResponseCode("PLN-111") {
					t.Fatalf("expected code PLN-111, got %q", response.ResponseCode)
				}
				if helper.wantMessage && response.Message != "task not found" {
					t.Fatalf("expected message %q, got %q", "task not found", response.Message)
				}
			})
		}
	}
}

func TestErrorHelpersWithoutCodeUseDefaults(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "nil code", err: errors.WrapC(fmt.Errorf("plain failure"), nil)},
		{name: "typed nil", err: fmt.Errorf("x: %w", (*errors.Error)(nil))},
		{name: "plain error", err: fmt.Errorf("plain failure")},
	}

	for _, tc := range cases {
		t.Run("Error400/"+tc.name, func(t *testing.T) {
			response := Error400(tc.err)

			if response.ResponseCode != "" || response.Message != "" {
				t.Fatalf("expected empty code and message for Write to default, got %q %q", response.ResponseCode, response.Message)
			}
		})
		t.Run("Error500/"+tc.name, func(t *testing.T) {
			response := Error500(tc.err)

			if response.ResponseCode != ResponseCodes.InternalServerEerror || response.Message != InternalServerMessage {
				t.Fatalf("expected the Error500 defaults, got %q %q", response.ResponseCode, response.Message)
			}
		})
	}
}
