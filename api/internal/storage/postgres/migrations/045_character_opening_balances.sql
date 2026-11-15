-- The opening balances of the HP and PSP ledgers (ADR 0007 step 2, ADR 0043 step 5, ADR 0029 step
-- 12): the sum a character's ledgers already held when its action log began. A new character has 0
-- of each; a copy of a character starts with the deficit its source had (current minus maximum),
-- since a copy keeps no history and its log is empty. Current HP is hp_max + hp_opening + the sum of
-- the log's effective deltas, and PSPs likewise. It is not a log entry: nothing pops it, so the
-- ledger stays append-only.
--
-- Additive with defaults (ADR 0024 step 5.2, ADR 0025 step 2.2): every existing row reads 0, and the
-- release before this one, which ignores the columns, reads a character with an opening as if it
-- had none for the minutes both run.
ALTER TABLE characters ADD COLUMN hp_opening BIGINT NOT NULL DEFAULT 0;
ALTER TABLE characters ADD COLUMN psp_opening BIGINT NOT NULL DEFAULT 0;
