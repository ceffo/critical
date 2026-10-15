-- Skill checks (ADR 0014). The non-weapon proficiencies of the sheet, each
-- checked on a d20 against an ability score plus a modifier, and the bard
-- skills, each checked on a d100 against a percentage. Ids are stable keys
-- the UI names through i18n. Position keeps the order of the seed. Both
-- tables are seeded from seeds/skills.json when empty. A check is an entry
-- of the action log, so no roll is stored here.
CREATE TABLE IF NOT EXISTS proficiencies (
    id       TEXT    PRIMARY KEY,
    position INTEGER NOT NULL,
    ability  TEXT    NOT NULL CHECK (ability IN ('str', 'dex', 'con', 'int', 'wis', 'cha')),
    modifier INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS bard_skills (
    id       TEXT    PRIMARY KEY,
    position INTEGER NOT NULL,
    percent  INTEGER NOT NULL CHECK (percent BETWEEN 0 AND 100)
);
