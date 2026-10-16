-- name: GetSpellSlots :one
SELECT slots FROM spell_slots WHERE character_id = $1;

-- name: UpsertSpellSlots :exec
INSERT INTO spell_slots (character_id, slots) VALUES ($1, $2)
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
  AND (sqlc.arg(class)::text = '' OR lower(class) = sqlc.arg(class)::text)
  AND (sqlc.arg(any_level)::bigint = 1 OR level = ANY(sqlc.arg(levels)::bigint[]));

-- name: SpellsByKey :many
SELECT * FROM spells WHERE key = ANY(sqlc.arg(keys)::text[]);

-- TextsByID reads the localized_texts rows of the keys through the primary
-- key (text_id, lang), never a scan of the table (ADR 0023).

-- name: TextsByID :many
SELECT text_id, lang, body FROM localized_texts WHERE text_id = ANY(sqlc.arg(ids)::bigint[]);
