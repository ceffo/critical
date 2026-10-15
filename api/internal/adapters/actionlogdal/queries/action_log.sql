-- InsertAction
INSERT INTO action_log (character_id, round, kind, summary, hp_delta, payload, transaction_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id
;

-- StartActionTransaction
UPDATE action_log SET transaction_id = id
WHERE id = ?
;

-- InsertSnapshot
INSERT INTO action_snapshots (action_id, state_key, before_state, after_state)
VALUES (?, ?, ?, ?)
;

-- InsertCorrection
INSERT INTO action_corrections (action_id, kind, hp_delta, summary, payload, note, step_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
;

-- GetAction
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at /*:time.Time:notnull*/
FROM action_log
WHERE character_id = ? AND id = ?
;

-- ListActions
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at /*:time.Time:notnull*/
FROM action_log
WHERE character_id = ?
ORDER BY id DESC
LIMIT ?
;

-- ListActiveActionsByKind
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at /*:time.Time:notnull*/
FROM action_log
WHERE character_id = ? AND kind = ?
  AND NOT EXISTS (
    SELECT 1 FROM action_corrections v
    WHERE v.action_id = action_log.id AND v.kind = 'void'
      AND NOT EXISTS (
        SELECT 1 FROM action_corrections r WHERE r.action_id = v.action_id AND r.kind = 'redo' AND r.id > v.id
      )
  )
ORDER BY id DESC
LIMIT ?
;

-- ListCorrectionsFrom
SELECT action_corrections.id, action_corrections.action_id, action_corrections.kind,
       action_corrections.hp_delta, action_corrections.summary, action_corrections.payload,
       action_corrections.note, action_corrections.step_id,
       action_corrections.created_at /*:time.Time:notnull*/
FROM action_corrections
JOIN action_log ON action_log.id = action_corrections.action_id
WHERE action_log.character_id = ? AND action_corrections.action_id >= ?
ORDER BY action_corrections.id
;

-- ListSnapshotsFrom
SELECT action_snapshots.id, action_snapshots.action_id, action_snapshots.state_key,
       action_snapshots.before_state, action_snapshots.after_state
FROM action_snapshots
JOIN action_log ON action_log.id = action_snapshots.action_id
WHERE action_log.character_id = ? AND action_snapshots.action_id >= ?
ORDER BY action_snapshots.id
;

-- LatestActionID
SELECT id
FROM action_log
WHERE character_id = ? AND kind = ?
ORDER BY id DESC
LIMIT 1
;

-- LatestActiveActionID
SELECT id
FROM action_log
WHERE character_id = ?
  AND NOT EXISTS (
    SELECT 1 FROM action_corrections v
    WHERE v.action_id = action_log.id AND v.kind = 'void'
      AND NOT EXISTS (
        SELECT 1 FROM action_corrections r WHERE r.action_id = v.action_id AND r.kind = 'redo' AND r.id > v.id
      )
  )
ORDER BY id DESC
LIMIT 1
;

-- ListTransactionActions
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at /*:time.Time:notnull*/
FROM action_log
WHERE character_id = ? AND transaction_id = ?
ORDER BY id DESC
;

-- LatestPoppedAction
SELECT action_log.id, action_log.transaction_id
FROM action_corrections v
JOIN action_log ON action_log.id = v.action_id
WHERE action_log.character_id = ? AND v.kind = 'void'
  AND NOT EXISTS (
    SELECT 1 FROM action_corrections r WHERE r.action_id = v.action_id AND r.kind = 'redo' AND r.id > v.id
  )
ORDER BY v.id DESC
LIMIT 1
;
