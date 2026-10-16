-- Redo (ADR 0019 step 1): replaying a popped transaction un-pops its entries
-- with a new correction row of kind redo, and an entry popped again after a
-- redo gets a second void. An entry is voided when its newest void or redo
-- correction is a void. The kind CHECK gains redo and the one-void-per-entry
-- index goes. SQLite cannot change a CHECK in place, so the table is rebuilt
-- and renamed (no other table references action_corrections). Every row keeps
-- its id and columns.
CREATE TABLE action_corrections_v26 (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    action_id  INTEGER  NOT NULL REFERENCES action_log(id),
    kind       TEXT     NOT NULL CHECK (kind IN ('amend', 'void', 'redo')),
    hp_delta   INTEGER,
    summary    TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    payload    TEXT,
    note       TEXT,
    step_id    INTEGER REFERENCES action_log(id)
);

INSERT INTO action_corrections_v26 (id, action_id, kind, hp_delta, summary, created_at, payload, note, step_id)
SELECT id, action_id, kind, hp_delta, summary, created_at, payload, note, step_id
FROM action_corrections
ORDER BY id;

DROP TABLE action_corrections;

ALTER TABLE action_corrections_v26 RENAME TO action_corrections;

CREATE INDEX IF NOT EXISTS idx_action_corrections_action ON action_corrections(action_id, id);
