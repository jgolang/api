package api

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Codec serializes response bodies.
type Codec interface {
	ContentType() string
	Encode(w io.Writer, value any) error
}

type jsonCodec struct{}

func (codec jsonCodec) ContentType() string {
	return "application/json"
}

func (codec jsonCodec) Encode(w io.Writer, value any) error {
	return json.NewEncoder(w).Encode(value)
}

var codecRegistry = struct {
	sync.RWMutex
	codecs map[string]Codec
}{
	codecs: map[string]Codec{
		"json": jsonCodec{},
	},
}

// RegisterCodec registers a response serialization codec.
func RegisterCodec(name string, codec Codec) {
	if name == "" || codec == nil {
		return
	}
	codecRegistry.Lock()
	defer codecRegistry.Unlock()
	codecRegistry.codecs[name] = codec
}

// CodecByName returns a registered response serialization codec.
func CodecByName(name string) (Codec, error) {
	codecRegistry.RLock()
	codec, ok := codecRegistry.codecs[name]
	codecRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("api codec %q is not registered", name)
	}
	return codec, nil
}
