-- Each character owns its weapons, items, nonweapon proficiencies, bard skills and spellbook.
--
-- Additive only (ADR 0024 step 5.2, ADR 0025 step 2.2): the release before this one keeps running
-- on this schema during a rolling deploy, so nothing it reads or writes goes away here.
--   - character_id is added nullable: the previous release inserts rows without it. The boot's
--     seed step gives every row without an owner to the character named in the seeds'
--     character.json (no data here, ADR 0025 step 2.1), the rows the previous release wrote
--     during the deploy included, on the next boot.
--   - The single-column primary keys stay: the previous release's seed upserts conflict on them.
--     A later migration, once no release without character_id runs, makes character_id NOT NULL
--     and the primary keys (character_id, id); until then an id is unique across characters in
--     the database (the in-memory store already keys them per character).
--   - The (character_id, id) unique indexes are what this release looks rows up and upserts by.
ALTER TABLE weapons       ADD COLUMN character_id BIGINT REFERENCES characters(id);
ALTER TABLE items         ADD COLUMN character_id BIGINT REFERENCES characters(id);
ALTER TABLE proficiencies ADD COLUMN character_id BIGINT REFERENCES characters(id);
ALTER TABLE bard_skills   ADD COLUMN character_id BIGINT REFERENCES characters(id);

CREATE UNIQUE INDEX weapons_character_id_id       ON weapons (character_id, id);
CREATE UNIQUE INDEX items_character_id_id         ON items (character_id, id);
CREATE UNIQUE INDEX proficiencies_character_id_id ON proficiencies (character_id, id);
CREATE UNIQUE INDEX bard_skills_character_id_id   ON bard_skills (character_id, id);

-- The spells a character knows, by catalogue key (spells.key), in the order the sheet lists them.
-- No foreign key to spells: a fresh database is seeded before the catalogue is imported. The
-- entries are checked against the catalogue when they are written, and the catalogue import keeps
-- every spell an entry refers to (ADR 0021 step 5.4.3, game state).
CREATE TABLE spellbook_entries (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    spell_key    TEXT   NOT NULL,
    position     BIGINT NOT NULL,
    PRIMARY KEY (character_id, spell_key)
);
