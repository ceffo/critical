-- The French texts of the six spell stats (catalogue format 2, ADR 0021).
-- Nullable: absent means the French book has no usable text for that stat
-- and the reader falls back to the English column, per field. The typed
-- stats keep being parsed from the English texts only.
ALTER TABLE spells ADD COLUMN casting_time_fr TEXT;
ALTER TABLE spells ADD COLUMN range_fr TEXT;
ALTER TABLE spells ADD COLUMN duration_fr TEXT;
ALTER TABLE spells ADD COLUMN area_of_effect_fr TEXT;
ALTER TABLE spells ADD COLUMN components_fr TEXT;
ALTER TABLE spells ADD COLUMN saving_throw_fr TEXT;
