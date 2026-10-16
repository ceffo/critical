-- Crit table multipliers may be decimals: x1.5, x2.5. The
-- column becomes REAL and keeps CHECK multiplier >= 1. SQLite cannot alter a
-- column type, so the table is rebuilt and the rows copied (integers become
-- 2.0, 3.0 and so on, same values). (The migrator splits on semicolons: none in comments.)
CREATE TABLE crit_table_rows_new (
    roll_from  INTEGER PRIMARY KEY CHECK (roll_from BETWEEN 1 AND 100),
    roll_to    INTEGER NOT NULL CHECK (roll_to BETWEEN 1 AND 100 AND roll_to >= roll_from),
    multiplier REAL NOT NULL CHECK (multiplier >= 1)
);
INSERT INTO crit_table_rows_new (roll_from, roll_to, multiplier)
    SELECT roll_from, roll_to, multiplier FROM crit_table_rows;
DROP TABLE crit_table_rows;
ALTER TABLE crit_table_rows_new RENAME TO crit_table_rows;
