-- An amend correction may carry a new payload (critical-qki.14): adjusting a
-- saving throw after the roll records the recomputed save as an amend whose
-- payload replaces the entry's (latest amend with a payload wins). Existing
-- rows keep NULL, so their entries read exactly as before.
ALTER TABLE action_corrections ADD COLUMN payload TEXT;
