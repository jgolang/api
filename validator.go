package api

import (
	"fmt"
	"sync"
)

// Validator validates a decoded value.
type Validator interface {
	Validate(value any) error
}

var validatorRegistry = struct {
	sync.RWMutex
	validators map[string]Validator
}{
	validators: make(map[string]Validator),
}

// RegisterValidator registers a validation provider.
func RegisterValidator(name string, validator Validator) {
	if name == "" || validator == nil {
		return
	}
	validatorRegistry.Lock()
	defer validatorRegistry.Unlock()
	validatorRegistry.validators[name] = validator
}

// ValidatorByName returns a registered validation provider.
func ValidatorByName(name string) (Validator, error) {
	validatorRegistry.RLock()
	validator, ok := validatorRegistry.validators[name]
	validatorRegistry.RUnlock()
	if !ok {
		return nil, fmt.Errorf("api validator %q is not registered", name)
	}
	return validator, nil
}
