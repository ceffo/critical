package actionlogsvc

import "errors"

// Service-local sentinels, mapped to HTTP statuses in internal/render.
var (
	// ErrAlreadyVoided: an amend or adjustment of a voided entry.
	ErrAlreadyVoided = errors.New("entry already undone")
	// ErrNotAmendable: AmendLatest on an entry that is not the roll kind asked.
	ErrNotAmendable = errors.New("entry cannot be amended")
	// ErrNotLatest: only the latest roll can be adjusted.
	ErrNotLatest = errors.New("not the latest entry")
	// ErrTopChanged: an undo whose expected top is no longer the newest
	// non-voided entry (another action landed, or it was already popped).
	ErrTopChanged = errors.New("the newest entry changed")
	// ErrNothingToUndo: an undo on a log with no non-voided entry.
	ErrNothingToUndo = errors.New("nothing to undo")
	// ErrRedoChanged: a redo whose expected transaction is no longer the one
	// a redo replays (another action landed, or it was already replayed).
	ErrRedoChanged = errors.New("the transaction to redo changed")
	// ErrNothingToRedo: a redo with no popped transaction above the live top.
	ErrNothingToRedo = errors.New("nothing to redo")
	// ErrSnapshotNotReencoded: an undo or redo of an entry whose snapshot has
	// no envelope (ADR 0018 step 9): the release before the codec recorded it
	// during a rolling deploy and the data step has not re-encoded it yet.
	// Nothing is written.
	ErrSnapshotNotReencoded = errors.New("snapshot recorded by the previous release, not re-encoded yet")
)
