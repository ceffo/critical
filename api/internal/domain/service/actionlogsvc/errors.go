package actionlogsvc

import "errors"

// Service-local sentinels, mapped to HTTP statuses in internal/render.
var (
	// ErrAlreadyVoided: amend or void on a voided entry.
	ErrAlreadyVoided = errors.New("entry already voided")
	// ErrNotAmendable: AmendLatest on an entry that is not the roll kind asked.
	ErrNotAmendable = errors.New("entry cannot be amended")
	// ErrNotLatest: only the newest non-voided entry can be voided (pop), and
	// only the latest roll can be adjusted.
	ErrNotLatest = errors.New("not the latest entry")
	// ErrTopChanged: an undo whose expected top is no longer the newest
	// non-voided entry (another action landed, or it was already popped).
	ErrTopChanged = errors.New("the newest entry changed")
	// ErrNothingToUndo: an undo on a log with no non-voided entry.
	ErrNothingToUndo = errors.New("nothing to undo")
)
