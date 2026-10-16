-- name: GetText :many
SELECT lang, body FROM localized_texts WHERE text_id = $1;

-- name: NextTextKey :one
SELECT (COALESCE(MAX(text_id), 0) + 1)::bigint AS next_key FROM localized_texts;

-- name: DeleteText :exec
DELETE FROM localized_texts WHERE text_id = $1;

-- name: InsertText :exec
INSERT INTO localized_texts (text_id, lang, body) VALUES ($1, $2, $3);
