-- The spells a character knows, by catalogue key, in the sheet's order.

-- name: ListSpellbook :many
SELECT spell_key FROM spellbook_entries WHERE character_id = $1 ORDER BY position, spell_key;
