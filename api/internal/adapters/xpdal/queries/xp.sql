-- name: ListXPEntries :many
SELECT id, entry_date, session, reason, base, bonus
FROM xp_entries WHERE character_id = ? ORDER BY id DESC;

-- name: GetXPEntry :one
SELECT id, entry_date, session, reason, base, bonus
FROM xp_entries WHERE character_id = ? AND id = ?;

-- name: InsertXPEntry :one
INSERT INTO xp_entries (character_id, entry_date, session, reason, base, bonus)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateXPEntry :execrows
UPDATE xp_entries SET session = ?, reason = ?, base = ?, bonus = ?
WHERE character_id = ? AND id = ?;

-- name: DeleteXPEntry :execrows
DELETE FROM xp_entries WHERE character_id = ? AND id = ?;

-- name: RestoreXPEntry :exec
INSERT INTO xp_entries (id, character_id, entry_date, session, reason, base, bonus)
VALUES (?, ?, ?, ?, ?, ?, ?);
