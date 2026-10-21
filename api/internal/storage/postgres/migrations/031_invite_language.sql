-- The language an invite's mails are written in (ADR 0029 step 2.6): the admin picks it per invite.
--
-- One column holding a language code ('en' or 'fr'), not a column per language (ADR 0023): the
-- code names which template the mail adapter renders. Existing invites default to 'en'.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): the release before this one never reads the
-- column, and its inserts leave the default.
ALTER TABLE invites ADD COLUMN language TEXT NOT NULL DEFAULT 'en' CHECK (language IN ('en', 'fr'));
