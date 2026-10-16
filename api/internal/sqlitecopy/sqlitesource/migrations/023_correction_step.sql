-- A correction made after a roll (target AC, save adjustment, GM verdict) is
-- its own step on the action stack: a correction entry in action_log, and its
-- amend points at it through step_id. Popping that entry (a void on it) puts
-- the amend out of effect, so the corrected entry reads its previous payload
-- again. Rows written before keep NULL: they stay in effect until their entry
-- is popped, as before.
ALTER TABLE action_corrections ADD COLUMN step_id INTEGER REFERENCES action_log(id);
