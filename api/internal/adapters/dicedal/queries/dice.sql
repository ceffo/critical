-- ListDiceFavourites
SELECT id, formula FROM dice_favourites WHERE character_id = ? ORDER BY id
;

-- InsertDiceFavourite
INSERT INTO dice_favourites (character_id, formula)
VALUES (?, ?)
RETURNING id
;

-- DeleteDiceFavourite
DELETE FROM dice_favourites WHERE character_id = ? AND id = ?
RETURNING id
;
