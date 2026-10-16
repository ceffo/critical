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

-- ListSavedBuffs
SELECT id, name, duration, charges, effects, note, created_at /*:time.Time:notnull*/ FROM saved_buffs WHERE character_id = ? ORDER BY id
;

-- InsertSavedBuff
INSERT INTO saved_buffs (character_id, name, duration, charges, effects, note) VALUES (?, ?, ?, ?, ?, ?)
RETURNING id
;

-- InsertSavedBuffWithID
INSERT INTO saved_buffs (id, character_id, name, duration, charges, effects, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ? /*:string:notnull*/)
;

-- DeleteSavedBuffs
DELETE FROM saved_buffs WHERE character_id = ?
;

-- GetInitiative
SELECT initiative_d10, initiative_entry_id FROM round_state WHERE character_id = ?
;

-- UpsertInitiative
INSERT INTO round_state (character_id, initiative_d10, initiative_entry_id) VALUES (?, ?, ?)
ON CONFLICT(character_id) DO UPDATE SET initiative_d10 = excluded.initiative_d10, initiative_entry_id = excluded.initiative_entry_id
;
