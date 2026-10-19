-- Every snapshot has its envelopes (migration 019): the data step run just before this file
-- re-encoded every stored one (reencodeSnapshots, ADR 0025 step 2.1.1). Required from now on, so
-- the release before 019, which does not write them, cannot record a snapshot during a rolling
-- deploy (its request fails and rolls back instead of leaving a snapshot in the old encoding,
-- which this release no longer reads: ADR 0018 step 9). Schema only (ADR 0025 step 2.1).
ALTER TABLE action_snapshots ALTER COLUMN before_envelope SET NOT NULL;
ALTER TABLE action_snapshots ALTER COLUMN after_envelope  SET NOT NULL;
