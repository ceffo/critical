-- A class entry's cut-off in the XP ledger (ADR 0042 steps 1 and 5): the id of the last ledger
-- entry the class entry ignores. A class entry earns its share of the awards with a greater id
-- only; 0 ignores none, which is every stored entry. A character that takes a new class (dual-class)
-- gets the new entry's cut-off at the ledger's last id, so the new class starts at 0 XP without a
-- negative starting_xp, and a later edit or undo of an older award leaves it alone.
--
-- Additive (ADR 0024 step 5.2): existing rows take 0, and the release before this one never reads it.
ALTER TABLE character_classes
    ADD COLUMN ignored_awards_through BIGINT NOT NULL DEFAULT 0 CHECK (ignored_awards_through >= 0);
