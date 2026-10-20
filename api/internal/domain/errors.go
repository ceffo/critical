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
	// ErrOverBudget is a save that leaves a budget over its limit and spends
	// more than before it (ADR 0037 step 10): nothing is written. A
	// *BudgetError names the field and the figures.
	ErrOverBudget = errors.New("over budget")
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

// BudgetError is the refusal of ADR 0037 step 10 on one pool: after the save,
// Spent is above Limit and above what was spent before it. Field is the wire
// name of the pool's field (e.g. "skillPoints", or "skillPoints.open_locks"
// for one skill's cap). It matches ErrOverBudget.
type BudgetError struct {
	Field        string
	Spent, Limit int
}

func (e *BudgetError) Error() string {
	return fmt.Errorf("%s over budget: %d of %d", e.Field, e.Spent, e.Limit).Error()
}

// Is makes errors.Is(err, ErrOverBudget) true for a BudgetError.
func (e *BudgetError) Is(target error) bool { return target == ErrOverBudget }
