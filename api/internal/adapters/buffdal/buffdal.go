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

// DAL is the SQLite-backed implementation of buffsvc.DAL. Every call runs on
// the transaction carried by ctx when there is one (sqlite.Conn), so buff
// writes commit together with their action-log entry.
type DAL struct {
	q sqlite.Querier
}

// New returns a DAL on the pool; calls join the ctx transaction if any.
func New(q sqlite.Querier) *DAL { return &DAL{q: q} }

func (d *DAL) exec(ctx context.Context) sqlite.BobExec {
	return sqlite.BobExec{Querier: sqlite.Conn(ctx, d.q)}
}

// jsonBuff mirrors buffsvc.BuffEffect with JSON tags so we can preserve the
// existing on-disk format. Rows written before qki.8 also carry a "type"
// (charge|duration) key; it is ignored: the counters say how a buff ends.
type jsonBuff struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Charges  int         `json:"charges"`
	Duration int         `json:"duration"`
	Effects  jsonEffects `json:"effects"`
	Note     string      `json:"note,omitempty"`
}

type jsonEffects struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrSet     *int     `json:"str_set,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
	ToHit      *int     `json:"to_hit,omitempty"`
	Damage     *int     `json:"damage,omitempty"`
}

func (j jsonBuff) toDomain() buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: j.ID, Name: j.Name, Charges: j.Charges, Duration: j.Duration,
		Effects: buffsvc.BuffEffects(j.Effects), Note: j.Note,
	}
}

func fromDomain(b buffsvc.BuffEffect) jsonBuff {
	return jsonBuff{
		ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
		Effects: jsonEffects(b.Effects), Note: b.Note,
	}
}

// GetActiveBuffs returns the persisted list (empty when none).
func (d *DAL) GetActiveBuffs(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error) {
	blob, err := queries.GetActiveBuffs(characterID).One(ctx, d.exec(ctx))
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
	}).Exec(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("upsert active_buffs: %w", err)
	}
	return nil
}

// GetRoundNumber returns the persisted round; 0 when no row exists.
func (d *DAL) GetRoundNumber(ctx context.Context, characterID int64) (int, error) {
	round, err := queries.GetRoundNumber(characterID).One(ctx, d.exec(ctx))
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
	}).Exec(ctx, d.exec(ctx))
	if err != nil {
		return fmt.Errorf("upsert round_state: %w", err)
	}
	return nil
}

// ListSavedBuffs returns the character's templates, oldest first.
func (d *DAL) ListSavedBuffs(ctx context.Context, characterID int64) ([]buffsvc.SavedBuff, error) {
	rows, err := queries.ListSavedBuffs(characterID).All(ctx, d.exec(ctx))
	if err != nil {
		return nil, fmt.Errorf("query saved_buffs: %w", err)
	}
	out := make([]buffsvc.SavedBuff, 0, len(rows))
	for _, r := range rows {
		var eff jsonEffects
		if err := json.Unmarshal([]byte(r.Effects), &eff); err != nil {
			return nil, fmt.Errorf("unmarshal saved buff %d effects: %w", r.ID, err)
		}
		out = append(out, buffsvc.SavedBuff{
			ID: r.ID, Name: r.Name, Duration: int(r.Duration), Charges: int(r.Charges),
			Effects: buffsvc.BuffEffects(eff), Note: r.Note,
		})
	}
	return out, nil
}

// ReplaceSavedBuffs makes the stored list equal to saved.
func (d *DAL) ReplaceSavedBuffs(ctx context.Context, characterID int64, saved []buffsvc.SavedBuff) error {
	if _, err := queries.DeleteSavedBuffs(characterID).Exec(ctx, d.exec(ctx)); err != nil {
		return fmt.Errorf("delete saved_buffs: %w", err)
	}
	for _, s := range saved {
		eff, err := json.Marshal(jsonEffects(s.Effects))
		if err != nil {
			return fmt.Errorf("marshal saved buff effects: %w", err)
		}
		if s.ID != 0 {
			_, err = queries.InsertSavedBuffWithID([]queries.InsertSavedBuffWithID_SavedBuffs{{
				ID: s.ID, CharacterID: characterID, Name: s.Name, Duration: int64(s.Duration),
				Charges: int64(s.Charges), Effects: string(eff), Note: s.Note,
			}}).Exec(ctx, d.exec(ctx))
		} else {
			_, err = queries.InsertSavedBuff([]queries.InsertSavedBuff_SavedBuffs{{
				CharacterID: characterID, Name: s.Name, Duration: int64(s.Duration),
				Charges: int64(s.Charges), Effects: string(eff), Note: s.Note,
			}}).One(ctx, d.exec(ctx))
		}
		if err != nil {
			return fmt.Errorf("insert saved_buffs: %w", err)
		}
	}
	return nil
}
