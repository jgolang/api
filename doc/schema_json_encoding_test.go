package doc

import (
	"strings"
	"testing"
)

type blob []byte

type hexBytes []byte

func (h hexBytes) MarshalJSON() ([]byte, error) {
	return []byte(`"00"`), nil
}

type byteElement byte

type quotedLevel int

func (l quotedLevel) MarshalJSON() ([]byte, error) {
	return []byte(`1`), nil
}

type pointerHexBytes []byte

func (h *pointerHexBytes) MarshalJSON() ([]byte, error) {
	return []byte(`"00"`), nil
}

type textCode int

func (c textCode) MarshalText() ([]byte, error) {
	return []byte("code"), nil
}

type selfEncodingByte byte

func (b selfEncodingByte) MarshalJSON() ([]byte, error) {
	return []byte(`0`), nil
}

type jsonEncodingFixture struct {
	Bytes        []byte             `json:"bytes"`
	Blob         blob               `json:"blob"`
	NamedElement []byteElement      `json:"named_element"`
	Hex          hexBytes           `json:"hex"`
	BytesPointer *[]byte            `json:"bytes_pointer"`
	Int          int                `json:"int,string"`
	Int8         int8               `json:"int8,string"`
	Int16        int16              `json:"int16,string"`
	Int32        int32              `json:"int32,string"`
	Int64        int64              `json:"int64,string"`
	Uint         uint               `json:"uint,string"`
	Uint8        uint8              `json:"uint8,string"`
	Uint16       uint16             `json:"uint16,string"`
	Uint32       uint32             `json:"uint32,string"`
	Uint64       uint64             `json:"uint64,string"`
	Bool         bool               `json:"bool,string"`
	Float32      float32            `json:"float32,string"`
	Float64      float64            `json:"float64,string"`
	Pointer      *int64             `json:"pointer,string"`
	Ranked       int                `json:"ranked,string" enum:"1,2,3" default:"2"`
	Flag         bool               `json:"flag,string" default:"true"`
	Ratio        float32            `json:"ratio,string" enum:"0.1,1.5"`
	PointerEnum  *int               `json:"pointer_enum,string" enum:"1,2"`
	BadEnum      int8               `json:"bad_enum,string" enum:"300"`
	Bounded      uint               `json:"bounded,string" minimum:"1"`
	Formatted    int64              `json:"formatted,omitempty,string" format:"int64"`
	Level        quotedLevel        `json:"level,string"`
	Text         string             `json:"text,string"`
	ByteArray    [2]byte            `json:"byte_array"`
	PointerHex   pointerHexBytes    `json:"pointer_hex"`
	TextCode     textCode           `json:"text_code,string"`
	SelfBytes    []selfEncodingByte `json:"self_bytes"`
	Example      int64              `json:"example,string" example:"42"`
	TextExample  int                `json:"text_example,string" example:"seven"`
}

var jsonEncodingCases = []struct {
	property string
	want     string
}{
	{"bytes", `{"type":"string","format":"byte","nullable":true}`},
	{"blob", `{"type":"string","format":"byte","nullable":true}`},
	{"named_element", `{"type":"string","format":"byte","nullable":true}`},
	{"hex", `{"type":"array","items":{"type":"integer","format":"int32","minimum":0}}`},
	{"bytes_pointer", `{"type":"string","format":"byte","nullable":true}`},
	{"int", `{"type":"string"}`},
	{"int8", `{"type":"string"}`},
	{"int16", `{"type":"string"}`},
	{"int32", `{"type":"string"}`},
	{"int64", `{"type":"string"}`},
	{"uint", `{"type":"string"}`},
	{"uint8", `{"type":"string"}`},
	{"uint16", `{"type":"string"}`},
	{"uint32", `{"type":"string"}`},
	{"uint64", `{"type":"string"}`},
	{"bool", `{"type":"string"}`},
	{"float32", `{"type":"string"}`},
	{"float64", `{"type":"string"}`},
	{"pointer", `{"type":"string","nullable":true}`},
	{"ranked", `{"type":"string","enum":["1","2","3"],"default":"2"}`},
	{"flag", `{"type":"string","default":"true"}`},
	{"ratio", `{"type":"string","enum":["0.1","1.5"]}`},
	{"pointer_enum", `{"type":"string","enum":["1","2",null],"nullable":true}`},
	{"bad_enum", `{"type":"string"}`},
	{"bounded", `{"type":"string","minimum":1}`},
	{"formatted", `{"type":"string","format":"int64"}`},
	{"level", `{"type":"integer","format":"int32"}`},
	{"text", `{"type":"string"}`},
	{"byte_array", `{"type":"array","items":{"type":"integer","format":"int32","minimum":0}}`},
	{"pointer_hex", `{"type":"array","items":{"type":"integer","format":"int32","minimum":0}}`},
	{"text_code", `{"type":"integer","format":"int32"}`},
	{"self_bytes", `{"type":"array","items":{"type":"integer","format":"int32","minimum":0},"nullable":true}`},
	{"example", `{"type":"string","example":"42"}`},
	{"text_example", `{"type":"string","example":"seven"}`},
}

func TestGenerateOpenAPIDocumentsFieldsAsEncodingJSONSendsThem(t *testing.T) {
	properties := componentProperties(t, jsonEncodingFixture{})

	for _, tc := range jsonEncodingCases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}

func TestSchemaFromTypeDocumentsFieldsAsEncodingJSONSendsThem(t *testing.T) {
	properties := SchemaFromType(jsonEncodingFixture{}).Properties

	for _, tc := range jsonEncodingCases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}

func TestSchemaFromTypeDocumentsAPointerToBytes(t *testing.T) {
	got := marshalSchema(t, SchemaFromType(new([]byte)))

	if want := `{"type":"string","format":"byte","nullable":true}`; got != want {
		t.Fatalf("SchemaFromType(*[]byte) = %s, want %s", got, want)
	}
}

func TestCheckSchemaTagsReportsAnOutOfRangeEnumOnAQuotedField(t *testing.T) {
	docs := New(API{Title: "bad", Version: "1"})
	docs.Register("POST", "/items", Body(struct {
		Small int8 `json:"small,string" enum:"300"`
	}{}))

	err := CheckSchemaTags(docs)

	if err == nil || !strings.Contains(err.Error(), "invalid enum tag") {
		t.Fatalf("CheckSchemaTags() = %v, want an invalid enum tag error", err)
	}
}

func TestSchemaFromTypeDocumentsANilSliceAsEncodingJSONSendsIt(t *testing.T) {
	type fixture struct {
		Plain    []string `json:"plain"`
		Omitted  []string `json:"omitted,omitempty"`
		Required []string `json:"required" validate:"required"`
		Array    [2]int   `json:"array"`
		Hex      hexBytes `json:"hex"`
	}
	properties := SchemaFromType(fixture{}).Properties
	cases := []struct {
		property string
		want     string
	}{
		{"plain", `{"type":"array","items":{"type":"string"},"nullable":true}`},
		{"omitted", `{"type":"array","items":{"type":"string"}}`},
		{"required", `{"type":"array","items":{"type":"string"}}`},
		{"array", `{"type":"array","items":{"type":"integer","format":"int32"}}`},
		{"hex", `{"type":"array","items":{"type":"integer","format":"int32","minimum":0}}`},
	}

	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			if got := marshalSchema(t, properties[tc.property]); got != tc.want {
				t.Fatalf("%s = %s, want %s", tc.property, got, tc.want)
			}
		})
	}
}
