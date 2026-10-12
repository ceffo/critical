-- Keep only the newest row per character, then enforce one row per character
-- so the ON CONFLICT(character_id) upserts work.
DELETE FROM active_buffs WHERE id NOT IN (SELECT MAX(id) FROM active_buffs GROUP BY character_id);
DELETE FROM spell_slots WHERE id NOT IN (SELECT MAX(id) FROM spell_slots GROUP BY character_id);
DELETE FROM round_state WHERE id NOT IN (SELECT MAX(id) FROM round_state GROUP BY character_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_active_buffs_character_id ON active_buffs(character_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_spell_slots_character_id ON spell_slots(character_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_round_state_character_id ON round_state(character_id);
