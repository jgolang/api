package doc

import (
	"reflect"
	"testing"
)

type mediaTypeBody struct {
	Name string `json:"name"`
}

func binarySchema() *Schema {
	return &Schema{Type: "string", Format: "binary"}
}

func uploadSchema() *Schema {
	return &Schema{
		Type:       "object",
		Properties: map[string]*Schema{"file": binarySchema()},
		Required:   []string{"file"},
	}
}

func withBodyContentType(contentType string) RouteOption {
	return func(route *Route) {
		route.BodyContentType = contentType
	}
}

func generateMediaTypeOperation(t *testing.T, opts ...RouteOption) Operation {
	t.Helper()
	docs := New(API{Title: "test", Version: "1"})
	docs.Register("POST", "/files", opts...)
	operation, ok := GenerateOpenAPI(docs).Paths["/files"]["post"]
	if !ok {
		t.Fatal("expected a POST /files operation")
	}
	return operation
}

func TestRequestBodyMediaType(t *testing.T) {
	tests := []struct {
		name        string
		opts        []RouteOption
		wantKey     string
		wantSchema  *Schema
		wantExample interface{}
	}{
		{
			name:    "empty content type defaults to JSON",
			opts:    []RouteOption{Body(mediaTypeBody{})},
			wantKey: "application/json",
		},
		{
			name:       "multipart upload",
			opts:       []RouteOption{BodySchema(uploadSchema()), withBodyContentType("multipart/form-data")},
			wantKey:    "multipart/form-data",
			wantSchema: uploadSchema(),
		},
		{
			name:        "example kept under custom media type",
			opts:        []RouteOption{BodyWithExample(mediaTypeBody{}, mediaTypeBody{Name: "a"}), withBodyContentType("application/x-www-form-urlencoded")},
			wantKey:     "application/x-www-form-urlencoded",
			wantExample: mediaTypeBody{Name: "a"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operation := generateMediaTypeOperation(t, test.opts...)

			if operation.RequestBody == nil {
				t.Fatal("expected a request body")
			}
			content := operation.RequestBody.Content
			media, ok := content[test.wantKey]
			if !ok || len(content) != 1 {
				t.Fatalf("expected only %q content, got %+v", test.wantKey, content)
			}
			if test.wantSchema != nil && !reflect.DeepEqual(media.Schema, test.wantSchema) {
				t.Fatalf("expected schema %+v, got %+v", test.wantSchema, media.Schema)
			}
			if !reflect.DeepEqual(media.Example, test.wantExample) {
				t.Fatalf("expected example %+v, got %+v", test.wantExample, media.Example)
			}
		})
	}
}

func TestResponseMediaType(t *testing.T) {
	tests := []struct {
		name       string
		response   RouteResponse
		wantKey    string
		wantSchema *Schema
	}{
		{
			name:     "empty content type defaults to JSON",
			response: RouteResponse{Status: 200, Body: mediaTypeBody{}},
			wantKey:  "application/json",
		},
		{
			name:       "binary download",
			response:   RouteResponse{Status: 200, ContentType: "application/octet-stream", Schema: binarySchema()},
			wantKey:    "application/octet-stream",
			wantSchema: binarySchema(),
		},
		{
			name:       "any media type",
			response:   RouteResponse{Status: 200, ContentType: "*/*", Schema: binarySchema()},
			wantKey:    "*/*",
			wantSchema: binarySchema(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operation := generateMediaTypeOperation(t, withResponse(test.response))

			content := operation.Responses["200"].Content
			media, ok := content[test.wantKey]
			if !ok || len(content) != 1 {
				t.Fatalf("expected only %q content, got %+v", test.wantKey, content)
			}
			if test.wantSchema != nil && !reflect.DeepEqual(media.Schema, test.wantSchema) {
				t.Fatalf("expected schema %+v, got %+v", test.wantSchema, media.Schema)
			}
		})
	}
}

func TestResponseContentTypeWithoutBodyWritesNoContent(t *testing.T) {
	operation := generateMediaTypeOperation(t, withResponse(RouteResponse{Status: 204, ContentType: "application/octet-stream"}))

	if content := operation.Responses["204"].Content; content != nil {
		t.Fatalf("expected no content, got %+v", content)
	}
}

func TestResponseContentTypeKeepsExamples(t *testing.T) {
	tests := []struct {
		name         string
		response     RouteResponse
		wantExample  interface{}
		wantExamples map[string]ExampleObject
	}{
		{
			name:        "single example",
			response:    RouteResponse{Status: 200, ContentType: "application/problem+json", Body: mediaTypeBody{}, Example: mediaTypeBody{Name: "a"}},
			wantExample: mediaTypeBody{Name: "a"},
		},
		{
			name: "named examples",
			response: RouteResponse{Status: 200, ContentType: "application/problem+json", Body: mediaTypeBody{},
				Examples: map[string]interface{}{"first": mediaTypeBody{Name: "b"}}},
			wantExamples: map[string]ExampleObject{"first": {Value: mediaTypeBody{Name: "b"}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operation := generateMediaTypeOperation(t, withResponse(test.response))

			media, ok := operation.Responses["200"].Content["application/problem+json"]
			if !ok {
				t.Fatalf("expected application/problem+json content, got %+v", operation.Responses["200"].Content)
			}
			if !reflect.DeepEqual(media.Example, test.wantExample) || !reflect.DeepEqual(media.Examples, test.wantExamples) {
				t.Fatalf("expected example %+v and examples %+v, got %+v", test.wantExample, test.wantExamples, media)
			}
		})
	}
}

func withResponse(response RouteResponse) RouteOption {
	return func(route *Route) {
		route.Responses = append(route.Responses, response)
	}
}
