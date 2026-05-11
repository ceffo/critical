-- GetActiveBuffs
SELECT buffs FROM active_buffs WHERE character_id = ?
;

-- UpsertActiveBuffs
INSERT INTO active_buffs (character_id, buffs) VALUES (?, ?)
ON CONFLICT(character_id) DO UPDATE SET buffs = excluded.buffs
;

-- GetRoundNumber
SELECT round_number FROM round_state WHERE character_id = ?
;

-- UpsertRoundNumber
INSERT INTO round_state (character_id, round_number) VALUES (?, ?)
ON CONFLICT(character_id) DO UPDATE SET round_number = excluded.round_number
;
