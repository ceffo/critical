-- A scroll names the spell it holds (ADR 0033 step 2, ADR 0035 item 6.2): items.spell_id is a
-- spell catalogue id ('' = none), set on a magic scroll only. Using a named scroll corrects the
-- round's initiative with that spell's casting time.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a column with a default, which the release before
-- this one never reads nor writes (its item UPDATE names its columns), so a scroll it edits during
-- a rolling deploy keeps its spell. Schema only, no data step.
ALTER TABLE items ADD COLUMN spell_id TEXT NOT NULL DEFAULT '';
