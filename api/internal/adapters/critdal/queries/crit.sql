-- name: ListCritRows :many
SELECT roll_from, roll_to, multiplier
FROM crit_table_rows ORDER BY roll_from;

-- name: DeleteCritRows :exec
DELETE FROM crit_table_rows;

-- name: InsertCritRow :exec
INSERT INTO crit_table_rows (roll_from, roll_to, multiplier)
VALUES ($1, $2, $3);
