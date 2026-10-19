package actionlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// The snapshot codec is the log's (ADR 0018 step 8): one Snapshotter per
// state-key family encodes a key's "before" and "after", decodes them and
// writes them back. A service declares only what it snapshots (its state type
// T and how to write and remove it); the envelope, the absence and the
// replay are the log's, tested once here.
//
// Every "before" and "after" is an envelope (a JSON object):
//
//	{"id":"arrows_plus3","state":{...}}   the thing as it was
//	{"id":"arrows_plus3","absent":true}   the thing did not exist
//
// id is the part of the key after KeySeparator ("" and left out for a key
// without one). It is in the envelope because a Restorer is handed the state
// alone; for a thing renamed outside the log it is the name to restore under
// (ADR 0033 step 26.3, settled when the snapshots were re-encoded).

// envelope is the stored form of a snapshot's "before" or "after".
type envelope struct {
	ID     string          `json:"id,omitempty"`
	Absent bool            `json:"absent,omitempty"`
	State  json.RawMessage `json:"state,omitempty"`
}

// ErrNotAnEnvelope is a stored state that is not the codec's envelope: an
// encoding the log no longer reads, or a corrupt one.
var ErrNotAnEnvelope = errors.New("snapshot state is not an envelope")

// EncodeState is the envelope of a thing present in a state: id (empty for a
// key without one) and state, the owning service's encoding of it.
func EncodeState(id string, state json.RawMessage) (json.RawMessage, error) {
	if len(state) == 0 {
		return nil, fmt.Errorf("encode snapshot %q: empty state", id)
	}
	return json.Marshal(envelope{ID: id, State: state})
}

// EncodeAbsent is the envelope of a thing absent from a state (not created
// yet, or gone).
func EncodeAbsent(id string) (json.RawMessage, error) {
	return json.Marshal(envelope{ID: id, Absent: true})
}

// DecodeEnvelope reads an envelope: its id, and its state (nil when absent).
// Anything else is ErrNotAnEnvelope.
func DecodeEnvelope(raw json.RawMessage) (id string, state json.RawMessage, err error) {
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrNotAnEnvelope, err)
	}
	switch {
	case e.Absent && len(e.State) > 0:
		return "", nil, fmt.Errorf("%w: absent with a state", ErrNotAnEnvelope)
	case e.Absent:
		return e.ID, nil, nil
	case len(e.State) == 0:
		return "", nil, fmt.Errorf("%w: neither a state nor absent", ErrNotAnEnvelope)
	}
	return e.ID, e.State, nil
}

// WriteFunc writes state back as the state of thing id ("" for a key without
// an id), inside the undo's or the redo's transaction (ctx carries it). It
// validates state first.
type WriteFunc[T any] func(ctx context.Context, characterID int64, id string, state T) error

// RemoveFunc makes thing id absent (deletes it), inside the undo's or the
// redo's transaction.
type RemoveFunc func(ctx context.Context, characterID int64, id string) error

// Snapshotter is the codec of one state-key family whose state is a T. A
// service declares it once (NewSnapshotter, MayBeAbsent, WithReplay), encodes
// its snapshots with it, and registers Restorer(write, remove) for the
// family: that value implements Restorer and Replayer.
type Snapshotter[T any] struct {
	family string
	absent bool
	write  WriteFunc[T]
	remove RemoveFunc
	stamp  func(state *T, entryID int64)
}

// NewSnapshotter is the codec of family, whose things are always present.
func NewSnapshotter[T any](family string) Snapshotter[T] {
	return Snapshotter[T]{family: family}
}

// MayBeAbsent lets the family's things be absent from a state (not created
// yet, or gone): Snapshot takes nil for them, and Restorer needs a remove.
func (s Snapshotter[T]) MayBeAbsent() Snapshotter[T] {
	s.absent = true
	return s
}

// WithReplay gives a replayed "after" its entry (ADR 0019 step 1): stamp sets
// entryID on a state encoded before its entry had an id.
func (s Snapshotter[T]) WithReplay(stamp func(state *T, entryID int64)) Snapshotter[T] {
	s.stamp = stamp
	return s
}

// Restorer is s writing states back through write, and absent ones through
// remove (nil unless MayBeAbsent).
func (s Snapshotter[T]) Restorer(write WriteFunc[T], remove RemoveFunc) Snapshotter[T] {
	s.write, s.remove = write, remove
	return s
}

// Family is the state-key family the codec is registered for.
func (s Snapshotter[T]) Family() string { return s.family }

// Codec is a family's Snapshotter with its restorer, as a service's module
// declares it to the log (domain.Module): it restores every key of Family()
// (and a Snapshotter replays them too: Replayer).
type Codec interface {
	Restorer
	Family() string
	// CanRestore is nil when the codec can write back every state it
	// decodes: it has a write, and a remove when its things may be absent.
	CanRestore() error
}

// CanRestore is nil when s was given a write (Restorer), and a remove when
// its things may be absent (MayBeAbsent).
func (s Snapshotter[T]) CanRestore() error {
	switch {
	case s.write == nil:
		return fmt.Errorf("%w: %s has no write", ErrNoRestorer, s.family)
	case s.absent && s.remove == nil:
		return fmt.Errorf("%w: %s may be absent but has no remove", ErrNoRestorer, s.family)
	}
	return nil
}

// ErrNoRestorer is a codec declared without the functions that write its
// states back.
var ErrNoRestorer = errors.New("snapshot codec cannot restore")

// Key is the state key of thing id: the family, or "family:id".
func (s Snapshotter[T]) Key(id string) string {
	if id == "" {
		return s.family
	}
	return s.family + KeySeparator + id
}

// Snapshot is the snapshot of thing id ("" for a key without one) going from
// before to after; nil is absent.
func (s Snapshotter[T]) Snapshot(id string, before, after *T) (Snapshot, error) {
	b, err := s.encode(id, before)
	if err != nil {
		return Snapshot{}, err
	}
	a, err := s.encode(id, after)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Key: s.Key(id), Before: b, After: a}, nil
}

func (s Snapshotter[T]) encode(id string, state *T) (json.RawMessage, error) {
	if state == nil {
		if !s.absent {
			return nil, fmt.Errorf("encode %s snapshot: absent, but its things are always present", s.family)
		}
		return EncodeAbsent(id)
	}
	raw, err := json.Marshal(*state)
	if err != nil {
		return nil, fmt.Errorf("encode %s snapshot: %w", s.family, err)
	}
	return EncodeState(id, raw)
}

// Decode reads a stored state: the thing's id and its state, nil when absent.
func (s Snapshotter[T]) Decode(raw json.RawMessage) (string, *T, error) {
	id, state, err := DecodeEnvelope(raw)
	if err != nil {
		return "", nil, fmt.Errorf("decode %s snapshot: %w", s.family, err)
	}
	if state == nil {
		if !s.absent {
			return "", nil, fmt.Errorf("decode %s snapshot: absent, but its things are always present", s.family)
		}
		return id, nil, nil
	}
	var v T
	if err := json.Unmarshal(state, &v); err != nil {
		return "", nil, fmt.Errorf("decode %s snapshot: %w", s.family, err)
	}
	return id, &v, nil
}

// Restore writes a stored state back: the thing's state, or its removal when
// absent (ADR 0018 step 2 with the "before", ADR 0019 step 1 with the
// "after").
func (s Snapshotter[T]) Restore(ctx context.Context, characterID int64, raw json.RawMessage) error {
	id, state, err := s.Decode(raw)
	if err != nil {
		return err
	}
	return s.put(ctx, characterID, id, state)
}

// Replay writes the "after" of entry entryID back on a redo, stamped with
// the entry when the family names it (WithReplay).
func (s Snapshotter[T]) Replay(ctx context.Context, characterID, entryID int64, raw json.RawMessage) error {
	id, state, err := s.Decode(raw)
	if err != nil {
		return err
	}
	if state != nil && s.stamp != nil {
		s.stamp(state, entryID)
	}
	return s.put(ctx, characterID, id, state)
}

func (s Snapshotter[T]) put(ctx context.Context, characterID int64, id string, state *T) error {
	var err error
	switch {
	case state == nil && s.remove == nil, state != nil && s.write == nil:
		return fmt.Errorf("restore %s: the codec has no restorer", s.Key(id))
	case state == nil:
		err = s.remove(ctx, characterID, id)
	default:
		err = s.write(ctx, characterID, id, *state)
	}
	if err != nil {
		return fmt.Errorf("restore %s: %w", s.Key(id), err)
	}
	return nil
}
