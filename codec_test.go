package api

import (
	"bytes"
	"io"
	"testing"
)

type testCodec struct{}

func (codec testCodec) ContentType() string {
	return "application/test"
}

func (codec testCodec) Encode(w io.Writer, value any) error {
	_, err := w.Write([]byte("encoded"))
	return err
}

func TestDefaultJSONCodecIsRegistered(t *testing.T) {
	codec, err := CodecByName("json")
	if err != nil {
		t.Fatalf("expected json codec, got error: %v", err)
	}
	if codec.ContentType() != "application/json" {
		t.Fatalf("unexpected content type: %s", codec.ContentType())
	}

	var buf bytes.Buffer
	if err := codec.Encode(&buf, map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if buf.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("unexpected json output: %q", buf.String())
	}
}

func TestRegisteredCodecCanBeResolved(t *testing.T) {
	RegisterCodec("test-codec", testCodec{})

	codec, err := CodecByName("test-codec")
	if err != nil {
		t.Fatalf("expected registered codec, got error: %v", err)
	}
	if codec.ContentType() != "application/test" {
		t.Fatalf("unexpected content type: %s", codec.ContentType())
	}
}

func TestCodecByNameReturnsErrorForMissingCodec(t *testing.T) {
	codec, err := CodecByName("missing-codec")
	if err == nil {
		t.Fatalf("expected error")
	}
	if codec != nil {
		t.Fatalf("expected nil codec, got %#v", codec)
	}
}

func TestRegisterCodecIgnoresInvalidInput(t *testing.T) {
	RegisterCodec("", testCodec{})
	RegisterCodec("nil-codec", nil)

	if _, err := CodecByName(""); err == nil {
		t.Fatalf("empty codec name should not be registered")
	}
	if _, err := CodecByName("nil-codec"); err == nil {
		t.Fatalf("nil codec should not be registered")
	}
}
