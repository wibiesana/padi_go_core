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
