-- The spells a character knows, by catalogue key, in the sheet's order.

-- name: ListSpellbook :many
SELECT spell_key FROM spellbook_entries WHERE character_id = $1 ORDER BY position, spell_key;

-- name: AddSpellbookEntry :exec
INSERT INTO spellbook_entries (character_id, spell_key, position)
SELECT sqlc.arg(character_id), sqlc.arg(spell_key),
       COALESCE(MAX(position), -1) + 1 FROM spellbook_entries WHERE character_id = sqlc.arg(character_id)
ON CONFLICT (character_id, spell_key) DO NOTHING;

-- name: RemoveSpellbookEntry :exec
DELETE FROM spellbook_entries WHERE character_id = $1 AND spell_key = $2;

-- The spells a character failed to learn (ADR 0030 step 20.2), at the realized level of the failure.

-- name: GetLearnFailure :many
SELECT level FROM spellbook_learn_failures WHERE character_id = $1 AND spell_key = $2;

-- name: SetLearnFailure :exec
INSERT INTO spellbook_learn_failures (character_id, spell_key, level) VALUES ($1, $2, $3)
ON CONFLICT (character_id, spell_key) DO UPDATE SET level = EXCLUDED.level;

-- name: DeleteLearnFailure :exec
DELETE FROM spellbook_learn_failures WHERE character_id = $1 AND spell_key = $2;
