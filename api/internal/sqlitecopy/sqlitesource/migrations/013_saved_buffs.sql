-- Saved buff templates: a buff the player re-adds with one
-- click. Nothing is seeded, the list starts empty. New table only, so an
-- existing DB needs no data migration. The buff note needs no schema change:
-- active buffs are a JSON list, and a missing note reads as empty.
-- (The migrator splits on semicolons: none in comments.)
CREATE TABLE IF NOT EXISTS saved_buffs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER  NOT NULL REFERENCES characters(id),
    name         TEXT     NOT NULL,
    duration     INTEGER  NOT NULL DEFAULT 0 CHECK (duration >= 0),
    charges      INTEGER  NOT NULL DEFAULT 0 CHECK (charges >= 0),
    effects      TEXT     NOT NULL DEFAULT '{}',
    note         TEXT     NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_saved_buffs_character ON saved_buffs(character_id, id);
