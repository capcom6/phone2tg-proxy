package validator

import "github.com/go-playground/validator/v10"

type Validate = validator.Validate

// New creates a new validator instance with required struct validation enabled.
// WithRequiredStructEnabled ensures that struct fields marked as required
// are validated even when the struct itself is not explicitly validated.
func New() *validator.Validate {
	return validator.New(validator.WithRequiredStructEnabled())
}
