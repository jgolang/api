package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type createTaskRequest struct {
	Title string `json:"title"`
}

type taskResponse struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func TestRegistryRegistersRoutesWithMetadata(t *testing.T) {
	registry := NewRegistry(Info{Title: "Tasks API", Version: "1.0.0"})
	router := NewMetadataRouter(registry)

	Post(router, "/tasks", func(w http.ResponseWriter, r *http.Request) {},
		Summary("Create task"),
		Description("Creates a new task"),
		Tags("tasks"),
		Body(createTaskRequest{}),
		ResponseStatus(http.StatusCreated, taskResponse{}),
		Query("dry_run", Bool, false),
		Security("bearerAuth"),
	)

	routes := registry.Routes()
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
	registry := NewRegistry(Info{Title: "Tasks API", Version: "1.0.0"})
	registry.AddSecurityScheme("bearerAuth", BearerSecurity("JWT"))
	registry.Register(http.MethodGet, "/tasks/{id}",
		OperationID("getTask"),
		Summary("Get task"),
		Tags("tasks"),
		Header("X-Request-ID", String, false),
		Path("id", Int, true),
		Security("bearerAuth"),
		ResponseStatus(http.StatusOK, taskResponse{}),
		ResponseStatus(http.StatusBadRequest, ErrorResponse{}),
	)

	doc := GenerateOpenAPI(registry)
	if doc.OpenAPI != "3.0.3" {
		t.Fatalf("unexpected OpenAPI version: %s", doc.OpenAPI)
	}
	operation, ok := doc.Paths["/tasks/{id}"]["get"]
	if !ok {
		t.Fatalf("GET /tasks/{id} operation was not generated: %#v", doc.Paths)
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
	if operation.Parameters[0].Name != "X-Request-ID" || operation.Parameters[0].In != "header" {
		t.Fatalf("header parameter was not generated: %#v", operation.Parameters)
	}
	if operation.Parameters[1].Name != "id" || operation.Parameters[1].In != "path" {
		t.Fatalf("path parameter was not generated: %#v", operation.Parameters)
	}
	response := operation.Responses["200"]
	responseSchema := resolveOpenAPISchema(doc, response.Content["application/json"].Schema)
	if responseSchema.Properties["id"].Type != "integer" {
		t.Fatalf("response schema was not generated: %#v", response)
	}
	if response.Content["application/json"].Schema.Ref == "" {
		t.Fatalf("response schema should use a component ref: %#v", response.Content["application/json"].Schema)
	}
	if doc.Components.SecuritySchemes["bearerAuth"].Scheme != "bearer" {
		t.Fatalf("security scheme was not generated: %#v", doc.Components)
	}
}

func TestGenerateOpenAPIUsesConfiguredSecurityScheme(t *testing.T) {
	registry := NewRegistry(Info{Title: "Tasks API", Version: "1.0.0"})
	registry.AddSecurityScheme("apiKeyAuth", APIKeySecurity("X-API-Key", "header"))
	registry.Register(http.MethodGet, "/tasks", Security("apiKeyAuth"), ResponseStatus(http.StatusOK, taskResponse{}))

	doc := GenerateOpenAPI(registry)
	scheme := doc.Components.SecuritySchemes["apiKeyAuth"]
	if scheme.Type != "apiKey" || scheme.Name != "X-API-Key" || scheme.In != "header" {
		t.Fatalf("configured security scheme was not used: %#v", scheme)
	}
}

func TestRegistryRoutesReturnsDefensiveCopy(t *testing.T) {
	registry := NewRegistry(Info{Title: "Tasks API", Version: "1.0.0"})
	registry.Register(http.MethodGet, "/tasks",
		Tags("tasks"),
		ResponseSchema(http.StatusOK, "OK", &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"id": {Type: "integer"},
			},
		}),
	)

	routes := registry.Routes()
	routes[0].Tags[0] = "mutated"
	routes[0].Responses[0].Schema.Properties["id"].Type = "string"

	fresh := registry.Routes()
	if fresh[0].Tags[0] != "tasks" {
		t.Fatalf("tags were mutated through Routes snapshot: %#v", fresh[0].Tags)
	}
	if fresh[0].Responses[0].Schema.Properties["id"].Type != "integer" {
		t.Fatalf("schema was mutated through Routes snapshot: %#v", fresh[0].Responses[0].Schema)
	}
}

func TestOpenAPIHandler(t *testing.T) {
	registry := NewRegistry(Info{Title: "Tasks API", Version: "1.0.0"})
	registry.Register(http.MethodGet, "/tasks", ResponseStatus(http.StatusOK, []taskResponse{}))

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	res := httptest.NewRecorder()
	OpenAPIHandler(registry).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	var doc OpenAPI
	if err := json.Unmarshal(res.Body.Bytes(), &doc); err != nil {
		t.Fatalf("handler returned invalid JSON: %v", err)
	}
	if doc.Info.Title != "Tasks API" {
		t.Fatalf("unexpected info: %#v", doc.Info)
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

	registry := NewRegistry(Info{
		Title:       "Users API",
		Version:     "1.0.0",
		Description: "User management API",
	})
	registry.AddSecurityScheme("apiKeyAuth", APIKeySecurity("X-API-Key", "header"))
	registry.Register(http.MethodPost, "/users",
		Summary("Create user"),
		Description("Creates a new user"),
		Tags("users"),
		Query("notify", Bool, false),
		Security("apiKeyAuth"),
		Body(JSONRequestOf[createUserRequest]{}),
		ResponseStatus(http.StatusCreated, JSONResponseOf[userResponse]{}),
		ResponseStatus(http.StatusBadRequest, JSONErrorResponse{}),
	)

	doc := GenerateOpenAPI(registry)
	if doc.OpenAPI != "3.0.3" || doc.Info.Title != "Users API" || doc.Info.Description == "" {
		t.Fatalf("unexpected document metadata: %#v", doc)
	}
	operation := doc.Paths["/users"]["post"]
	if operation.Summary != "Create user" || len(operation.Tags) != 1 || operation.Tags[0] != "users" {
		t.Fatalf("unexpected operation metadata: %#v", operation)
	}
	if len(operation.Parameters) != 1 || operation.Parameters[0].Name != "notify" || operation.Parameters[0].Schema.Type != "boolean" {
		t.Fatalf("unexpected query parameter: %#v", operation.Parameters)
	}
	requestSchema := operation.RequestBody.Content["application/json"].Schema
	if requestSchema.Ref == "" || !strings.Contains(requestSchema.Ref, "JSONRequestOfCreateUserRequest") {
		t.Fatalf("request schema should use a component ref, got %#v", requestSchema)
	}
	requestSchema = resolveOpenAPISchema(doc, requestSchema)
	if requestSchema.Properties["header"] == nil {
		t.Fatalf("request wrapper header was not generated: %#v", requestSchema)
	}
	requestContentSchema := resolveOpenAPISchema(doc, requestSchema.Properties["content"])
	emailRequestSchema := requestContentSchema.Properties["email"]
	if emailRequestSchema.Format != "email" || emailRequestSchema.Example != "user@example.com" {
		t.Fatalf("request schema tags were not generated: %#v", emailRequestSchema)
	}
	if !containsString(requestContentSchema.Required, "email") {
		t.Fatalf("required field was not generated: %#v", requestContentSchema.Required)
	}
	created := operation.Responses["201"]
	createdSchema := created.Content["application/json"].Schema
	if createdSchema.Ref == "" || !strings.Contains(createdSchema.Ref, "JSONResponseOfUserResponse") {
		t.Fatalf("response schema should use a component ref, got %#v", createdSchema)
	}
	createdSchema = resolveOpenAPISchema(doc, createdSchema)
	if created.Description != "Created" || createdSchema.Properties["header"] == nil {
		t.Fatalf("unexpected response schema: %#v", created)
	}
	responseContentSchema := resolveOpenAPISchema(doc, createdSchema.Properties["content"])
	if responseContentSchema.Properties["id"].Type != "integer" {
		t.Fatalf("typed response content schema was not generated: %#v", responseContentSchema)
	}
	errorSchema := resolveOpenAPISchema(doc, operation.Responses["400"].Content["application/json"].Schema)
	if errorSchema.Properties["content"] != nil {
		t.Fatalf("error response schema should not include content: %#v", operation.Responses["400"])
	}
	if operation.Security[0]["apiKeyAuth"] == nil {
		t.Fatalf("operation security was not generated: %#v", operation.Security)
	}
	scheme := doc.Components.SecuritySchemes["apiKeyAuth"]
	if scheme.Type != "apiKey" || scheme.Name != "X-API-Key" || scheme.In != "header" {
		t.Fatalf("security scheme was not generated: %#v", scheme)
	}
}

func resolveOpenAPISchema(doc OpenAPI, schema *Schema) *Schema {
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
