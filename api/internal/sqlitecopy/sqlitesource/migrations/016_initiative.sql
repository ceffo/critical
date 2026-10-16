-- The round's initiative (ADR 0013): the d10 rolled on Next Round and the
-- action-log entry that rolled it. Both NULL when there is none (out of
-- combat, after End combat, before the first Next Round). Existing rows keep NULL.
ALTER TABLE round_state ADD COLUMN initiative_d10 INTEGER;
ALTER TABLE round_state ADD COLUMN initiative_entry_id INTEGER;
