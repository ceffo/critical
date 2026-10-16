// Package textdal stores the localized texts of the editable reference data
// (item and weapon names) in localized_texts (ADR 0023): a text is a key with
// one row per language. The owning row holds the key. It is a helper of the
// DALs that own such a column, used inside their transactions.
package textdal

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/textdal/queries"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// Store reads and writes texts through q (joining the transaction ctx
// carries, sqlite.InTx).
type Store struct {
	q sqlite.Querier
}

// New returns a Store bound to q.
func New(q sqlite.Querier) Store { return Store{q: q} }

func (s Store) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, s.q)}
}

// Get is the text under key (empty when it has no row).
func (s Store) Get(ctx context.Context, key int64) (localized.Text, error) {
	rows, err := queries.GetText(key).All(ctx, s.exec(ctx))
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
	var key int64
	// An aggregate: bob's static parser has none (docs/backend.md).
	if err := sqlite.Conn(ctx, s.q).QueryRowContext(ctx,
		`SELECT COALESCE(MAX(text_id), 0) + 1 FROM localized_texts`).Scan(&key); err != nil {
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
	if _, err := queries.DeleteText(key).Exec(ctx, s.exec(ctx)); err != nil {
		return fmt.Errorf("delete text %d: %w", key, err)
	}
	return nil
}

func (s Store) insert(ctx context.Context, key int64, t localized.Text) error {
	for _, lang := range t.Languages() {
		if _, err := queries.InsertText([]queries.InsertText_LocalizedTexts{{TextID: key, Lang: lang, Body: t[lang]}}).
			Exec(ctx, s.exec(ctx)); err != nil {
			return fmt.Errorf("insert text %d %s: %w", key, lang, err)
		}
	}
	return nil
}
