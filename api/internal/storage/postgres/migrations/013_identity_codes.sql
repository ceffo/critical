-- A character's alignment and sex are codes (ADR 0032 steps 2, 3 and 8): characters.alignment is
-- one of the nine PHB alignments (lawful_good ... chaotic_evil), characters.sex is male or female,
-- '' is a character that lacks it (the card then asks the player to pick). The columns stay TEXT.
-- characters.race, the free-text race detail, is no longer read or written (the rules race is
-- race_code, ADR 0032 step 1) and its data is discarded.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): no column is added, changed or dropped, so the
-- release before this one keeps working on the schema while the new one is deployed (it reads and
-- writes the free text of the same columns, and the race detail it still writes is ignored). The
-- column characters.race is dropped by a later migration, once no running release reads it (the
-- next release, never this one). Schema only (step 2.1): the stored free-text values are mapped to
-- codes, the unrecognised ones cleared and the race detail emptied by a data step run just after
-- this file (mapIdentity), and the seed character already stored gets the alignment and sex its
-- seed names (identitySeedCharacter).
COMMENT ON COLUMN characters.alignment IS 'alignment code (lawful_good ... chaotic_evil), empty when missing';
COMMENT ON COLUMN characters.sex IS 'sex code (male, female), empty when missing';
COMMENT ON COLUMN characters.race IS 'unused since migration 013: the free-text race detail is discarded; race_code is the race';
