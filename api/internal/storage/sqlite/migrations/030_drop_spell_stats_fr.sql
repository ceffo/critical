-- Drops the French stat texts added by 029 (ADR 0021 step 8): the six stats
-- are stored as the book's English text plus their typed form (`stats`), and
-- the UI words the typed values in its language. No other column, index or
-- view refers to them, so a plain DROP COLUMN is safe on a database with 029
-- applied; every other spell column and row is kept.
ALTER TABLE spells DROP COLUMN casting_time_fr;
ALTER TABLE spells DROP COLUMN range_fr;
ALTER TABLE spells DROP COLUMN duration_fr;
ALTER TABLE spells DROP COLUMN area_of_effect_fr;
ALTER TABLE spells DROP COLUMN components_fr;
ALTER TABLE spells DROP COLUMN saving_throw_fr;

-- The typed stats are parsed at import (ADR 0021 step 5.6) and the area
-- parser now marks the readings it holds only in part (step 8). Forgetting
-- the recorded artifact makes the next startup import it again, re-typing
-- every spell with the current parser; the import keeps every key and row.
DELETE FROM catalogue_meta;
