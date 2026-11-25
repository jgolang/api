package doc

import (
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

type minimumOnStringFixture struct {
	Name string `json:"name" minimum:"1"`
}

type maximumOnStringPointerFixture struct {
	Name *string `json:"name" maximum:"9"`
}

type minimumOnStructFixture struct {
	Address schemaAddress `json:"address" minimum:"1"`
}

type maximumOnSliceFixture struct {
	Scores []int `json:"scores" maximum:"9"`
}

type minLengthOnIntFixture struct {
	Count int `json:"count" minLength:"1"`
}

type maxLengthOnFloatFixture struct {
	Ratio float64 `json:"ratio" maxLength:"5"`
}

type patternOnIntPointerFixture struct {
	Count *int `json:"count" pattern:"^[0-9]+$"`
}

type patternOnBoolFixture struct {
	Done bool `json:"done" pattern:"^true$"`
}

type minimumOnQuotedFixture struct {
	ExternalID uint `json:"external_id,string" minimum:"1"`
}

type maximumOnQuotedPointerFixture struct {
	Ratio *float64 `json:"ratio,string" maximum:"1"`
}

type minimumAboveMaximumFixture struct {
	Score float64 `json:"score" minimum:"10" maximum:"5"`
}

type unsignedMaximumBelowZeroFixture struct {
	Count uint `json:"count" maximum:"-1"`
}

type minLengthAboveMaxLengthFixture struct {
	Name string `json:"name" minLength:"10" maxLength:"5"`
}

type intDefaultNotInEnumFixture struct {
	Priority int `json:"priority" enum:"1,2,3" default:"4"`
}

type stringDefaultNotInEnumFixture struct {
	Status string `json:"status" enum:"open,closed" default:"Open"`
}

type pointerDefaultNotInEnumFixture struct {
	Level *float64 `json:"level" enum:"0.5,1.5" default:"2"`
}

func TestCheckSchemaTagsReportsTagsOnTheWrongTypeAndContradictions(t *testing.T) {
	cases := []struct {
		name  string
		body  interface{}
		parts []string
	}{
		{"minimum on string", minimumOnStringFixture{}, []string{"minimumOnStringFixture", "Name", "minimum", "number and integer"}},
		{"maximum on string pointer", maximumOnStringPointerFixture{}, []string{"maximumOnStringPointerFixture", "Name", "maximum", "number and integer"}},
		{"minimum on struct", minimumOnStructFixture{}, []string{"minimumOnStructFixture", "Address", "minimum", "number and integer"}},
		{"maximum on slice", maximumOnSliceFixture{}, []string{"maximumOnSliceFixture", "Scores", "maximum", "number and integer"}},
		{"minLength on int", minLengthOnIntFixture{}, []string{"minLengthOnIntFixture", "Count", "minLength", "string"}},
		{"maxLength on float", maxLengthOnFloatFixture{}, []string{"maxLengthOnFloatFixture", "Ratio", "maxLength", "string"}},
		{"pattern on int pointer", patternOnIntPointerFixture{}, []string{"patternOnIntPointerFixture", "Count", "pattern", "string"}},
		{"pattern on bool", patternOnBoolFixture{}, []string{"patternOnBoolFixture", "Done", "pattern", "string"}},
		{"minimum on quoted uint", minimumOnQuotedFixture{}, []string{"minimumOnQuotedFixture", "ExternalID", "minimum", "number and integer", "string option"}},
		{"maximum on quoted float pointer", maximumOnQuotedPointerFixture{}, []string{"maximumOnQuotedPointerFixture", "Ratio", "maximum", "number and integer", "string option"}},
		{"minimum above maximum", minimumAboveMaximumFixture{}, []string{"minimumAboveMaximumFixture", "Score", "minimum 10", "maximum 5"}},
		{"unsigned maximum below zero", unsignedMaximumBelowZeroFixture{}, []string{"unsignedMaximumBelowZeroFixture", "Count", "minimum 0", "maximum -1"}},
		{"minLength above maxLength", minLengthAboveMaxLengthFixture{}, []string{"minLengthAboveMaxLengthFixture", "Name", "minLength 10", "maxLength 5"}},
		{"int default not in enum", intDefaultNotInEnumFixture{}, []string{"intDefaultNotInEnumFixture", "Priority", "default 4", "enum"}},
		{"string default not in enum", stringDefaultNotInEnumFixture{}, []string{"stringDefaultNotInEnumFixture", "Status", "default Open", "enum"}},
		{"pointer default not in enum", pointerDefaultNotInEnumFixture{}, []string{"pointerDefaultNotInEnumFixture", "Level", "default 2", "enum"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs := New(API{Title: "bad", Version: "1"})
			docs.Register("POST", "/items", Body(tc.body))

			err := CheckSchemaTags(docs)

			if err == nil {
				t.Fatal("CheckSchemaTags() = nil, want an error")
			}
			for _, part := range tc.parts {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("error %q does not mention %q", err, part)
				}
			}
			var appErr *errors.Error
			if !stderrors.As(err, &appErr) || appErr.Code == nil || appErr.Code.Str() != codes.CfgInvalid.Str() {
				t.Fatalf("error %v does not carry %v", err, codes.CfgInvalid)
			}
		})
	}
}

type validTagCombinationsFixture struct {
	Ratio      *float64  `json:"ratio" minimum:"0" maximum:"1"`
	Count      uint32    `json:"count" minimum:"5" maximum:"5"`
	Code       *string   `json:"code" minLength:"3" maxLength:"3" pattern:"^[A-Z]+$"`
	Due        time.Time `json:"due" pattern:"^2026"`
	Factor     float64   `json:"factor" enum:"1,2" default:"1.0"`
	Priority   *int      `json:"priority" enum:"1,2,3" default:"02"`
	Status     string    `json:"status" enum:"open,closed" default:"open"`
	Limit      int       `json:"limit" default:"10"`
	OnlyMax    int       `json:"only_max" maximum:"-5"`
	OnlyLength string    `json:"only_length" maxLength:"0"`
	Rank       int       `json:"rank,string" enum:"1,2,3" default:"2"`
	Flag       *bool     `json:"flag,string" enum:"true" default:"true"`
}

func TestCheckSchemaTagsAcceptsValidTagCombinations(t *testing.T) {
	docs := New(API{Title: "valid", Version: "1"})
	docs.Register("POST", "/items", Body(validTagCombinationsFixture{}))

	if err := CheckSchemaTags(docs); err != nil {
		t.Fatalf("CheckSchemaTags() = %v, want nil", err)
	}
}

func TestCheckSchemaTagsReportsAMalformedTagOnTheWrongTypeOnce(t *testing.T) {
	type malformedOnStringFixture struct {
		Name string `json:"name" minimum:"low"`
	}
	docs := New(API{Title: "bad", Version: "1"})
	docs.Register("POST", "/items", Body(malformedOnStringFixture{}))

	err := CheckSchemaTags(docs)

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 1 || !strings.Contains(err.Error(), "invalid minimum tag") {
		t.Fatalf("error %v, want only the parse error", err)
	}
}

func TestCheckSchemaTagsLeavesTheSpecUnchanged(t *testing.T) {
	type mistakesFixture struct {
		Name     string `json:"name" minimum:"1" maxLength:"2" minLength:"3"`
		Count    int    `json:"count" pattern:"^1$" enum:"1,2" default:"3"`
		Distance int    `json:"distance" minimum:"9" maximum:"1"`
	}
	properties := SchemaFromType(mistakesFixture{}).Properties
	cases := []struct {
		property string
		want     string
	}{
		{"name", `{"type":"string","minimum":1,"minLength":3,"maxLength":2}`},
		{"count", `{"type":"integer","format":"int32","enum":[1,2],"pattern":"^1$","default":3}`},
		{"distance", `{"type":"integer","format":"int32","minimum":9,"maximum":1}`},
	}

	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}
