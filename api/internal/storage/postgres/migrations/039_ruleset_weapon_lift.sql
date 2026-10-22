-- The ruleset's weapon lift (ADR 0041 step 2.5, ADR 0042 step 16.1): when true, a multi-class
-- character's blunt_only class entry no longer judges every weapon alone. False is the book's rule
-- and the default ruleset's value (ruleset.Default).
--
-- Additive (ADR 0024 step 5.2): existing rulesets take false, which is what they played under, and
-- the release before this one never reads it.
ALTER TABLE rulesets
    ADD COLUMN multiclass_any_weapon BOOLEAN NOT NULL DEFAULT FALSE;
