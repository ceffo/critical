package domain

import "errors"

// Cross-cutting sentinels. DALs translate driver-specific errors into these
// before returning to a service. Handlers map them to HTTP statuses through
// internal/render.
var (
	ErrNotFound          = errors.New("not found")
	ErrCharacterNotFound = errors.New("character not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrConflict          = errors.New("conflict")
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
