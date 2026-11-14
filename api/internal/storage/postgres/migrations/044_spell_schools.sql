-- A spell's school codes (ADR 0050 step 1.4, catalogue format 23): the PHB schools of the spell as a JSON
-- list in spells.schools, written by the catalogue import and served with the spell. NULL is a row not
-- imported from a format-23 artifact yet (the format change makes the next start import every spell
-- again); "[]" is a spell of no PHB school. Additive (ADR 0024 step 5.2): an older release ignores the
-- column. Schema only.
ALTER TABLE spells ADD COLUMN schools TEXT;
COMMENT ON COLUMN spells.schools IS 'the spell''s school codes (ADR 0050): ["abjuration", "illusion", ...], [] for no PHB school, NULL: not imported from format 23 yet';
