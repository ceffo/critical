-- name: UserByID :one
SELECT id, email, google_subject, role, is_root, disabled, last_login, created_at FROM users WHERE id = $1;

-- name: UserBySubject :one
SELECT id, email, google_subject, role, is_root, disabled, last_login, created_at FROM users WHERE google_subject = $1;

-- name: UserByEmail :one
SELECT id, email, google_subject, role, is_root, disabled, last_login, created_at FROM users WHERE email = $1;

-- name: RootUser :one
SELECT id, email, google_subject, role, is_root, disabled, last_login, created_at FROM users WHERE is_root;

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
