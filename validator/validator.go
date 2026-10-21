package validator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Use json tag name for errors instead of struct field names
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return fld.Name
		}
		return name
	})
}

// GetValidator returns singleton validator instance
func GetValidator() *validator.Validate {
	return validate
}

// ValidationErrorDetails map of field -> message
type ValidationErrorDetails map[string]string

// ValidateStruct validates struct and returns human-readable error map
func ValidateStruct(s interface{}) (ValidationErrorDetails, bool) {
	err := validate.Struct(s)
	if err == nil {
		return nil, true
	}

	errors := make(ValidationErrorDetails)
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			tag := e.Tag()
			param := e.Param()

			var msg string
			switch tag {
			case "required":
				msg = fmt.Sprintf("%s is required", field)
			case "email":
				msg = fmt.Sprintf("%s must be a valid email address", field)
			case "min":
				msg = fmt.Sprintf("%s must be at least %s characters/items", field, param)
			case "max":
				msg = fmt.Sprintf("%s may not be greater than %s characters/items", field, param)
			case "numeric":
				msg = fmt.Sprintf("%s must be numeric", field)
			case "alphanum":
				msg = fmt.Sprintf("%s must only contain letters and numbers", field)
			case "eqfield":
				msg = fmt.Sprintf("%s must match %s", field, param)
			default:
				msg = fmt.Sprintf("%s failed validation rule: %s", field, tag)
			}
			errors[field] = msg
		}
	} else {
		errors["general"] = err.Error()
	}

	return errors, false
}

// BindJSON decodes request body and validates the target struct
func BindJSON(r *http.Request, target interface{}) (ValidationErrorDetails, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ValidationErrorDetails{"body": "failed to read request body"}, err
	}

	if len(body) == 0 {
		return ValidationErrorDetails{"body": "request body cannot be empty"}, io.EOF
	}

	if err := json.Unmarshal(body, target); err != nil {
		return ValidationErrorDetails{"body": fmt.Sprintf("invalid JSON payload: %v", err)}, err
	}

	if errs, valid := ValidateStruct(target); !valid {
		return errs, fmt.Errorf("validation failed")
	}

	return nil, nil
}

// Bind decodes request body and validates directly into a typed struct pointer
func Bind[T any](r *http.Request) (*T, ValidationErrorDetails, error) {
	var target T
	errs, err := BindJSON(r, &target)
	if err != nil {
		return nil, errs, err
	}
	return &target, nil, nil
}

// FormValidator provides a fluent, procedural validation builder
type FormValidator struct {
	data   map[string]interface{}
	errors ValidationErrorDetails
}

// New creates a new fluent FormValidator from http.Request or map
func New(source ...interface{}) *FormValidator {
	fv := &FormValidator{
		data:   make(map[string]interface{}),
		errors: make(ValidationErrorDetails),
	}
	if len(source) > 0 {
		switch s := source[0].(type) {
		case *http.Request:
			_ = s.ParseForm()
			for k, v := range s.Form {
				if len(v) == 1 {
					fv.data[k] = v[0]
				} else if len(v) > 1 {
					fv.data[k] = v
				}
			}
		case map[string]interface{}:
			fv.data = s
		}
	}
	return fv
}

// Set adds a key-value pair to validator
func (v *FormValidator) Set(key string, val interface{}) *FormValidator {
	v.data[key] = val
	return v
}

// Required ensures fields are present and not empty
func (v *FormValidator) Required(fields ...string) *FormValidator {
	for _, f := range fields {
		val, exists := v.data[f]
		if !exists || val == nil || fmt.Sprintf("%v", val) == "" {
			if _, already := v.errors[f]; !already {
				v.errors[f] = fmt.Sprintf("%s is required", f)
			}
		}
	}
	return v
}

// Email ensures fields contain valid email formats
func (v *FormValidator) Email(fields ...string) *FormValidator {
	for _, f := range fields {
		if val, exists := v.data[f]; exists && val != nil {
			str := fmt.Sprintf("%v", val)
			if str != "" && (!strings.Contains(str, "@") || !strings.Contains(str, ".")) {
				if _, already := v.errors[f]; !already {
					v.errors[f] = fmt.Sprintf("%s must be a valid email address", f)
				}
			}
		}
	}
	return v
}

// Min ensures string length is at least min characters
func (v *FormValidator) Min(field string, min int) *FormValidator {
	if val, exists := v.data[field]; exists && val != nil {
		str := fmt.Sprintf("%v", val)
		if len(str) < min {
			if _, already := v.errors[field]; !already {
				v.errors[field] = fmt.Sprintf("%s must be at least %d characters", field, min)
			}
		}
	}
	return v
}

// Max ensures string length is not greater than max characters
func (v *FormValidator) Max(field string, max int) *FormValidator {
	if val, exists := v.data[field]; exists && val != nil {
		str := fmt.Sprintf("%v", val)
		if len(str) > max {
			if _, already := v.errors[field]; !already {
				v.errors[field] = fmt.Sprintf("%s may not be greater than %d characters", field, max)
			}
		}
	}
	return v
}

// AddError manually appends an error message for a field
func (v *FormValidator) AddError(field, message string) *FormValidator {
	v.errors[field] = message
	return v
}

// Passes returns true if there are no validation errors
func (v *FormValidator) Passes() bool {
	return len(v.errors) == 0
}

// Fails returns true if there are validation errors
func (v *FormValidator) Fails() bool {
	return len(v.errors) > 0
}

// Errors returns the validation error map
func (v *FormValidator) Errors() ValidationErrorDetails {
	return v.errors
}
