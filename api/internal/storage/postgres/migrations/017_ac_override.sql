-- The character's AC override (ADR 0033 step 13): the AC the player sets for what the derived AC
-- cannot hold (the Shield spell), a number -10 to 10, and the optional label the player typed. NULL
-- is no override. The AC itself is derived on read, never stored (step 10).
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): two columns with a default the release before
-- this one never reads. Schema only, no data step: no character has an override yet.
ALTER TABLE characters ADD COLUMN ac_override INTEGER CHECK (ac_override BETWEEN -10 AND 10);
ALTER TABLE characters ADD COLUMN ac_override_label TEXT NOT NULL DEFAULT '';
