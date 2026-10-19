-- Drops the legacy HP ledger table. Migration 007 carried its rows into action_log and nothing has
-- read or written it since; the HP ledger is the action log (ADR 0007).
--
-- Not additive, but safe (ADR 0024 step 5.2, ADR 0025 step 2.2): the release before this one never
-- touches hp_events, so it keeps running against the schema while the new release migrates. Schema
-- only, no data step: the rows were already carried.
DROP TABLE hp_events;
