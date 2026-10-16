-- name: GetSpellSlots :one
SELECT slots FROM spell_slots WHERE character_id = ?;

-- name: UpsertSpellSlots :exec
INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)
ON CONFLICT(character_id) DO UPDATE SET slots = excluded.slots;

-- SearchSpells reads the served rows (a row without a key is an older
-- duplicate the catalogue import left in place, ADR 0021 step 5.4.1) of a
-- class (lower(class) equal to the class the adapter lowered: case-insensitive
-- on every engine, never LIKE, whose case rule differs between SQLite and
-- PostgreSQL; '' is any class) and of the levels given (any level when
-- any_level is 1). The adapter sorts the result.

-- name: SearchSpells :many
SELECT * FROM spells
WHERE key IS NOT NULL
  AND (CAST(sqlc.arg(class) AS TEXT) = '' OR lower(class) = sqlc.arg(class))
  AND (CAST(sqlc.arg(any_level) AS INTEGER) = 1 OR level IN (sqlc.slice(levels)));

-- name: SpellsByKey :many
SELECT * FROM spells WHERE key IN (sqlc.slice(keys));

-- TextsByID reads the localized_texts rows of the keys through the primary
-- key (text_id, lang), never a scan of the table (ADR 0023).

-- name: TextsByID :many
SELECT text_id, lang, body FROM localized_texts WHERE text_id IN (sqlc.slice(ids));
