-- An amend correction may carry a note (critical-qki.38): the action log
-- amend now changes only the note of an entry, never its values. Value amends
-- written before stay valid and still count. Existing rows keep NULL.
ALTER TABLE action_corrections ADD COLUMN note TEXT;
