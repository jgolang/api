package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
)

type createTaskRequest struct {
	Title string `json:"title"`
}

type taskResponse struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type testRequestOf[T any] struct {
	Header  testRequestInfo `json:"header,omitempty"`
	Content *T              `json:"content,omitempty"`
}

type testResponseOf[T any] struct {
	Header  testResponseInfo `json:"header,omitempty"`
	Content *T               `json:"content,omitempty"`
}

type testErrorResponse struct {
	Header testResponseInfo `json:"header,omitempty"`
}

type testRequestInfo struct {
	UUID string `json:"uuid,omitempty" example:"ADAD3-ADD33-AFSFK"`
}

type testResponseInfo struct {
	Type string `json:"type" example:"success"`
}

func TestRegistryRegistersRoutesWithMetadata(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	router := doc.NewRouter(docs)

	api.Post(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {},
		doc.Summary("Create task"),
		doc.Description("Creates a new task"),
		doc.Tags("tasks"),
		doc.Body(createTaskRequest{}),
		doc.Status(http.StatusCreated, taskResponse{}),
		doc.Query("dry_run", doc.Bool, false),
		doc.Security("bearerAuth"),
	)

	routes := docs.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	route := routes[0]
	if route.Method != http.MethodPost || route.Path != "/tasks" {
		t.Fatalf("unexpected route: %#v", route)
	}
	if route.Summary != "Create task" || route.Description != "Creates a new task" {
		t.Fatalf("metadata was not registered: %#v", route)
	}
	if len(route.Tags) != 1 || route.Tags[0] != "tasks" {
		t.Fatalf("tags were not registered: %#v", route.Tags)
	}
	if len(route.Responses) != 1 || route.Responses[0].Status != http.StatusCreated {
		t.Fatalf("responses were not registered: %#v", route.Responses)
	}
}

func TestGenerateOpenAPI(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.AddSecurityScheme("bearerAuth", doc.BearerSecurity("JWT"))
	docs.Register(http.MethodGet, "/tasks/{id}",
		doc.OperationID("getTask"),
		doc.Summary("Get task"),
		doc.Tags("tasks"),
		doc.HeaderWithDescription("X-Request-ID", doc.String, false, "Trace request ID"),
		doc.PathWithDescription("id", doc.Int, false, "Task ID"),
		doc.Security("bearerAuth"),
		doc.Status(http.StatusOK, taskResponse{}),
		doc.Status(http.StatusBadRequest, testErrorResponse{}),
	)

	openapiDoc := doc.GenerateOpenAPI(docs)
	if openapiDoc.OpenAPI != "3.0.3" {
		t.Fatalf("unexpected OpenAPI version: %s", openapiDoc.OpenAPI)
	}
	operation, ok := openapiDoc.Paths["/tasks/{id}"]["get"]
	if !ok {
		t.Fatalf("GET /tasks/{id} operation was not generated: %#v", openapiDoc.Paths)
	}
	if operation.Summary != "Get task" {
		t.Fatalf("unexpected summary: %s", operation.Summary)
	}
	if operation.OperationID != "getTask" {
		t.Fatalf("unexpected operationId: %s", operation.OperationID)
	}
	if len(operation.Parameters) != 2 {
		t.Fatalf("path parameter was not generated: %#v", operation.Parameters)
	}
	if operation.Parameters[0].Name != "X-Request-ID" || operation.Parameters[0].In != "header" || operation.Parameters[0].Description != "Trace request ID" {
		t.Fatalf("header parameter was not generated: %#v", operation.Parameters)
	}
	if operation.Parameters[1].Name != "id" || operation.Parameters[1].In != "path" || !operation.Parameters[1].Required || operation.Parameters[1].Description != "Task ID" {
		t.Fatalf("path parameter was not generated: %#v", operation.Parameters)
	}
	response := operation.Responses["200"]
	responseSchema := resolveOpenAPISchema(openapiDoc, response.Content["application/json"].Schema)
	if responseSchema.Properties["id"].Type != "integer" {
		t.Fatalf("response schema was not generated: %#v", response)
	}
	if response.Content["application/json"].Schema.Ref == "" {
		t.Fatalf("response schema should use a component ref: %#v", response.Content["application/json"].Schema)
	}
	if openapiDoc.Components.SecuritySchemes["bearerAuth"].Scheme != "bearer" {
		t.Fatalf("security scheme was not generated: %#v", openapiDoc.Components)
	}
}

func TestGenerateOpenAPIUsesConfiguredSecurityScheme(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.AddSecurityScheme("apiKeyAuth", doc.APIKeySecurity("X-API-Key", "header"))
	docs.Register(http.MethodGet, "/tasks", doc.Security("apiKeyAuth"), doc.Status(http.StatusOK, taskResponse{}))

	openapiDoc := doc.GenerateOpenAPI(docs)
	scheme := openapiDoc.Components.SecuritySchemes["apiKeyAuth"]
	if scheme.Type != "apiKey" || scheme.Name != "X-API-Key" || scheme.In != "header" {
		t.Fatalf("configured security scheme was not used: %#v", scheme)
	}
}

func TestRegistryRoutesReturnsDefensiveCopy(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks",
		doc.Tags("tasks"),
		doc.StatusWithHeaders(http.StatusOK, taskResponse{}, doc.ResponseHeaderInfo{
			Name:   "X-RateLimit-Remaining",
			Schema: &doc.Schema{Type: "integer"},
		}),
	)

	routes := docs.Routes()
	routes[0].Tags[0] = "mutated"
	routes[0].Responses[0].Headers[0].Schema.Type = "string"

	fresh := docs.Routes()
	if fresh[0].Tags[0] != "tasks" {
		t.Fatalf("tags were mutated through Routes snapshot: %#v", fresh[0].Tags)
	}
	if fresh[0].Responses[0].Headers[0].Schema.Type != "integer" {
		t.Fatalf("response header schema was mutated through Routes snapshot: %#v", fresh[0].Responses[0].Headers[0].Schema)
	}
}

func TestOpenAPIHandler(t *testing.T) {
	docs := doc.New(doc.API{Title: "Tasks API", Version: "1.0.0"})
	docs.Register(http.MethodGet, "/tasks", doc.Status(http.StatusOK, []taskResponse{}))

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	res := httptest.NewRecorder()
	doc.OpenAPIHandler(docs).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	var doc doc.OpenAPI
	if err := json.Unmarshal(res.Body.Bytes(), &doc); err != nil {
		t.Fatalf("handler returned invalid JSON: %v", err)
	}
	if doc.Info.Title != "Tasks API" {
		t.Fatalf("unexpected api metadata: %#v", doc.Info)
	}
}

func TestGenerateOpenAPIMinimalDocumentShape(t *testing.T) {
	type createUserRequest struct {
		Email string `json:"email" format:"email" example:"user@example.com" validate:"required"`
		Name  string `json:"name,omitempty"`
	}
	type userResponse struct {
		ID    int    `json:"id"`
		Email string `json:"email" format:"email"`
	}

	docs := doc.New(doc.API{
		Title:       "Users API",
		Version:     "1.0.0",
		Description: "User management API",
	})
	docs.AddSecurityScheme("apiKeyAuth", doc.APIKeySecurity("X-API-Key", "header"))
	docs.Register(http.MethodPost, "/users",
		doc.Summary("Create user"),
		doc.Description("Creates a new user"),
		doc.Tags("users"),
		doc.QueryWithDescription("notify", doc.Bool, false, "Send notification email"),
		doc.Security("apiKeyAuth"),
		doc.Body(testRequestOf[createUserRequest]{}),
		doc.StatusWithHeaders(http.StatusCreated, testResponseOf[userResponse]{},
			doc.ResponseHeader("Location", doc.String, "Created user URL"),
		),
		doc.Status(http.StatusBadRequest, testErrorResponse{}),
	)

	openapiDoc := doc.GenerateOpenAPI(docs)
	if openapiDoc.OpenAPI != "3.0.3" || openapiDoc.Info.Title != "Users API" || openapiDoc.Info.Description == "" {
		t.Fatalf("unexpected document metadata: %#v", openapiDoc)
	}
	operation := openapiDoc.Paths["/users"]["post"]
	if operation.Summary != "Create user" || len(operation.Tags) != 1 || operation.Tags[0] != "users" {
		t.Fatalf("unexpected operation metadata: %#v", operation)
	}
	if len(operation.Parameters) != 1 || operation.Parameters[0].Name != "notify" || operation.Parameters[0].Schema.Type != "boolean" || operation.Parameters[0].Description != "Send notification email" {
		t.Fatalf("unexpected query parameter: %#v", operation.Parameters)
	}
	requestSchema := operation.RequestBody.Content["application/json"].Schema
	if requestSchema.Ref == "" || !strings.Contains(requestSchema.Ref, "RequestOfCreateUserRequest") {
		t.Fatalf("request schema should use a component ref, got %#v", requestSchema)
	}
	requestSchema = resolveOpenAPISchema(openapiDoc, requestSchema)
	if requestSchema.Properties["header"] == nil {
		t.Fatalf("request wrapper header was not generated: %#v", requestSchema)
	}
	requestContentSchema := resolveOpenAPISchema(openapiDoc, requestSchema.Properties["content"])
	emailRequestSchema := requestContentSchema.Properties["email"]
	if emailRequestSchema.Format != "email" || emailRequestSchema.Example != "user@example.com" {
		t.Fatalf("request schema tags were not generated: %#v", emailRequestSchema)
	}
	if !containsString(requestContentSchema.Required, "email") {
		t.Fatalf("required field was not generated: %#v", requestContentSchema.Required)
	}
	created := operation.Responses["201"]
	location := created.Headers["Location"]
	if location.Description != "Created user URL" || location.Schema.Type != "string" {
		t.Fatalf("response header was not generated: %#v", created.Headers)
	}
	createdSchema := created.Content["application/json"].Schema
	if createdSchema.Ref == "" || !strings.Contains(createdSchema.Ref, "ResponseOfUserResponse") {
		t.Fatalf("response schema should use a component ref, got %#v", createdSchema)
	}
	createdSchema = resolveOpenAPISchema(openapiDoc, createdSchema)
	if created.Description != "Created" || createdSchema.Properties["header"] == nil {
		t.Fatalf("unexpected response schema: %#v", created)
	}
	responseContentSchema := resolveOpenAPISchema(openapiDoc, createdSchema.Properties["content"])
	if responseContentSchema.Properties["id"].Type != "integer" {
		t.Fatalf("typed response content schema was not generated: %#v", responseContentSchema)
	}
	errorSchema := resolveOpenAPISchema(openapiDoc, operation.Responses["400"].Content["application/json"].Schema)
	if errorSchema.Properties["content"] != nil {
		t.Fatalf("error response schema should not include content: %#v", operation.Responses["400"])
	}
	if operation.Security[0]["apiKeyAuth"] == nil {
		t.Fatalf("operation security was not generated: %#v", operation.Security)
	}
	scheme := openapiDoc.Components.SecuritySchemes["apiKeyAuth"]
	if scheme.Type != "apiKey" || scheme.Name != "X-API-Key" || scheme.In != "header" {
		t.Fatalf("security scheme was not generated: %#v", scheme)
	}
}

func resolveOpenAPISchema(doc doc.OpenAPI, schema *doc.Schema) *doc.Schema {
	if schema == nil || schema.Ref == "" {
		return schema
	}
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(schema.Ref, prefix) {
		return schema
	}
	resolved := doc.Components.Schemas[strings.TrimPrefix(schema.Ref, prefix)]
	if resolved == nil {
		return schema
	}
	return resolved
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
