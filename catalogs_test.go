package api

import (
	"net/http"
	"testing"

	"github.com/jgolang/api/core"
)

// useInternalServerErrorCodes sets both internal server error code fields for
// one test and restores them afterwards.
func useInternalServerErrorCodes(t *testing.T, deprecated, current string) {
	t.Helper()
	oldDeprecated, oldCurrent := ResponseCodes.InternalServerEerror, ResponseCodes.InternalServerError
	t.Cleanup(func() {
		ResponseCodes.InternalServerEerror, ResponseCodes.InternalServerError = oldDeprecated, oldCurrent
	})
	if deprecated != "" {
		ResponseCodes.InternalServerEerror = core.ResponseCode(deprecated)
	}
	if current != "" {
		ResponseCodes.InternalServerError = core.ResponseCode(current)
	}
}

func TestInternalServerErrorCode(t *testing.T) {
	writers := map[string]http.HandlerFunc{
		"Error500":        func(w http.ResponseWriter, r *http.Request) { Error500().Write(w, r) },
		"WriteError(nil)": func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, nil) },
	}
	tests := []struct {
		name         string
		deprecated   string
		current      string
		wantDefault  string
		wantInstance string
	}{
		{name: "default code", wantDefault: "e300", wantInstance: "E300"},
		{name: "only the deprecated field set", deprecated: "X1", wantDefault: "x1", wantInstance: "X1"},
		{name: "only the new field set", current: "X2", wantDefault: "x2", wantInstance: "X2"},
		{name: "both set, new wins", deprecated: "X1", current: "X2", wantDefault: "x2", wantInstance: "X2"},
	}

	for _, tt := range tests {
		for writerName, writer := range writers {
			t.Run(tt.name+"/"+writerName, func(t *testing.T) {
				useInternalServerErrorCodes(t, tt.deprecated, tt.current)

				defaultCode, _ := codeAndMessage(serve(writer, http.MethodGet, "", nil).Body.Bytes())
				instanceCode, _ := codeAndMessage(serve(New().Handler(writer), http.MethodGet, "", nil).Body.Bytes())

				if defaultCode != tt.wantDefault {
					t.Fatalf("default instance code = %q, want %q", defaultCode, tt.wantDefault)
				}
				if instanceCode != tt.wantInstance {
					t.Fatalf("instance code = %q, want %q", instanceCode, tt.wantInstance)
				}
			})
		}
	}
}
