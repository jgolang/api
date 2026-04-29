package api

import (
	"fmt"
	"sync"

	"github.com/jgolang/api/doc"
)

// AdapterFactory creates a Router from an adapter-specific target.
type AdapterFactory func(target any, docs *doc.Docs) (Router, error)

var adapterRegistry = struct {
	sync.RWMutex
	factories map[string]AdapterFactory
}{
	factories: make(map[string]AdapterFactory),
}

// RegisterAdapter registers a router adapter factory.
func RegisterAdapter(name string, factory AdapterFactory) {
	if name == "" || factory == nil {
		return
	}
	adapterRegistry.Lock()
	defer adapterRegistry.Unlock()
	adapterRegistry.factories[name] = factory
}

// NewRouter creates a Router from a registered adapter factory.
func NewRouter(name string, target any, docs *doc.Docs) (Router, error) {
	adapterRegistry.RLock()
	factory, ok := adapterRegistry.factories[name]
	adapterRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("api adapter %q is not registered", name)
	}
	return factory(target, docs)
}
