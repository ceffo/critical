-- Localized text (ADR 0023): a text in a human language is a key into
-- localized_texts, one row per (text_id, lang); English is the 'en' row like
-- any other language. The spells table's ten text columns (name,
-- description, school, sphere and the six stat texts) now hold that key
-- instead of the text, and name_fr / description_fr are gone.
--
-- A text column is NULL or holds a key with at least one row: a text with no
-- language has no key (an absent sphere), and an absent language has no row,
-- never an empty body. Keys are allocated by the importer (one more than the
-- highest); text_id is not unique on its own, so the columns carry no
-- FOREIGN KEY clause. The rows are found through the primary key
-- (text_id, lang) by key, so reading the names never scans the descriptions.
CREATE TABLE IF NOT EXISTS localized_texts (
    text_id INTEGER NOT NULL,
    lang    TEXT    NOT NULL,
    body    TEXT    NOT NULL CHECK (body <> ''),
    PRIMARY KEY (text_id, lang)
) WITHOUT ROWID;

-- Every existing row's texts move, keyed (id - 1) * 10 + the field's rank,
-- so keys are unique and distinct per field. Rows without a key (older
-- duplicates the import never serves) move too: the import still matches
-- them by English name. An empty text gets no row and no key.
INSERT INTO localized_texts (text_id, lang, body)
SELECT (id - 1) * 10 + 1, 'en', name FROM spells WHERE name <> ''
UNION ALL SELECT (id - 1) * 10 + 1, 'fr', name_fr FROM spells WHERE name <> '' AND name_fr <> ''
UNION ALL SELECT (id - 1) * 10 + 2, 'en', description FROM spells WHERE description <> ''
UNION ALL SELECT (id - 1) * 10 + 2, 'fr', description_fr FROM spells WHERE description <> '' AND description_fr <> ''
UNION ALL SELECT (id - 1) * 10 + 3, 'en', school FROM spells WHERE school <> ''
UNION ALL SELECT (id - 1) * 10 + 4, 'en', sphere FROM spells WHERE sphere <> ''
UNION ALL SELECT (id - 1) * 10 + 5, 'en', casting_time FROM spells WHERE casting_time <> ''
UNION ALL SELECT (id - 1) * 10 + 6, 'en', range FROM spells WHERE range <> ''
UNION ALL SELECT (id - 1) * 10 + 7, 'en', duration FROM spells WHERE duration <> ''
UNION ALL SELECT (id - 1) * 10 + 8, 'en', area_of_effect FROM spells WHERE area_of_effect <> ''
UNION ALL SELECT (id - 1) * 10 + 9, 'en', components FROM spells WHERE components <> ''
UNION ALL SELECT (id - 1) * 10 + 10, 'en', saving_throw FROM spells WHERE saving_throw <> '';

-- A French text whose English one is empty has no key to hang on: none
-- exists in any catalogue (English is required) and none is kept.

-- SQLite cannot change a column's type: the table is rebuilt with every row
-- under its id (game state refers to spells by key, which is kept as is).
CREATE TABLE spells_new (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    key            TEXT,
    name           INTEGER,
    class          TEXT    NOT NULL,
    level          INTEGER NOT NULL,
    school         INTEGER,
    sphere         INTEGER,
    casting_time   INTEGER,
    range          INTEGER,
    duration       INTEGER,
    area_of_effect INTEGER,
    components     INTEGER,
    saving_throw   INTEGER,
    description    INTEGER,
    source         TEXT,
    stats          TEXT
);

INSERT INTO spells_new (id, key, name, class, level, school, sphere, casting_time, range, duration,
    area_of_effect, components, saving_throw, description, source, stats)
SELECT id, key,
    CASE WHEN name <> '' THEN (id - 1) * 10 + 1 END,
    class, level,
    CASE WHEN school <> '' THEN (id - 1) * 10 + 3 END,
    CASE WHEN sphere <> '' THEN (id - 1) * 10 + 4 END,
    CASE WHEN casting_time <> '' THEN (id - 1) * 10 + 5 END,
    CASE WHEN range <> '' THEN (id - 1) * 10 + 6 END,
    CASE WHEN duration <> '' THEN (id - 1) * 10 + 7 END,
    CASE WHEN area_of_effect <> '' THEN (id - 1) * 10 + 8 END,
    CASE WHEN components <> '' THEN (id - 1) * 10 + 9 END,
    CASE WHEN saving_throw <> '' THEN (id - 1) * 10 + 10 END,
    CASE WHEN description <> '' THEN (id - 1) * 10 + 2 END,
    source, stats
FROM spells;

-- Keep the id sequence where it was (ids of deleted spells are not reused).
UPDATE sqlite_sequence SET seq = (SELECT seq FROM sqlite_sequence WHERE name = 'spells')
WHERE name = 'spells_new' AND EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = 'spells');

DROP INDEX IF EXISTS idx_spells_name;
DROP INDEX IF EXISTS idx_spells_key;
DROP TABLE spells;
ALTER TABLE spells_new RENAME TO spells;
CREATE UNIQUE INDEX IF NOT EXISTS idx_spells_key ON spells(key);

-- Forgetting the recorded artifact makes the next startup import it again
-- into the new shape (as 030 did); the import keeps every key and row.
DELETE FROM catalogue_meta;
