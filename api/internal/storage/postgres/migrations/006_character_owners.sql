-- Characters belong to users (ADR 0029 step 7): each character's owner, a retired flag, and each
-- user's current character, the one every per-character request acts on.
--
-- Additive only (ADR 0024 step 5.2, ADR 0025 step 2.2): three nullable or defaulted columns the
-- release before this one never reads, so it keeps running on this schema during a rolling deploy
-- (it plays the first character, whoever owns it). Schema only, no rows (step 2.1): the characters
-- without an owner (the seed character, Kelsen on the hosted apps) go to the root admin, and become
-- the root admin's current character, at every start once a root admin exists, and when the
-- root-admin script or the dev login makes one (authsvc AdoptUnownedCharacters).
--
-- owner_user_id stays nullable: a fresh database seeds its character before any user exists.
-- A character without an owner is nobody's: no request reaches it.
ALTER TABLE characters ADD COLUMN owner_user_id UUID REFERENCES users(id);
-- Retiring hides a character from its owner's lists; nothing is deleted (step 7.3).
ALTER TABLE characters ADD COLUMN retired BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX characters_owner_user_id ON characters (owner_user_id);

-- The user's current character (step 7.2), kept on the server so it follows the user across
-- devices. NULL: the user has no character yet (or none set). The server only sets it to a
-- character the user owns, and reads it through that ownership (a character that changed hands
-- or was retired is no current character).
ALTER TABLE users ADD COLUMN current_character_id BIGINT REFERENCES characters(id);
