-- name: ListDiceFavourites :many
SELECT id, formula FROM dice_favourites WHERE character_id = $1 ORDER BY position, id;

-- name: LastDiceFavouritePosition :one
SELECT position FROM dice_favourites WHERE character_id = $1 ORDER BY position DESC LIMIT 1;

-- name: InsertDiceFavourite :one
INSERT INTO dice_favourites (character_id, formula, position)
VALUES ($1, $2, $3)
RETURNING id;

-- name: DeleteDiceFavourite :execrows
DELETE FROM dice_favourites WHERE character_id = $1 AND id = $2;

-- name: SetDiceFavouritePosition :execrows
UPDATE dice_favourites SET position = $1 WHERE character_id = $2 AND id = $3;
