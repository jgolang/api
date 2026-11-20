package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jgolang/api/core"
	"github.com/jgolang/api/envelope"
)

// useFormatterSettings sets the package-level formatter settings for one test
// and restores them afterwards.
func useFormatterSettings(t *testing.T, blankSuccess bool) {
	t.Helper()
	oldMode, oldBlank, oldTrace := CurrentResponseMode, BlankSuccess, TraceVisibility
	CurrentResponseMode, BlankSuccess, TraceVisibility = ResponseModeEnvelope, blankSuccess, 1
	t.Cleanup(func() {
		CurrentResponseMode, BlankSuccess, TraceVisibility = oldMode, oldBlank, oldTrace
	})
}

func formattedHeader(t *testing.T, kind core.ResponseType) envelope.ResponseInfo {
	t.Helper()
	body, ok := ResponseFormatter{}.Format(core.ResponseData{
		Title:        "Heads up",
		Message:      "task not found",
		ResponseType: kind,
		ResponseCode: "PLN-111",
	}).Body.(envelope.Response)
	if !ok {
		t.Fatal("expected an envelope body")
	}
	return body.Header
}

func TestBlankSuccessDefaultsToTrue(t *testing.T) {
	if !BlankSuccess {
		t.Fatal("expected BlankSuccess to default to true")
	}
}

func TestBlankSuccessBlanksOnlySuccessResponses(t *testing.T) {
	tests := []struct {
		name         string
		blankSuccess bool
		kind         core.ResponseType
		wantBlank    bool
	}{
		{name: "success blanked", blankSuccess: true, kind: SuccessType, wantBlank: true},
		{name: "success kept when off", blankSuccess: false, kind: SuccessType, wantBlank: false},
		{name: "error kept", blankSuccess: true, kind: ErrorType, wantBlank: false},
		{name: "warning kept", blankSuccess: true, kind: WarningType, wantBlank: false},
		{name: "informative kept", blankSuccess: true, kind: InformativeType, wantBlank: false},
		{name: "custom type kept", blankSuccess: true, kind: core.ResponseType("custom"), wantBlank: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFormatterSettings(t, tt.blankSuccess)

			header := formattedHeader(t, tt.kind)

			if tt.wantBlank {
				if header.Message != "" || header.Title != "" {
					t.Fatalf("expected blank title and message, got %q %q", header.Title, header.Message)
				}
				return
			}
			if header.Title != "Heads up" || !strings.HasPrefix(header.Message, "task not found (") {
				t.Fatalf("expected title and message with trace suffix, got %q %q", header.Title, header.Message)
			}
		})
	}
}

func TestTraceSuffixIsSkippedForEmptyMessages(t *testing.T) {
	useFormatterSettings(t, true)

	for _, visibility := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("TraceVisibility %d", visibility), func(t *testing.T) {
			TraceVisibility = visibility

			body, ok := ResponseFormatter{}.Format(core.ResponseData{
				ResponseType: ErrorType,
				ResponseCode: "PLN-111",
				EventID:      "abc123",
			}).Body.(envelope.Response)
			if !ok {
				t.Fatal("expected an envelope body")
			}
			if body.Header.Message != "" {
				t.Fatalf("expected an empty message, got %q", body.Header.Message)
			}
		})
	}
}
