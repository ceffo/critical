-- Drops the legacy HP ledger table. The SQLite migration 007 (before the PostgreSQL baseline)
-- carried its rows into action_log; nothing has read or written it since, and the databases of
-- staging and production started from the seeds, so none ever held a row. The HP ledger is the
-- action log (ADR 0007).
--
-- A DROP is not additive (the open-questions row "A migration during a rolling production deploy",
-- ADR 0024 step 5.2), but it keeps the row's reason: the release before this one has no code that
-- reads or writes hp_events, so it keeps running on the new schema while its machines are replaced.
-- Schema only, no data step (ADR 0025 step 2.2).
DROP TABLE hp_events;
