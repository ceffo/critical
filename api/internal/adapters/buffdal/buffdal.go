// Package buffdal is the storage adapter for buffsvc. The active buff list is
// persisted as JSON in active_buffs.buffs for simplicity; the round counter
// lives in round_state.round_number. Delegates to bob-generated typed queries.
package buffdal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/buffdal/queries"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// TxRunner is the storage-level transaction primitive. The composition root
// passes a *sqlite.DB that satisfies it.
type TxRunner interface {
	WrapTx(ctx context.Context, body func(sqlite.Querier) error) error
}

// DAL is the SQLite-backed implementation of buffsvc.DAL.
type DAL struct {
	q  sqlite.Querier
	tx TxRunner
}

// New returns a DAL bound to the pool plus a tx runner.
func New(q sqlite.Querier, tx TxRunner) *DAL { return &DAL{q: q, tx: tx} }

// newTxBound is used internally by WrapTx.
func newTxBound(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec() sqlite.BobExec { return sqlite.BobExec{Querier: d.q} }

// jsonBuff mirrors buffsvc.BuffEffect with JSON tags so we can preserve the
// existing on-disk format.
type jsonBuff struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Charges  int         `json:"charges"`
	Duration int         `json:"duration"`
	Effects  jsonEffects `json:"effects"`
}

type jsonEffects struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
}

func (j jsonBuff) toDomain() buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: j.ID, Name: j.Name, Type: j.Type, Charges: j.Charges, Duration: j.Duration,
		Effects: buffsvc.BuffEffects{
			MinStr: j.Effects.MinStr, SpellsBuff: j.Effects.SpellsBuff,
			WeaponBuff: j.Effects.WeaponBuff, Multiplier: j.Effects.Multiplier,
			SaveBonus: j.Effects.SaveBonus,
		},
	}
}

func fromDomain(b buffsvc.BuffEffect) jsonBuff {
	return jsonBuff{
		ID: b.ID, Name: b.Name, Type: b.Type, Charges: b.Charges, Duration: b.Duration,
		Effects: jsonEffects{
			MinStr: b.Effects.MinStr, SpellsBuff: b.Effects.SpellsBuff,
			WeaponBuff: b.Effects.WeaponBuff, Multiplier: b.Effects.Multiplier,
			SaveBonus: b.Effects.SaveBonus,
		},
	}
}

// GetActiveBuffs returns the persisted list (empty when none).
func (d *DAL) GetActiveBuffs(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error) {
	blob, err := queries.GetActiveBuffs(characterID).One(ctx, d.exec())
	if errors.Is(err, sql.ErrNoRows) {
		return []buffsvc.BuffEffect{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active_buffs: %w", err)
	}
	if blob == "" {
		return []buffsvc.BuffEffect{}, nil
	}
	var raw []jsonBuff
	if err := json.Unmarshal([]byte(blob), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal active_buffs: %w", err)
	}
	out := make([]buffsvc.BuffEffect, 0, len(raw))
	for _, j := range raw {
		out = append(out, j.toDomain())
	}
	return out, nil
}

// SaveActiveBuffs upserts the list.
func (d *DAL) SaveActiveBuffs(ctx context.Context, characterID int64, buffs []buffsvc.BuffEffect) error {
	raw := make([]jsonBuff, 0, len(buffs))
	for _, b := range buffs {
		raw = append(raw, fromDomain(b))
	}
	blob, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal active_buffs: %w", err)
	}
	_, err = queries.UpsertActiveBuffs([]queries.UpsertActiveBuffs_ActiveBuffs{
		{CharacterID: characterID, Buffs: string(blob)},
	}).Exec(ctx, d.exec())
	if err != nil {
		return fmt.Errorf("upsert active_buffs: %w", err)
	}
	return nil
}

// GetRoundNumber returns the persisted round; 0 when no row exists.
func (d *DAL) GetRoundNumber(ctx context.Context, characterID int64) (int, error) {
	round, err := queries.GetRoundNumber(characterID).One(ctx, d.exec())
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query round_state: %w", err)
	}
	return int(round), nil
}

// SaveRoundNumber upserts the counter.
func (d *DAL) SaveRoundNumber(ctx context.Context, characterID int64, round int) error {
	_, err := queries.UpsertRoundNumber([]queries.UpsertRoundNumber_RoundState{
		{CharacterID: characterID, RoundNumber: int64(round)},
	}).Exec(ctx, d.exec())
	if err != nil {
		return fmt.Errorf("upsert round_state: %w", err)
	}
	return nil
}

// WrapTx runs body with a tx-bound DAL. If d is already tx-bound, this panics
// — nested transactions are not supported on SQLite.
func (d *DAL) WrapTx(ctx context.Context, body func(buffsvc.DAL) error) error {
	if d.tx == nil {
		return fmt.Errorf("buffdal: WrapTx called on tx-bound DAL")
	}
	return d.tx.WrapTx(ctx, func(q sqlite.Querier) error {
		return body(newTxBound(q))
	})
}
