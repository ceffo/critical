-- ListXPEntries
SELECT id, entry_date, session, reason, base, bonus
FROM xp_entries WHERE character_id = ? ORDER BY id DESC
;

-- GetXPEntry
SELECT id, entry_date, session, reason, base, bonus
FROM xp_entries WHERE character_id = ? AND id = ?
;

-- InsertXPEntry
INSERT INTO xp_entries (character_id, entry_date, session, reason, base, bonus)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id
;

-- UpdateXPEntry
UPDATE xp_entries SET session = ?, reason = ?, base = ?, bonus = ?
WHERE character_id = ? AND id = ?
RETURNING id
;

-- DeleteXPEntry
DELETE FROM xp_entries WHERE character_id = ? AND id = ?
RETURNING id
;
