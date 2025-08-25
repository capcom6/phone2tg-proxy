package validator

// Validatable can be implemented by request/DTO structs to run custom validation
// in addition to tag-based validation.
type Validatable interface {
	Validate() error
}
