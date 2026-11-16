package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/skills"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// freePicksMigration documents characters.proficiency_picks with the picks a class grants free.
// Right after it the run gives each character the slots it paid for them (returnFreeSlots, ADR
// 0037 step 22.7).
const freePicksMigration = "046_free_proficiencies.sql"

// nonweaponKey is the column's key of the non-weapon picks, as characterdal stores it.
const nonweaponKey = "nonweapon"

// returnFreeSlots is ADR 0037 step 22.7, the data step after migration 046: for each character
// and each stored non-weapon pick whose id one of its class entries grants free (an active entry
// at its realized level, a frozen one at its left-at level; step 22.5), the slots become the
// stored slots minus the cost the pick paid before (step 4 on the active entries' groups, ADR 0042
// step 15), never below 0 (skills.ReturnFreeSlots). Every other key of the column, and every other
// pick, is written back as stored. A character who holds none of them is not written; each
// converted one is logged with the ids and the slots returned. Nothing is refused. It runs once,
// with the migration: a second run would take the cost again. book is the catalogue
// (seeds/proficiencies.json) with the class definitions.
func returnFreeSlots(ctx context.Context, q storeq.Querier, book skills.Book) error {
	rows, err := q.CharacterClassEntries(ctx)
	if err != nil {
		return fmt.Errorf("list class entries: %w", err)
	}
	entries := map[int64]skills.Classes{}
	for _, r := range rows {
		e := skills.ClassLevel{Class: r.Class, Level: int(r.RealizedLevel)}
		if r.State == "frozen" {
			e.Level, e.Frozen = int(r.LeftAtLevel), true
		}
		entries[r.CharacterID] = append(entries[r.CharacterID], e)
	}
	chars, err := q.CharacterPicks(ctx)
	if err != nil {
		return fmt.Errorf("list picks: %w", err)
	}
	for _, c := range chars {
		cls, ok := entries[c.ID]
		if !ok {
			continue
		}
		column := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(c.ProficiencyPicks), &column); err != nil {
			return fmt.Errorf("character %d: decode proficiency picks: %w", c.ID, err)
		}
		raw, ok := column[nonweaponKey]
		if !ok {
			continue
		}
		var picks []skills.Pick
		if err := json.Unmarshal(raw, &picks); err != nil {
			return fmt.Errorf("character %d: decode non-weapon picks: %w", c.ID, err)
		}
		out, returned := skills.ReturnFreeSlots(book, cls, picks)
		if len(returned) == 0 {
			continue
		}
		if column[nonweaponKey], err = json.Marshal(encodedPicks(out)); err != nil {
			return fmt.Errorf("character %d: encode non-weapon picks: %w", c.ID, err)
		}
		encoded, err := json.Marshal(column)
		if err != nil {
			return fmt.Errorf("character %d: encode proficiency picks: %w", c.ID, err)
		}
		if err := q.SetCharacterPicks(ctx, storeq.SetCharacterPicksParams{ProficiencyPicks: string(encoded), ID: c.ID}); err != nil {
			return fmt.Errorf("character %d picks: %w", c.ID, err)
		}
		slog.Info("migration 046: the slots paid for proficiencies a class grants free returned",
			"character", c.ID, "returned", returned)
	}
	return nil
}

// storedPick is a non-weapon pick as the column stores it.
type storedPick struct {
	ID    string `json:"id"`
	Slots int    `json:"slots"`
}

func encodedPicks(ps []skills.Pick) []storedPick {
	out := make([]storedPick, len(ps))
	for i, p := range ps {
		out[i] = storedPick(p)
	}
	return out
}

// returnFreeSlotsFrom reads the catalogue of seedsDir (its classes checked against defs, whose
// features name what is free), then runs returnFreeSlots.
func returnFreeSlotsFrom(ctx context.Context, q storeq.Querier, seedsDir string, defs *classes.Registry) error {
	book, err := ruletablesadpt.ReadProficiencies(seedsDir, defs)
	if err != nil {
		return err
	}
	return returnFreeSlots(ctx, q, book)
}
