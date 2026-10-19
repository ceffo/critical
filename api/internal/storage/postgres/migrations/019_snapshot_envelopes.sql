-- The action log's snapshot codec (ADR 0018 steps 8 and 9): every snapshot's "before" and "after"
-- are stored as the codec's envelope ({"id":…,"state":…} or {"id":…,"absent":true}) in two new
-- columns. A data step run just before migration 020 re-encodes every stored snapshot into them
-- (reencodeSnapshots); the old columns keep the old encoding, unchanged, and are no longer read.
--
-- Additive (ADR 0024 step 5.2): the release before this one keeps reading the old columns, so on
-- the machine not yet replaced an undo of an entry recorded before the deploy still restores its
-- state. A row the new release writes leaves the old columns at their default, a JSON string
-- that every decoder of the release before refuses (its undo of such an entry fails, changing
-- nothing, instead of writing a misread state). Migration 020 makes the new columns required, so
-- the release before cannot record a snapshot the new one could not read. The old columns are
-- dropped by a later migration, once no running release reads them (with the data step).
-- Schema only (ADR 0025 step 2.1).
ALTER TABLE action_snapshots ADD COLUMN before_envelope TEXT;
ALTER TABLE action_snapshots ADD COLUMN after_envelope  TEXT;
ALTER TABLE action_snapshots ALTER COLUMN before_state SET DEFAULT '"see before_envelope"';
ALTER TABLE action_snapshots ALTER COLUMN after_state  SET DEFAULT '"see after_envelope"';
