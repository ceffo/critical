-- One append-only action log for every table action (HP change, cast, buff
-- change, round advance, attack, item use). Rows are never updated: amend and
-- void are new rows in action_corrections pointing at the original entry.
-- Current HP is the SUM of effective hp_delta over action_log (latest amend
-- wins, voided entries count 0).
CREATE TABLE IF NOT EXISTS action_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER  NOT NULL REFERENCES characters(id),
    round        INTEGER  NOT NULL DEFAULT 0,
    kind         TEXT     NOT NULL,
    summary      TEXT     NOT NULL,
    hp_delta     INTEGER  NOT NULL DEFAULT 0,
    payload      TEXT     NOT NULL DEFAULT '{}',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_action_log_character ON action_log(character_id, id);

-- Before/after state captured by an entry, one row per state key (for
-- example buffs, spell_slots, round). Voiding restores before_state.
CREATE TABLE IF NOT EXISTS action_snapshots (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    action_id    INTEGER NOT NULL REFERENCES action_log(id),
    state_key    TEXT    NOT NULL,
    before_state TEXT    NOT NULL,
    after_state  TEXT    NOT NULL,
    UNIQUE (action_id, state_key)
);

CREATE INDEX IF NOT EXISTS idx_action_snapshots_key ON action_snapshots(state_key, action_id);

-- Audited corrections. amend carries the new hp_delta and summary, void
-- carries nothing. At most one void per entry.
CREATE TABLE IF NOT EXISTS action_corrections (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    action_id  INTEGER  NOT NULL REFERENCES action_log(id),
    kind       TEXT     NOT NULL CHECK (kind IN ('amend', 'void')),
    hp_delta   INTEGER,
    summary    TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_action_corrections_action ON action_corrections(action_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_action_corrections_one_void ON action_corrections(action_id) WHERE kind = 'void';

-- Carry the old hp_events ledger over as hp_change entries, in id order, so
-- totals are unchanged (undo rows were negations and are kept as they are).
-- Guarded so it only runs into an empty log. hp_events stays as a frozen
-- archive and is no longer written.
INSERT INTO action_log (character_id, round, kind, summary, hp_delta, payload, created_at)
SELECT character_id, 0, 'hp_change', source, delta, '{}', created_at
FROM hp_events
WHERE NOT EXISTS (SELECT 1 FROM action_log)
ORDER BY id;
