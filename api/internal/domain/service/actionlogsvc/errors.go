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
)
