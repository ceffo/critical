-- The name Google gives a user (ADR 0029 step 3.8), refreshed at every login; shown instead of the
-- email, which stays the detail of the admin section.
--
-- Nullable, no backfill: a user has no name until their next login, and the app shows the email
-- meanwhile. A login whose token carries no name keeps the stored one.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): the release before this one never reads the
-- column, and its inserts and updates leave it as it is.
ALTER TABLE users ADD COLUMN display_name TEXT;
