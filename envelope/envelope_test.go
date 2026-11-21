package envelope

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestResponseJSON(t *testing.T) {
	tests := []struct {
		name     string
		response any
		want     string
	}{
		{
			name:     "empty header keeps only type",
			response: Response{},
			want:     `{"header":{"type":""}}`,
		},
		{
			name: "every header field and content",
			response: Response{
				Header: ResponseInfo{
					Type: "success", Title: "t", Message: "m", Code: "c", Token: "k",
					Action: "a", EventID: "e", Info: map[string]string{"f": "v"},
				},
				Content: map[string]int{"id": 1},
			},
			want: `{"header":{"type":"success","title":"t","message":"m","code":"c","token":"k","action":"a","event_id":"e","info":{"f":"v"}},"content":{"id":1}}`,
		},
		{
			name:     "error response has no content",
			response: Error(),
			want:     `{"header":{"type":""}}`,
		},
		{
			name:     "typed success omits nil content",
			response: Success[struct{ ID int }](),
			want:     `{"header":{"type":""}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.response)

			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestJSONRequestRoundTrip(t *testing.T) {
	const body = `{"header":{"uuid":"u","device_type":"phone","device_brand":"b","device_model":"m","os":"android","os_version":"14","lang":"es","timezone":"tz","app_version":"3","app_build_version":"1","app_name":"n","token":"k","device_id":"d","device_serial":"s","lat":"1","lon":"2"},"content":{"id":1}}`
	var request JSONRequest

	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := json.Marshal(request)

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != body {
		t.Fatalf("got %s, want %s", got, body)
	}
}

func TestRequestInfoOmitsEmptyKeys(t *testing.T) {
	got, err := json.Marshal(JSONRequest{})

	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != `{"header":{}}` {
		t.Fatalf("got %s, want only an empty header", got)
	}
}

func TestTypedRequest(t *testing.T) {
	type task struct {
		Name string `json:"name"`
	}
	request := Request[task]()

	if err := json.Unmarshal([]byte(`{"content":{"name":"x"}}`), &request); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(request.Content, &task{Name: "x"}) {
		t.Fatalf("content = %+v", request.Content)
	}
}
