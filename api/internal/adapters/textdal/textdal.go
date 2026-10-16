// Package textdal stores the localized texts of the editable reference data
// (item and weapon names) in localized_texts (ADR 0023): a text is a key with
// one row per language. The owning row holds the key. It is a helper of the
// DALs that own such a column, used inside their transactions. It delegates
// to sqlc-generated queries.
package textdal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// Store reads and writes texts through q.
type Store struct {
	q queries.Querier
}

// New returns a Store running its queries on q. The composition root builds
// q over the pool, routed to the caller's transaction (ADR 0025 step 5.2).
func New(q queries.Querier) Store { return Store{q: q} }

// Get is the text under key (empty when it has no row).
func (s Store) Get(ctx context.Context, key int64) (localized.Text, error) {
	rows, err := s.q.GetText(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get text %d: %w", key, err)
	}
	out := make(localized.Text, len(rows))
	for _, r := range rows {
		out[r.Lang] = r.Body
	}
	return out, nil
}

// Add stores t under a new key, one more than the highest stored (the rule
// of every writer of localized_texts), and returns the key. The caller
// checked that t holds a language.
func (s Store) Add(ctx context.Context, t localized.Text) (int64, error) {
	key, err := s.q.NextTextKey(ctx)
	if err != nil {
		return 0, fmt.Errorf("next text key: %w", err)
	}
	if err := s.insert(ctx, key, t); err != nil {
		return 0, err
	}
	return key, nil
}

// Put replaces the rows under key with t's: one per language, none for a
// blank body.
func (s Store) Put(ctx context.Context, key int64, t localized.Text) error {
	if err := s.Delete(ctx, key); err != nil {
		return err
	}
	return s.insert(ctx, key, t)
}

// Delete removes every row under key.
func (s Store) Delete(ctx context.Context, key int64) error {
	if err := s.q.DeleteText(ctx, key); err != nil {
		return fmt.Errorf("delete text %d: %w", key, err)
	}
	return nil
}

func (s Store) insert(ctx context.Context, key int64, t localized.Text) error {
	for _, lang := range t.Languages() {
		if err := s.q.InsertText(ctx, queries.InsertTextParams{TextID: key, Lang: lang, Body: t[lang]}); err != nil {
			return fmt.Errorf("insert text %d %s: %w", key, lang, err)
		}
	}
	return nil
}
