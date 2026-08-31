package validator_test

import (
	"testing"

	"github.com/wibiesana/padi_go_core/validator"
)

type SamplePayload struct {
	Name  string `json:"name" validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"min=18"`
}

func TestStructValidation(t *testing.T) {
	// Valid struct
	valid := SamplePayload{
		Name:  "Alex",
		Email: "alex@example.com",
		Age:   25,
	}

	errs, ok := validator.ValidateStruct(valid)
	if !ok || len(errs) > 0 {
		t.Fatalf("Expected valid payload to pass, got errs: %v", errs)
	}

	// Invalid struct
	invalid := SamplePayload{
		Name:  "Al",
		Email: "not-an-email",
		Age:   12,
	}

	errs, ok = validator.ValidateStruct(invalid)
	if ok {
		t.Fatalf("Expected invalid payload to fail validation")
	}

	if _, exists := errs["name"]; !exists {
		t.Errorf("Expected 'name' validation error")
	}
	if _, exists := errs["email"]; !exists {
		t.Errorf("Expected 'email' validation error")
	}
	if _, exists := errs["age"]; !exists {
		t.Errorf("Expected 'age' validation error")
	}
}
