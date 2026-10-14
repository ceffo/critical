package actionlogsvc

import "errors"

// Service-local sentinels, mapped to HTTP statuses in internal/render.
var (
	// ErrAlreadyVoided: amend or void on a voided entry.
	ErrAlreadyVoided = errors.New("entry already voided")
	// ErrVoidBlocked: a later non-voided entry changed the same state, so
	// restoring this entry's "before" state would overwrite it.
	ErrVoidBlocked = errors.New("void blocked")
	// ErrNotAmendable: only hp_change entries can be amended.
	ErrNotAmendable = errors.New("entry cannot be amended")
	// ErrNotLatest: only the newest entry of its kind can be adjusted; fix
	// an older one with void and a new roll.
	ErrNotLatest = errors.New("not the latest entry")
	// ErrAboveMaxHP: the amend or void would leave current HP above max HP
	// (HP-3, HP-4); fix the later heal first.
	ErrAboveMaxHP = errors.New("hp would exceed max hp")
)
