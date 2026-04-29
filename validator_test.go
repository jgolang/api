package api

import (
	"errors"
	"testing"
)

type testValidator struct {
	err error
}

func (validator testValidator) Validate(value any) error {
	return validator.err
}

func TestRegisteredValidatorCanBeResolved(t *testing.T) {
	RegisterValidator("test-validator", testValidator{})

	validator, err := ValidatorByName("test-validator")
	if err != nil {
		t.Fatalf("expected registered validator, got error: %v", err)
	}
	if err := validator.Validate(struct{}{}); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestRegisteredValidatorReturnsValidationError(t *testing.T) {
	expected := errors.New("invalid")
	RegisterValidator("failing-validator", testValidator{err: expected})

	validator, err := ValidatorByName("failing-validator")
	if err != nil {
		t.Fatalf("expected registered validator, got error: %v", err)
	}
	if err := validator.Validate(struct{}{}); !errors.Is(err, expected) {
		t.Fatalf("expected validation error %v, got %v", expected, err)
	}
}

func TestValidatorByNameReturnsErrorForMissingValidator(t *testing.T) {
	validator, err := ValidatorByName("missing-validator")
	if err == nil {
		t.Fatalf("expected error")
	}
	if validator != nil {
		t.Fatalf("expected nil validator, got %#v", validator)
	}
}

func TestRegisterValidatorIgnoresInvalidInput(t *testing.T) {
	RegisterValidator("", testValidator{})
	RegisterValidator("nil-validator", nil)

	if _, err := ValidatorByName(""); err == nil {
		t.Fatalf("empty validator name should not be registered")
	}
	if _, err := ValidatorByName("nil-validator"); err == nil {
		t.Fatalf("nil validator should not be registered")
	}
}
