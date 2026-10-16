-- The spell catalogue artifact (ADR 0021 step 5). spells keeps its integer
-- id; the artifact's stable id goes in key, NULL on rows the importer has
-- not keyed yet (backfilled on the first import; older duplicate rows keep
-- NULL for good and are never served). SQLite treats NULLs as distinct in a
-- unique index, so several unkeyed rows may coexist. French texts are
-- nullable: absent means the reader falls back to English. source is the
-- book code of the English text.
ALTER TABLE spells ADD COLUMN key TEXT;
ALTER TABLE spells ADD COLUMN name_fr TEXT;
ALTER TABLE spells ADD COLUMN description_fr TEXT;
ALTER TABLE spells ADD COLUMN source TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_spells_key ON spells(key);

-- The imported artifact's format and version (one row). Same version at
-- startup: the import does nothing.
CREATE TABLE IF NOT EXISTS catalogue_meta (
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    format  INTEGER NOT NULL,
    version TEXT    NOT NULL
);
