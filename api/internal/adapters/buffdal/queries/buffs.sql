-- name: GetActiveBuffs :one
SELECT buffs FROM active_buffs WHERE character_id = $1;

-- name: UpsertActiveBuffs :exec
INSERT INTO active_buffs (character_id, buffs) VALUES ($1, $2)
ON CONFLICT(character_id) DO UPDATE SET buffs = excluded.buffs;

-- name: GetRoundNumber :one
SELECT round_number FROM round_state WHERE character_id = $1;

-- name: UpsertRoundNumber :exec
INSERT INTO round_state (character_id, round_number) VALUES ($1, $2)
ON CONFLICT(character_id) DO UPDATE SET round_number = excluded.round_number;

-- name: ListSavedBuffs :many
SELECT id, name, duration, charges, effects, note, created_at FROM saved_buffs WHERE character_id = $1 ORDER BY id;

-- name: InsertSavedBuff :exec
INSERT INTO saved_buffs (character_id, name, duration, charges, effects, note) VALUES ($1, $2, $3, $4, $5, $6);

-- A restored row keeps its creation time.

-- name: InsertSavedBuffWithID :exec
INSERT INTO saved_buffs (id, character_id, name, duration, charges, effects, note, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: DeleteSavedBuffs :exec
DELETE FROM saved_buffs WHERE character_id = $1;

-- name: GetInitiative :one
SELECT initiative_d10, initiative_entry_id FROM round_state WHERE character_id = $1;

-- name: UpsertInitiative :exec
INSERT INTO round_state (character_id, initiative_d10, initiative_entry_id) VALUES ($1, $2, $3)
ON CONFLICT(character_id) DO UPDATE SET initiative_d10 = excluded.initiative_d10, initiative_entry_id = excluded.initiative_entry_id;
