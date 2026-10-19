-- InsertAction appends an entry. The log is append-only (ADR 0007, 0010): an
-- entry is never updated but by StartActionTransaction on the row just
-- inserted; an amend, a void (pop) or a redo is a new action_corrections row.

-- name: InsertAction :one
INSERT INTO action_log (character_id, round, kind, summary, hp_delta, payload, transaction_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: StartActionTransaction :exec
UPDATE action_log SET transaction_id = id
WHERE id = $1;

-- name: InsertSnapshot :exec
INSERT INTO action_snapshots (action_id, state_key, before_envelope, after_envelope)
VALUES (@action_id, @state_key, sqlc.arg(before_envelope)::text, sqlc.arg(after_envelope)::text);

-- name: InsertCorrection :exec
INSERT INTO action_corrections (action_id, kind, hp_delta, summary, payload, note, step_id)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetAction :one
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = $1 AND id = $2;

-- name: ListActions :many
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = sqlc.arg(character_id)
ORDER BY id DESC
LIMIT sqlc.arg('limit')::bigint;

-- Which entries are popped is decided in Go (actionlog.StandingVoids, ADR
-- 0019 step 1) from ListPopCorrections; the queries below read the newest
-- rows, as many as the adapter needs to skip the popped ones.

-- name: ListActionsByKind :many
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = sqlc.arg(character_id) AND kind = sqlc.arg(kind)
ORDER BY id DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListActionIDs :many
SELECT id
FROM action_log
WHERE character_id = sqlc.arg(character_id)
ORDER BY id DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListPopCorrections :many
SELECT action_corrections.id, action_corrections.action_id, action_corrections.kind
FROM action_corrections
JOIN action_log ON action_log.id = action_corrections.action_id
WHERE action_log.character_id = $1 AND action_corrections.kind IN ('void', 'redo')
ORDER BY action_corrections.id;

-- name: ListCorrectionsFrom :many
SELECT action_corrections.id, action_corrections.action_id, action_corrections.kind,
       action_corrections.hp_delta, action_corrections.summary, action_corrections.payload,
       action_corrections.note, action_corrections.step_id, action_corrections.created_at
FROM action_corrections
JOIN action_log ON action_log.id = action_corrections.action_id
WHERE action_log.character_id = $1 AND action_corrections.action_id >= $2
ORDER BY action_corrections.id;

-- name: ListSnapshotsFrom :many
SELECT action_snapshots.id, action_snapshots.action_id, action_snapshots.state_key,
       action_snapshots.before_envelope, action_snapshots.after_envelope
FROM action_snapshots
JOIN action_log ON action_log.id = action_snapshots.action_id
WHERE action_log.character_id = $1 AND action_snapshots.action_id >= $2
ORDER BY action_snapshots.id;

-- name: LatestActionID :one
SELECT id
FROM action_log
WHERE character_id = $1 AND kind = $2
ORDER BY id DESC
LIMIT 1;

-- name: ListTransactionActions :many
SELECT id, character_id, round, kind, summary, hp_delta, payload, transaction_id, created_at
FROM action_log
WHERE character_id = $1 AND transaction_id = $2
ORDER BY id DESC;

-- ListHPDeltas reads every entry's own HP delta; the adapter adds the
-- corrections and sums the effective deltas in Go (actionlog.HPTotal, ADR
-- 0007 step 2).

-- name: ListHPDeltas :many
SELECT id, hp_delta
FROM action_log
WHERE character_id = $1
ORDER BY id;
