-- The PSP pool of a character with the psionics feature (ADR 0043 steps 4 and 5), modelled
-- like HP (ADR 0007 step 2, ADR 0018 step 4):
--
-- * characters.psp_max is the stored maximum, as hp_max: set at creation (computed, or entered
--   from the sheet) and grown by a psionicist level-up, never recomputed. 0 for a character
--   with no psionics feature, which never reads it.
-- * action_log.psp_delta is any entry's signed contribution to current PSPs; current PSPs are
--   psp_max plus the sum of the effective deltas (actionlog.PSPTotal). Entries stay append-only:
--   a popped entry counts 0, worked out in Go from its corrections.
--
-- Additive with defaults (ADR 0024 step 5.2, ADR 0025 step 2.2): every existing row reads 0, so
-- an existing character has no PSPs and the release before this one keeps writing rows that
-- read as no PSP change.
ALTER TABLE characters ADD COLUMN psp_max BIGINT NOT NULL DEFAULT 0 CHECK (psp_max >= 0);

ALTER TABLE action_log ADD COLUMN psp_delta BIGINT NOT NULL DEFAULT 0;
