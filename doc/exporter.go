package doc

import (
	"fmt"
	"net/http"
	"sync"
)

// Exporter serves documentation UI for an OpenAPI URL.
type Exporter interface {
	Handler(openAPIURL string) http.HandlerFunc
}

type swaggerExporter struct{}

func (exporter swaggerExporter) Handler(openAPIURL string) http.HandlerFunc {
	return SwaggerUIHandler(openAPIURL)
}

var exporterRegistry = struct {
	sync.RWMutex
	exporters map[string]Exporter
}{
	exporters: map[string]Exporter{
		"swagger": swaggerExporter{},
	},
}

// RegisterExporter registers a documentation UI exporter.
func RegisterExporter(name string, exporter Exporter) {
	if name == "" || exporter == nil {
		return
	}
	exporterRegistry.Lock()
	defer exporterRegistry.Unlock()
	exporterRegistry.exporters[name] = exporter
}

// ExporterByName returns a registered documentation UI exporter.
func ExporterByName(name string) (Exporter, error) {
	exporterRegistry.RLock()
	exporter, ok := exporterRegistry.exporters[name]
	exporterRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("doc exporter %q is not registered", name)
	}
	return exporter, nil
}
