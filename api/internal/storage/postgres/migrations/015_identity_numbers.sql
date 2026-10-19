-- A character's height and weight are numbers (ADR 0032 steps 5 and 8): characters.height_cm
-- (centimetres) and characters.weight_kg (kilograms), 0 when not set, replace the free-text
-- characters.height and characters.weight, which are no longer read or written. The age column
-- already is a number.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): two columns are added with a default and none is
-- changed or dropped, so the release before this one keeps working on the schema while the new one is
-- deployed (it reads and writes the free text of the old columns, which nobody reads any more). The
-- old columns are dropped by a later migration, once no running release reads them (the next release,
-- never this one). Schema only (step 2.1): the stored free text is parsed into the new columns, the
-- unrecognised ones left unset, by a data step run just after this file (mapMeasures).
ALTER TABLE characters ADD COLUMN height_cm DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE characters ADD COLUMN weight_kg DOUBLE PRECISION NOT NULL DEFAULT 0;
COMMENT ON COLUMN characters.height_cm IS 'height in centimetres, 0 when not set';
COMMENT ON COLUMN characters.weight_kg IS 'weight in kilograms, 0 when not set';
COMMENT ON COLUMN characters.height IS 'unused since migration 015: the free-text height is parsed into height_cm';
COMMENT ON COLUMN characters.weight IS 'unused since migration 015: the free-text weight is parsed into weight_kg';
