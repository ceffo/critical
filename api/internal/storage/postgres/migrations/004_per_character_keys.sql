-- Weapons, items, nonweapon proficiencies and bard skills are keyed per character: character_id is
-- required and the primary keys are (character_id, id), so two characters may hold the same id.
-- The contract step of migration 002 (docs/backend.md, "Per-character data").
--
-- Every row has an owner when this runs: the migration run gives the rows without one to the seed
-- character just before it, in the same transaction (storage/postgres ownerKeysMigration; a data
-- step in Go, since migrations hold no data, ADR 0025 step 2.1.1). That covers a database that gets
-- 002 and this file in one run (one upgraded from a release before 002).
--
-- Across the rolling deploy (ADR 0024 step 5.2) the release before this one keeps working when it
-- writes character_id (every release with migration 002): its upserts conflict on
-- (character_id, id), which the new primary key is. A release from before 002 would fail its writes
-- of new weapons and items (no character_id) for the minutes both run; reads, counts and deletes
-- still work.
--
-- The unique indexes of 002 become the primary keys (no index is rebuilt; each takes the name of
-- the constraint, <table>_pkey).
ALTER TABLE weapons       ALTER COLUMN character_id SET NOT NULL;
ALTER TABLE items         ALTER COLUMN character_id SET NOT NULL;
ALTER TABLE proficiencies ALTER COLUMN character_id SET NOT NULL;
ALTER TABLE bard_skills   ALTER COLUMN character_id SET NOT NULL;

ALTER TABLE weapons       DROP CONSTRAINT weapons_pkey;
ALTER TABLE items         DROP CONSTRAINT items_pkey;
ALTER TABLE proficiencies DROP CONSTRAINT proficiencies_pkey;
ALTER TABLE bard_skills   DROP CONSTRAINT bard_skills_pkey;

ALTER TABLE weapons       ADD CONSTRAINT weapons_pkey       PRIMARY KEY USING INDEX weapons_character_id_id;
ALTER TABLE items         ADD CONSTRAINT items_pkey         PRIMARY KEY USING INDEX items_character_id_id;
ALTER TABLE proficiencies ADD CONSTRAINT proficiencies_pkey PRIMARY KEY USING INDEX proficiencies_character_id_id;
ALTER TABLE bard_skills   ADD CONSTRAINT bard_skills_pkey   PRIMARY KEY USING INDEX bard_skills_character_id_id;
