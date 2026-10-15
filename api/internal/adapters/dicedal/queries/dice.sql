-- ListDiceFavourites
SELECT id, formula FROM dice_favourites WHERE character_id = ? ORDER BY position, id
;

-- InsertDiceFavourite
INSERT INTO dice_favourites (character_id, formula, position)
VALUES (?, ?, ?)
RETURNING id
;

-- LastDiceFavouritePosition
SELECT position FROM dice_favourites WHERE character_id = ? ORDER BY position DESC LIMIT 1
;

-- DeleteDiceFavourite
DELETE FROM dice_favourites WHERE character_id = ? AND id = ?
RETURNING id
;

-- SetDiceFavouritePosition
UPDATE dice_favourites SET position = ? WHERE character_id = ? AND id = ?
RETURNING id
;
