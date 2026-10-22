-- The powers a psionicist knows (ADR 0043 step 9): characters.psionic_picks is the whole list as
-- JSON, [{"kind": "discipline"|"power"|"relearn", "id": "..."}], in the player's order (the first
-- discipline is the primary one), edited as one list like the proficiency picks (ADR 0037 step 12).
-- '[]' for a character with no pick, whether or not it has the psionics feature.
--
-- Additive with a default (ADR 0024 step 5.2, ADR 0025 step 2.2): every existing row reads no pick
-- and the release before this one never reads the column.
ALTER TABLE characters ADD COLUMN psionic_picks TEXT NOT NULL DEFAULT '[]';
