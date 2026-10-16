CREATE TABLE IF NOT EXISTS hp_events (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    delta        INTEGER NOT NULL,
    source       TEXT    NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS xp_events (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    delta        INTEGER NOT NULL,
    source       TEXT    NOT NULL,
    session_date DATE    NOT NULL DEFAULT (date('now'))
);
