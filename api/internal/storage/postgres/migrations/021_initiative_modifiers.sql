-- The round's initiative carries its modifiers (ADR 0035 item 6, amending ADR 0013 step 3): the
-- weapon in hand's speed, the initiative effects, or the spell cast or item used that corrected
-- the round. A JSON list of terms ({source, value, itemId, name, spellId, magicKind, timing}),
-- written by buffsvc's DAL; NULL is none (a plain d10, as every initiative stored before this file).
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a nullable column the release before this one
-- never reads; what it writes leaves the column NULL, read as a plain d10. Schema only, no data step.
ALTER TABLE round_state ADD COLUMN initiative_modifiers TEXT;
