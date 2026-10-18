-- The level a character's max HP was last set for (ADR 0030 step 7): when the level its XP gives
-- is above it, a level-up is pending and the player rolls its hit points; each roll raises hp_max
-- by the gain and hp_level by 1, in one action-log transaction whose snapshot the undo writes back.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a nullable column the release before this one
-- never reads. Schema only (step 2.1), no data step: NULL is "the level the sheet's max HP was
-- entered for", the character's stored `level` (the imported sheet's level, or for a character
-- created in the app the level its starting XP gives: step 7.5), which every read falls back to.
ALTER TABLE characters ADD COLUMN hp_level BIGINT;
