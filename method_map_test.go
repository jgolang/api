package api

import (
	"net/http"
	"testing"
)

func TestDefaultMethodMaps(t *testing.T) {
	tests := []struct {
		key    string
		method string
	}{
		{key: MethodPostKey, method: http.MethodPost},
		{key: MethodGetKey, method: http.MethodGet},
		{key: MethodPutKey, method: http.MethodPut},
		{key: MethodPatchKey, method: http.MethodPatch},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if !ValidateMethods(tt.key, tt.method) {
				t.Fatalf("ValidateMethods(%q, %q) = false, want true", tt.key, tt.method)
			}
		})
	}
}
