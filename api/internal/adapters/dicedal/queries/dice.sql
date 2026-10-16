-- name: ListDiceFavourites :many
SELECT id, formula FROM dice_favourites WHERE character_id = ? ORDER BY position, id;

-- name: LastDiceFavouritePosition :one
SELECT position FROM dice_favourites WHERE character_id = ? ORDER BY position DESC LIMIT 1;

-- name: InsertDiceFavourite :one
INSERT INTO dice_favourites (character_id, formula, position)
VALUES (?, ?, ?)
RETURNING id;

-- name: DeleteDiceFavourite :execrows
DELETE FROM dice_favourites WHERE character_id = ? AND id = ?;

-- name: SetDiceFavouritePosition :execrows
UPDATE dice_favourites SET position = ? WHERE character_id = ? AND id = ?;
