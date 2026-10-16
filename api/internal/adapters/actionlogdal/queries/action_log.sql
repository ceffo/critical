-- InsertAction appends an entry. The log is append-only (ADR 0007, 0010): an
-- entry is never updated but by StartActionTransaction on the row just
-- inserted; an amend, a void (pop) or a redo is a new action_corrections row.

-- name: InsertAction :one
INSERT INTO action_log (character_id, round, kind, summary, hp_delta, payload, transaction_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: StartActionTransaction :exec
UPDATE action_log SET transaction_id = id
WHERE id = ?;

-- name: InsertSnapshot :exec
INSERT INTO action_snapshots (action_id, state_key, before_state, after_state)
VALUES (?, ?, ?, ?);

-- name: InsertCorrection :exec
INSERT INTO action_corrections (action_id, kind, hp_delta, summary, payload, note, step_id)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetAction :one
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = ? AND id = ?;

-- name: ListActions :many
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = ?
ORDER BY id DESC
LIMIT ?;

-- Which entries are popped is decided in Go (actionlog.StandingVoids, ADR
-- 0019 step 1) from ListPopCorrections; the queries below read the newest
-- rows, as many as the adapter needs to skip the popped ones.

-- name: ListActionsByKind :many
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = ? AND kind = ?
ORDER BY id DESC
LIMIT ?;

-- name: ListActionIDs :many
SELECT id
FROM action_log
WHERE character_id = ?
ORDER BY id DESC
LIMIT ?;

-- name: ListPopCorrections :many
SELECT action_corrections.id, action_corrections.action_id, action_corrections.kind
FROM action_corrections
JOIN action_log ON action_log.id = action_corrections.action_id
WHERE action_log.character_id = ? AND action_corrections.kind IN ('void', 'redo')
ORDER BY action_corrections.id;

-- name: ListCorrectionsFrom :many
SELECT action_corrections.id, action_corrections.action_id, action_corrections.kind,
       action_corrections.hp_delta, action_corrections.summary, action_corrections.payload,
       action_corrections.note, action_corrections.step_id, action_corrections.created_at
FROM action_corrections
JOIN action_log ON action_log.id = action_corrections.action_id
WHERE action_log.character_id = ? AND action_corrections.action_id >= ?
ORDER BY action_corrections.id;

-- name: ListSnapshotsFrom :many
SELECT action_snapshots.id, action_snapshots.action_id, action_snapshots.state_key,
       action_snapshots.before_state, action_snapshots.after_state
FROM action_snapshots
JOIN action_log ON action_log.id = action_snapshots.action_id
WHERE action_log.character_id = ? AND action_snapshots.action_id >= ?
ORDER BY action_snapshots.id;

-- name: LatestActionID :one
SELECT id
FROM action_log
WHERE character_id = ? AND kind = ?
ORDER BY id DESC
LIMIT 1;

-- name: ListTransactionActions :many
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = ? AND transaction_id = ?
ORDER BY id DESC;

-- ListHPDeltas reads every entry's own HP delta; the adapter adds the
-- corrections and sums the effective deltas in Go (actionlog.HPTotal, ADR
-- 0007 step 2).

-- name: ListHPDeltas :many
SELECT id, hp_delta
FROM action_log
WHERE character_id = ?
ORDER BY id;
