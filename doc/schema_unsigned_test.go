package doc

import "testing"

type unsignedFixture struct {
	Small     uint8   `json:"small"`
	Medium    uint16  `json:"medium"`
	Plain     uint    `json:"plain"`
	Wide      uint32  `json:"wide"`
	Widest    uint64  `json:"widest"`
	Optional  *uint32 `json:"optional,omitempty"`
	Bounded   uint    `json:"bounded" minimum:"5"`
	Malformed uint    `json:"malformed" minimum:"low"`
}

var unsignedCases = []struct {
	property string
	want     string
}{
	{"small", `{"type":"integer","format":"int32","minimum":0}`},
	{"medium", `{"type":"integer","format":"int32","minimum":0}`},
	{"plain", `{"type":"integer","format":"int64","minimum":0}`},
	{"wide", `{"type":"integer","format":"int64","minimum":0}`},
	{"widest", `{"type":"integer","minimum":0}`},
	{"optional", `{"type":"integer","format":"int64","nullable":true,"minimum":0}`},
	{"bounded", `{"type":"integer","format":"int64","minimum":5}`},
	{"malformed", `{"type":"integer","format":"int64","minimum":0}`},
}

func TestGenerateOpenAPIDocumentsUnsignedFields(t *testing.T) {
	properties := componentProperties(t, unsignedFixture{})

	for _, tc := range unsignedCases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}

func TestSchemaFromTypeDocumentsUnsignedFields(t *testing.T) {
	properties := SchemaFromType(unsignedFixture{}).Properties

	for _, tc := range unsignedCases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}

func TestSchemaFromTypeGivesEachUnsignedFieldItsOwnMinimum(t *testing.T) {
	properties := SchemaFromType(unsignedFixture{}).Properties

	*properties["small"].Minimum = 7

	if got := *properties["medium"].Minimum; got != 0 {
		t.Fatalf("medium minimum = %v after changing small, want 0", got)
	}
}
