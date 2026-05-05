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
