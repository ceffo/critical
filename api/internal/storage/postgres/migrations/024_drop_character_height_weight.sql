-- Drops characters.height and characters.weight, the free-text height and weight (ADR 0032 step 8):
-- migration 015 made the numbers characters.height_cm and characters.weight_kg and stopped reading
-- and writing the text, and the release that removed the string keys from the API is deployed
-- everywhere, so no running release reads them any more.
--
-- Contract step of the expand/contract that 015 began (ADR 0024 step 5.2, ADR 0025 step 2.2). The
-- release before this one never names the columns (its queries list their columns), so it keeps
-- running on the new schema until it is replaced; one restarted on it refuses to start, a database
-- holding a migration it does not embed (ADR 0024 step 6.2). Schema only (step 2.1). The free text
-- is not parsed any more: the data step 015 had is gone with the columns it read.
ALTER TABLE characters DROP COLUMN height;
ALTER TABLE characters DROP COLUMN weight;
