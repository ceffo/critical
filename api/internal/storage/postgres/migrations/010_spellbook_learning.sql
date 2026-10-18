-- Learning a spell (ADR 0030 step 20).
--
-- spellbook_learn_failures holds the spells a character failed to learn: the chance-to-learn roll
-- (PHB Table 4) came out above the Intelligence's chance, at the realized level stored here. The
-- spell is refused to that character until its realized level is higher than this one (step
-- 20.2). Undoing the failure's journal entry deletes the row. A spell learned later replaces the
-- block (the row is deleted with the spellbook entry's insert).
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): a new table the release before this one never
-- reads. Schema only, no data step: no stored character has a failure, and the spellbooks already
-- stored (the seed character's) stay as they are, with no learn roll (step 20.4).
CREATE TABLE spellbook_learn_failures (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    spell_key    TEXT   NOT NULL,
    level        BIGINT NOT NULL CHECK (level >= 1),
    PRIMARY KEY (character_id, spell_key)
);
