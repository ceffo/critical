-- name: ListXPEntries :many
SELECT id, entry_date, session, reason, base, bonus
FROM xp_entries WHERE character_id = $1 ORDER BY id DESC;

-- name: GetXPEntry :one
SELECT id, entry_date, session, reason, base, bonus
FROM xp_entries WHERE character_id = $1 AND id = $2;

-- name: InsertXPEntry :one
INSERT INTO xp_entries (character_id, entry_date, session, reason, base, bonus)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateXPEntry :execrows
UPDATE xp_entries SET session = $1, reason = $2, base = $3, bonus = $4
WHERE character_id = $5 AND id = $6;

-- name: DeleteXPEntry :execrows
DELETE FROM xp_entries WHERE character_id = $1 AND id = $2;

-- name: RestoreXPEntry :exec
INSERT INTO xp_entries (id, character_id, entry_date, session, reason, base, bonus)
VALUES ($1, $2, $3, $4, $5, $6, $7);
