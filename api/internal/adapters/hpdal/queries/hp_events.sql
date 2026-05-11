-- AppendHpEvent
INSERT INTO hp_events (character_id, delta, source) VALUES (?, ?, ?)
;

-- GetHpHistory
SELECT id, character_id, delta, source, created_at /*:time.Time:notnull*/
FROM hp_events
WHERE character_id = ?
ORDER BY id DESC
LIMIT ?
;

-- GetCharacterMaxHp
SELECT hp_max FROM characters WHERE id = ?
;

-- GetLastHpEvent
SELECT delta, source FROM hp_events WHERE character_id = ? ORDER BY id DESC LIMIT 1
;
