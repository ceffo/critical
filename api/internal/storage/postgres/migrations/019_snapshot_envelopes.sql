-- The action log's snapshot codec (ADR 0018 steps 8 and 9): every snapshot's "before" and "after"
-- are stored as the codec's envelope ({"id":…,"state":…} or {"id":…,"absent":true}) in two new
-- columns. A data step run right after this file re-encodes every stored snapshot into them
-- (reencodeSnapshots); the old columns keep the old encoding, unchanged, and are no longer read.
--
-- Additive (ADR 0024 step 5.2): the release before this one keeps working during a rolling
-- deploy. It reads the old columns, so its undo of an entry recorded before the deploy still
-- restores its state. A row the new release writes leaves the old columns at their default, a
-- JSON string every decoder of the release before refuses (its undo of such an entry fails,
-- changing nothing, instead of writing a misread state). The release before still records
-- snapshots, in the old columns only: the new release reads such a row's envelopes as missing and
-- refuses to undo or redo that entry. The envelopes become required, and the old columns are
-- dropped, by a later migration, once no running release writes or reads the old columns; the
-- data step runs again just before it, for the rows written meanwhile.
-- Schema only (ADR 0025 step 2.1).
ALTER TABLE action_snapshots ADD COLUMN before_envelope TEXT;
ALTER TABLE action_snapshots ADD COLUMN after_envelope  TEXT;
ALTER TABLE action_snapshots ALTER COLUMN before_state SET DEFAULT '"see before_envelope"';
ALTER TABLE action_snapshots ALTER COLUMN after_state  SET DEFAULT '"see after_envelope"';
