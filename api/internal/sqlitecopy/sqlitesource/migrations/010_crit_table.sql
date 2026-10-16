-- House crit table: the second D100 of a crit looks up its
-- multiplier here. Rows are inclusive D100 ranges covering 1-100 exactly, the
-- app validates that on save. New table only. An empty table means the
-- default (one range 1-100 -> x2, shown as "placeholder"), so an existing DB
-- needs no data migration. (The migrator splits on semicolons: none in comments.)
CREATE TABLE IF NOT EXISTS crit_table_rows (
    roll_from  INTEGER PRIMARY KEY CHECK (roll_from BETWEEN 1 AND 100),
    roll_to    INTEGER NOT NULL CHECK (roll_to BETWEEN 1 AND 100 AND roll_to >= roll_from),
    multiplier INTEGER NOT NULL CHECK (multiplier >= 1)
);
