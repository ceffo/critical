-- A character's class entries (ADR 0042 step 1): one row per class the character has, in the
-- order the classes were taken. An entry is active (it earns XP and levels) or frozen (a
-- dual-class's old class, with the level it was left at). starting_xp is the entry's XP as
-- entered; its share of the XP ledger is derived on read, never stored (step 5).
--
-- This file is the schema only (ADR 0025 step 2.1). Its data step, run right after it
-- (fillClassEntries), gives every stored character one active entry from its class, its realized
-- level (COALESCE(realized_level, level), ADR 0030 step 7.5) and its xp_total.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): characters.class, level, xp_total and
-- realized_level stay, written by this release for the character's first active entry, so the
-- release before this one keeps reading its characters during a rolling deploy; the contract
-- step that drops them follows once that release is gone. A character the release before this one
-- creates meanwhile has no entry: the reader gives it the one its columns state. Only the default
-- of class goes: a character always states its class (it was 'bard' for the first sheet).
CREATE TABLE character_classes (
    character_id   BIGINT NOT NULL REFERENCES characters(id),
    position       BIGINT NOT NULL CHECK (position >= 0),
    class          TEXT   NOT NULL,
    realized_level BIGINT NOT NULL CHECK (realized_level >= 0),
    starting_xp    BIGINT NOT NULL DEFAULT 0 CHECK (starting_xp >= 0),
    state          TEXT   NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'frozen')),
    left_at_level  BIGINT NOT NULL DEFAULT 0 CHECK (left_at_level >= 0),
    PRIMARY KEY (character_id, position),
    UNIQUE (character_id, class),
    CHECK ((state = 'frozen' AND left_at_level >= 2) OR (state = 'active' AND left_at_level = 0))
);

ALTER TABLE characters ALTER COLUMN class DROP DEFAULT;
