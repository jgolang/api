package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/stdadapter"
)

// constraintTagsCapture carries every constraint tag doc reads, so the
// contracttest module validates the generated keywords and checks the
// request example against them.
type constraintTagsCapture struct {
	TargetType string   `json:"target_type" enum:"goal,project,task,note" description:"What the capture becomes."`
	Priority   int      `json:"priority" enum:"1,2,3" default:"2"`
	Title      string   `json:"title" minLength:"1" maxLength:"80"`
	Slug       string   `json:"slug" pattern:"^[a-z-]+$"`
	Estimate   float64  `json:"estimate" minimum:"0" maximum:"99.5"`
	Status     string   `json:"status,omitempty" default:"open"`
	Archived   bool     `json:"archived,omitempty" default:"false"`
	LegacyID   string   `json:"legacy_id,omitempty" deprecated:"true"`
	Tags       []string `json:"tags,omitempty" default:"[]"`
	// Level is a nullable enum: the request example leaves it nil, so it is
	// sent as null, which the contracttest module validates against an enum
	// that lists null.
	Level *int `json:"level" enum:"1,2,3"`
}

func newConstraintTagsRoutes() *doc.Docs {
	docs := doc.New(doc.API{Title: "Constraint tags API", Version: "1.0.0"})
	router := stdadapter.New(http.NewServeMux(), docs)
	noop := func(http.ResponseWriter, *http.Request) {}

	api.Post(router, "/captures", noop,
		doc.OperationID("createCapture"),
		doc.BodyWithExample(constraintTagsCapture{}, constraintTagsCapture{
			TargetType: "task",
			Priority:   1,
			Title:      "Write docs",
			Slug:       "write-docs",
			Estimate:   2.5,
		}),
		doc.Status(http.StatusCreated, constraintTagsCapture{}),
	)
	return docs
}

func TestConstraintTagsSpecMatchesGolden(t *testing.T) {
	docs := newConstraintTagsRoutes()

	got, err := json.MarshalIndent(doc.GenerateOpenAPI(docs), "", "  ")
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	assertMatchesGolden(t, "testdata/openapi/constraint-tags.json", got)
}

func TestConstraintTagsAreValid(t *testing.T) {
	if err := doc.CheckSchemaTags(newConstraintTagsRoutes()); err != nil {
		t.Fatalf("CheckSchemaTags() = %v, want nil", err)
	}
}
