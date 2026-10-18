-- Every user read carries the user's current character (ADR 0029 step 7.2) through its ownership:
-- a character the user does not own, or retired, is no current character (NULL).

-- name: UserByID :one
SELECT u.id, u.email, u.google_subject, u.role, u.is_root, u.disabled, u.last_login, u.created_at, c.id AS current_character_id
FROM users u
LEFT JOIN characters c ON c.id = u.current_character_id AND c.owner_user_id = u.id AND NOT c.retired
WHERE u.id = $1;

-- name: UserBySubject :one
SELECT u.id, u.email, u.google_subject, u.role, u.is_root, u.disabled, u.last_login, u.created_at, c.id AS current_character_id
FROM users u
LEFT JOIN characters c ON c.id = u.current_character_id AND c.owner_user_id = u.id AND NOT c.retired
WHERE u.google_subject = $1;

-- name: UserByEmail :one
SELECT u.id, u.email, u.google_subject, u.role, u.is_root, u.disabled, u.last_login, u.created_at, c.id AS current_character_id
FROM users u
LEFT JOIN characters c ON c.id = u.current_character_id AND c.owner_user_id = u.id AND NOT c.retired
WHERE u.email = $1;

-- name: RootUser :one
SELECT u.id, u.email, u.google_subject, u.role, u.is_root, u.disabled, u.last_login, u.created_at, c.id AS current_character_id
FROM users u
LEFT JOIN characters c ON c.id = u.current_character_id AND c.owner_user_id = u.id AND NOT c.retired
WHERE u.is_root;

-- name: InsertUser :exec
INSERT INTO users (id, email, google_subject, role, is_root, disabled, last_login, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateUser :execrows
UPDATE users SET google_subject = $2, role = $3, is_root = $4, disabled = $5, last_login = $6 WHERE id = $1;

-- name: PendingInvite :one
SELECT id, email, invited_by, created_at FROM invites
WHERE email = $1 AND cancelled IS NULL AND accepted_by IS NULL;

-- name: AcceptInvite :execrows
UPDATE invites SET accepted_by = $2, accepted_at = $3 WHERE id = $1 AND cancelled IS NULL AND accepted_by IS NULL;

-- name: InsertSession :exec
INSERT INTO sessions (id, user_id, created_at, last_used, expires_at) VALUES ($1, $2, $3, $4, $5);

-- name: SessionByID :one
SELECT id, user_id, created_at, last_used, expires_at FROM sessions WHERE id = $1;

-- name: ExtendSession :exec
UPDATE sessions SET last_used = $2, expires_at = $3 WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: AdoptUnownedCharacters :exec
UPDATE characters SET owner_user_id = $1 WHERE owner_user_id IS NULL;

-- name: SetFirstCurrentCharacter :exec
-- The user's first character they own and have not retired becomes current, unless the current one
-- is such a character: a NULL current, a retired one and another user's are all repaired.
UPDATE users SET current_character_id = (
    SELECT min(c.id) FROM characters c WHERE c.owner_user_id = users.id AND NOT c.retired
) WHERE users.id = $1 AND NOT EXISTS (
    SELECT 1 FROM characters c
    WHERE c.id = users.current_character_id AND c.owner_user_id = users.id AND NOT c.retired
);

-- name: MoveCharacters :exec
-- Every character of one user becomes another's (a root admin that never logged in moves).
UPDATE characters SET owner_user_id = sqlc.arg(to_user) WHERE owner_user_id = sqlc.arg(from_user);

-- name: ClearCurrentCharacter :exec
UPDATE users SET current_character_id = NULL WHERE id = $1;

-- The admin section (ADR 0029 step 1.3): every user, the names of their characters (not the sheets),
-- and the pending invites.

-- name: ListUsers :many
SELECT id, email, google_subject, role, is_root, disabled, last_login, created_at FROM users
ORDER BY created_at, id;

-- name: OwnedCharacterNames :many
-- The names of every owned character not retired, by owner, in creation order.
SELECT owner_user_id, name FROM characters
WHERE owner_user_id IS NOT NULL AND NOT retired
ORDER BY id;

-- name: ListPendingInvites :many
SELECT id, email, invited_by, created_at FROM invites
WHERE cancelled IS NULL AND accepted_by IS NULL
ORDER BY created_at, id;

-- name: InsertInvite :exec
INSERT INTO invites (id, email, invited_by, created_at) VALUES ($1, $2, $3, $4);

-- name: CancelInvite :execrows
-- Only a pending invite is cancelled (step 2.3): a used or cancelled one stays as it is.
UPDATE invites SET cancelled = $2 WHERE id = $1 AND cancelled IS NULL AND accepted_by IS NULL;
