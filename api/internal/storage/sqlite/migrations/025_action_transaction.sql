-- Every entry belongs to a transaction (ADR 0018 step 1): everything one
-- state-changing request writes to the log, in its one DB transaction. The
-- transaction id is the id of its first entry. Undo pops a transaction whole
-- (step 2). An entry written before this migration is its own transaction.
ALTER TABLE action_log ADD COLUMN transaction_id INTEGER REFERENCES action_log(id);

UPDATE action_log SET transaction_id = id WHERE transaction_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_action_log_transaction ON action_log(character_id, transaction_id, id);
