package domain

import (
	"errors"
	"fmt"
)

// Cross-cutting sentinels. DALs translate driver-specific errors into these
// before returning to a service. Handlers map them to HTTP statuses through
// internal/render.
var (
	ErrNotFound          = errors.New("not found")
	ErrCharacterNotFound = errors.New("character not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrConflict          = errors.New("conflict")
	// ErrUnavailable is a request the database could not serve in time: it
	// is stopped, starting or too slow to answer within the request's
	// deadline. Nothing the request asked to write is known to be saved.
	ErrUnavailable = errors.New("database unavailable")
)

// FieldError is an invalid-input failure tied to one request field, so the
// UI can show the message under that field. It matches ErrInvalidInput.
type FieldError struct {
	Field   string // the wire name of the field, e.g. "effectDice"
	Message string
}

func (e *FieldError) Error() string { return e.Message }

// Is makes errors.Is(err, ErrInvalidInput) true for a FieldError.
func (e *FieldError) Is(target error) bool { return target == ErrInvalidInput }

// FieldErrorf is a FieldError whose message is formatted like fmt.Errorf's.
// It is error construction, the one place in the domain a message is
// formatted.
func FieldErrorf(field, format string, args ...any) *FieldError {
	return &FieldError{Field: field, Message: fmt.Errorf(format, args...).Error()}
}
