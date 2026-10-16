-- name: GetText :many
SELECT lang, body FROM localized_texts WHERE text_id = ?;

-- name: NextTextKey :one
SELECT CAST(COALESCE(MAX(text_id), 0) + 1 AS INTEGER) AS next_key FROM localized_texts;

-- name: DeleteText :exec
DELETE FROM localized_texts WHERE text_id = ?;

-- name: InsertText :exec
INSERT INTO localized_texts (text_id, lang, body) VALUES (?, ?, ?);
