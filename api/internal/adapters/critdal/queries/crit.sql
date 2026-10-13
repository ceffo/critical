-- ListCritRows
SELECT roll_from, roll_to, multiplier
FROM crit_table_rows ORDER BY roll_from
;

-- DeleteCritRows
DELETE FROM crit_table_rows
;

-- InsertCritRow
INSERT INTO crit_table_rows (roll_from, roll_to, multiplier)
VALUES (?, ?, ?)
;
