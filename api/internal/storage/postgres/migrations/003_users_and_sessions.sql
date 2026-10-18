-- Users, invites and sessions (ADR 0029 steps 1, 2, 4 and 5).
--
-- Additive only (ADR 0024 step 5.2, ADR 0025 step 2.2): three new tables; the release before this
-- one never reads them. Schema only, no rows: the root admin is set by `just root-admin` (step
-- 5.1), or by the dev login when the server runs without OAuth settings (step 8.1).

-- A user: the email (unique, compared lower-cased: the server stores it lower-cased), the Google
-- subject once the first login bound it (step 3.3), the role, the root flag, the disabled flag and
-- the time of the last login. Nothing is hard-deleted (step 1.2).
CREATE TABLE users (
    id             UUID        PRIMARY KEY,
    email          TEXT        NOT NULL CHECK (email <> '' AND email = lower(email)),
    google_subject TEXT        CHECK (google_subject <> ''),
    role           TEXT        NOT NULL CHECK (role IN ('admin', 'player')),
    is_root        BOOLEAN     NOT NULL DEFAULT FALSE,
    disabled       BOOLEAN     NOT NULL DEFAULT FALSE,
    last_login     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL,
    -- Step 5.2: a root is an enabled admin, whatever a handler does.
    CONSTRAINT users_root_is_enabled_admin CHECK (NOT is_root OR (role = 'admin' AND NOT disabled))
);
CREATE UNIQUE INDEX users_email ON users (email);
CREATE UNIQUE INDEX users_google_subject ON users (google_subject) WHERE google_subject IS NOT NULL;
-- Step 5.2: at most one root admin.
CREATE UNIQUE INDEX users_one_root ON users (is_root) WHERE is_root;

-- An invite authorises an email to log in through Google (step 2.1). It is cancelled, never
-- deleted (step 2.3), and consumed by the first login of its email (step 2.4: accepted_by, accepted_at).
CREATE TABLE invites (
    id          UUID        PRIMARY KEY,
    email       TEXT        NOT NULL CHECK (email <> '' AND email = lower(email)),
    invited_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL,
    cancelled   TIMESTAMPTZ,
    accepted_by UUID        REFERENCES users(id),
    accepted_at TIMESTAMPTZ,
    CHECK ((accepted_by IS NULL) = (accepted_at IS NULL))
);
-- One pending invite per email.
CREATE UNIQUE INDEX invites_pending_email ON invites (email) WHERE cancelled IS NULL AND accepted_by IS NULL;

-- A server-side session (step 4.1). id is the SHA-256 of the cookie's random token (hex), so the
-- table never holds a usable token. last_used is the time of the last extension (step 4.3).
CREATE TABLE sessions (
    id         TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,
    last_used  TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_id ON sessions (user_id);
