// Package validator wraps go-playground/validator with a single shared instance
// and turns its errors into short, client-safe messages.
package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	govalidator "github.com/go-playground/validator/v10"
)

var (
	instance *govalidator.Validate
	once     sync.Once
)

func get() *govalidator.Validate {
	once.Do(func() {
		instance = govalidator.New(govalidator.WithRequiredStructEnabled())
		// Report the JSON field name the client actually sent, not the Go field.
		instance.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
			if name == "" || name == "-" {
				return field.Name
			}
			return name
		})
	})
	return instance
}

// Error is returned when a request body fails its `validate` tags. Its message
// is safe to send back to the client verbatim.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

// Message returns the client-safe text of err if it is a validation failure,
// and "" otherwise.
func Message(err error) string {
	var verr *Error
	if errors.As(err, &verr) {
		return verr.Message
	}
	return ""
}

// Struct validates s against its `validate` tags. The returned error names only
// the offending JSON field and constraint, never the Go struct or package.
func Struct(s any) error {
	err := get().Struct(s)
	if err == nil {
		return nil
	}

	var fieldErrs govalidator.ValidationErrors
	if !errors.As(err, &fieldErrs) {
		return &Error{Message: "Invalid request body"}
	}

	messages := make([]string, 0, len(fieldErrs))
	for _, fe := range fieldErrs {
		messages = append(messages, describe(fe))
	}
	return &Error{Message: strings.Join(messages, "; ")}
}

// Validator adapts this package to fiber.StructValidator so that every
// c.Bind().Body() call is validated without the handler having to remember.
type Validator struct{}

func (Validator) Validate(out any) error { return Struct(out) }

func describe(fe govalidator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
