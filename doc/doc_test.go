package doc

import (
	"net/http"
	"testing"
)

type createUserRequest struct {
	Email string `json:"email" validate:"required"`
}

type userResponse struct {
	ID int `json:"id"`
}

func TestDocHelpersGenerateOpenAPIMetadata(t *testing.T) {
	docs := New(API{Title: "Users API", Version: "1.0.0"})
	docs.Register(http.MethodPost, "/users",
		OperationID("createUser"),
		Summary("Create user"),
		Tags("users"),
		HeaderWithDescription("X-Request-ID", String, false, "Trace request ID"),
		Body(testRequestOf[createUserRequest]{}),
		StatusWithHeaders(http.StatusCreated, testResponseOf[userResponse]{},
			ResponseHeader("Location", String, "Created user URL"),
		),
		Status(http.StatusBadRequest, testErrorResponse{}),
	)

	openapiDoc := GenerateOpenAPI(docs)
	operation := openapiDoc.Paths["/users"]["post"]
	if operation.OperationID != "createUser" || operation.Summary != "Create user" {
		t.Fatalf("operation metadata was not generated: %#v", operation)
	}
	if operation.Parameters[0].Name != "X-Request-ID" || operation.Parameters[0].Description != "Trace request ID" {
		t.Fatalf("header metadata was not generated: %#v", operation.Parameters)
	}
	if operation.RequestBody.Content["application/json"].Schema.Ref == "" {
		t.Fatalf("request body should use a schema ref: %#v", operation.RequestBody)
	}
	created := operation.Responses["201"]
	if created.Headers["Location"].Description != "Created user URL" {
		t.Fatalf("response header was not generated: %#v", created.Headers)
	}
	if created.Content["application/json"].Schema.Ref == "" {
		t.Fatalf("response body should use a schema ref: %#v", created.Content)
	}
	if operation.Responses["400"].Content["application/json"].Schema.Ref == "" {
		t.Fatalf("error response should use a schema ref: %#v", operation.Responses["400"])
	}
}
