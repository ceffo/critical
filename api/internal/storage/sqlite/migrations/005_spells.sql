CREATE TABLE IF NOT EXISTS spells (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT    NOT NULL,
    class          TEXT    NOT NULL,
    level          INTEGER NOT NULL,
    school         TEXT,
    sphere         TEXT,
    casting_time   TEXT,
    range          TEXT,
    duration       TEXT,
    area_of_effect TEXT,
    components     TEXT,
    saving_throw   TEXT,
    description    TEXT
);

CREATE INDEX IF NOT EXISTS idx_spells_name ON spells(name);
