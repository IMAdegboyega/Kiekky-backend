// internal/common/utils/validator.go
// Input validation using struct tags

package utils

import (
    "fmt"
    "strings"
    
    "github.com/go-playground/validator/v10"
)

// Global validator instance
var validate = validator.New()

// ValidateStruct validates a struct based on its tags
func ValidateStruct(s interface{}) error {
    err := validate.Struct(s)
    if err != nil {
        // Format validation errors into readable messages
        var errors []string
        for _, err := range err.(validator.ValidationErrors) {
            errors = append(errors, formatFieldError(err))
        }
        return fmt.Errorf(strings.Join (errors, ", "))
    }
    return nil
}

// formatFieldError converts validator errors to human-readable messages
func formatFieldError(fe validator.FieldError) string {
    field := fe.Field()
    tag := fe.Tag()
    
    switch tag {
    case "required":
        return fmt.Sprintf("%s is required", field)
    case "email":
        return fmt.Sprintf("%s must be a valid email", field)
    case "min":
        return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
    case "max":
        return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
    case "len":
        return fmt.Sprintf("%s must be exactly %s characters", field, fe.Param())
    case "alphanum":
        return fmt.Sprintf("%s must contain only letters and numbers", field)
    case "numeric":
        return fmt.Sprintf("%s must contain only numbers", field)
    case "eqfield":
        return fmt.Sprintf("%s must match %s", field, fe.Param())
    case "e164":
        return fmt.Sprintf("%s must be a valid phone number (E.164 format)", field)
    default:
        return fmt.Sprintf("%s is invalid", field)
    }
}