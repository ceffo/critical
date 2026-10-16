-- Character details the player edits in the app: race, alignment, age, sex,
-- height, weight, hair, eyes and languages. Text fields are free text and
-- start empty, age 0 means not set, languages is a JSON array of names as
-- entered. class is the class key shown read-only. The existing
-- character is a Bard, and the languages come from the player sheet. The
-- UPDATE only runs here, once, so later edits are never overwritten. A fresh
-- database gets the same values from seeds/character.json.
ALTER TABLE characters ADD COLUMN class TEXT NOT NULL DEFAULT 'bard';
ALTER TABLE characters ADD COLUMN race TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN alignment TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN age INTEGER NOT NULL DEFAULT 0;
ALTER TABLE characters ADD COLUMN sex TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN height TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN weight TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN hair TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN eyes TEXT NOT NULL DEFAULT '';
ALTER TABLE characters ADD COLUMN languages TEXT NOT NULL DEFAULT '[]';

UPDATE characters SET languages = '["Elfique","Commun","Orque","Nain"]' WHERE languages = '[]';
