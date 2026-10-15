-- The XP ledger (ADR 0015). One row per award the player records: the base
-- amount, whether the +10 percent prime-requisite bonus applies, a free-text
-- reason and session label and the day it was recorded. The ledger sits
-- outside the action log, so rows are edited and deleted in place. The
-- starting XP stays on characters.xp_total and the ledger adds to it, so an
-- existing DB needs no data migration.
-- xp_events (from 002) was never written by any code: it is dropped in favour
-- of this table. (The migrator splits on semicolons: none in comments.)
CREATE TABLE IF NOT EXISTS xp_entries (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    entry_date   TEXT    NOT NULL,
    session      TEXT    NOT NULL DEFAULT '',
    reason       TEXT    NOT NULL DEFAULT '',
    base         INTEGER NOT NULL CHECK (base > 0),
    bonus        INTEGER NOT NULL DEFAULT 0 CHECK (bonus IN (0, 1))
);

CREATE INDEX IF NOT EXISTS xp_entries_character ON xp_entries (character_id, id);

DROP TABLE IF EXISTS xp_events;
