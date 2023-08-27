package api

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

var bodyExample = `
{
    "info": {
        "deviceUUID": "ADAD3-ADD33-AFSFK...",
        "deviceType": "tablet",
        "deviceBrand": "Samsung",
        "deviceModel": "A11",
        "os": "android",
        "osVersion": "1.0.0",
        "osTimezone": "-6",
        "appLanguage": "es",
        "appVersion": "3.0.0",
        "appBuildVersion": "1.0.0.10",
        "sessionId": "sessionID_123"
    },
    "content": {
        "method": "google",
        "token": "9uQHRyaWJhbHdvcmxkd2lkZS5ndDpNaWt1bTFrdS4K..."
    }
}
`

type response struct {
	Method string `json:"method"`
	Token  string `json:"token"`
}

func TestProcessRequest(t *testing.T) {
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		requestData, err := GetRequestContext(r)
		if err != nil {
			log.Print(err)
			return
		}
		request := response{}
		err = requestData.DecodeContent(&request)
		if err != nil {
			Error{}.Write(w, r)
			log.Print(err)
		}
		log.Print(request.Method)
		Success{}.Write(w, r)
	}
	req := httptest.NewRequest(http.MethodGet, "http://www.your-domain.com/v1/foo/bar", nil)
	req.Body = io.NopCloser(bytes.NewBuffer([]byte(bodyExample)))
	res := httptest.NewRecorder()

	type args struct {
		next http.HandlerFunc
	}
	tests := []struct {
		name string
		args args
		want http.HandlerFunc
	}{
		{
			name: "Example",
			args: args{
				next: testHandler,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handleFunc := ProcessRequest(tt.args.next)
			handleFunc.ServeHTTP(res, req)
		})
	}
}
