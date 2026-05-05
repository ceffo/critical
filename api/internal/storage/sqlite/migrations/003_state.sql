CREATE TABLE IF NOT EXISTS active_buffs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    buffs        TEXT    NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS spell_slots (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    slots        TEXT    NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS round_state (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    round_number INTEGER NOT NULL DEFAULT 0
);
