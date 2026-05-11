-- GetSpellSlots
SELECT slots FROM spell_slots WHERE character_id = ?
;

-- UpsertSpellSlots
INSERT INTO spell_slots (character_id, slots) VALUES (?, ?)
ON CONFLICT(character_id) DO UPDATE SET slots = excluded.slots
;
