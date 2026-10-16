-- name: GetActiveBuffs :one
SELECT buffs FROM active_buffs WHERE character_id = ?;

-- name: UpsertActiveBuffs :exec
INSERT INTO active_buffs (character_id, buffs) VALUES (?, ?)
ON CONFLICT(character_id) DO UPDATE SET buffs = excluded.buffs;

-- name: GetRoundNumber :one
SELECT round_number FROM round_state WHERE character_id = ?;

-- name: UpsertRoundNumber :exec
INSERT INTO round_state (character_id, round_number) VALUES (?, ?)
ON CONFLICT(character_id) DO UPDATE SET round_number = excluded.round_number;

-- name: ListSavedBuffs :many
SELECT id, name, duration, charges, effects, note, created_at FROM saved_buffs WHERE character_id = ? ORDER BY id;

-- name: InsertSavedBuff :exec
INSERT INTO saved_buffs (character_id, name, duration, charges, effects, note) VALUES (?, ?, ?, ?, ?, ?);

-- created_at is written as text in the layout of the column's default, so a
-- restored row reads and sorts like the others.

-- name: InsertSavedBuffWithID :exec
INSERT INTO saved_buffs (id, character_id, name, duration, charges, effects, note, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, CAST(sqlc.arg(created_at) AS TEXT));

-- name: DeleteSavedBuffs :exec
DELETE FROM saved_buffs WHERE character_id = ?;

-- name: GetInitiative :one
SELECT initiative_d10, initiative_entry_id FROM round_state WHERE character_id = ?;

-- name: UpsertInitiative :exec
INSERT INTO round_state (character_id, initiative_d10, initiative_entry_id) VALUES (?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET initiative_d10 = excluded.initiative_d10, initiative_entry_id = excluded.initiative_entry_id;
