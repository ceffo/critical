-- Each spellbook entry stores its origin (ADR 0030 step 20.5).
--
-- spellbook_entries.origin is how the entry came in: 'learned' (a Learn roll that succeeded,
-- step 20.2), 'given' (the roll-free add, step 20.6), 'starting' (the creation's book, step
-- 20.7) or 'level_grant' (a level's free spell, step 20.8); grant_level is the realized level
-- that granted a 'level_grant' entry, 0 for the others. The origin never changes a rule of the
-- book; the application writes both columns on every insert.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): two columns with defaults the release before
-- this one never reads; its inserts in the minutes both run (a spell learned then) get the
-- default 'starting'. Schema only (step 2.1): a data step run just after this file marks
-- 'learned' every entry a successful spell_learn journal entry wrote (markLearnedSpells); every
-- other stored entry (Kelsen's imported book) stays 'starting' (step 20.5). No entry is added or
-- removed (step 20.9).
ALTER TABLE spellbook_entries
    ADD COLUMN origin TEXT NOT NULL DEFAULT 'starting'
        CHECK (origin IN ('learned', 'given', 'starting', 'level_grant')),
    ADD COLUMN grant_level BIGINT NOT NULL DEFAULT 0
        CHECK (grant_level >= 0);
