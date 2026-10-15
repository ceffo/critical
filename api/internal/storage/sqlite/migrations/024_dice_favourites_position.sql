-- Dice favourites get a user-chosen order. position is 1-based per character.
-- Existing rows are numbered in creation order (by id) so the list does not
-- change for anyone. New favourites take the next position (see dice.sql).
ALTER TABLE dice_favourites ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

UPDATE dice_favourites SET position = (
    SELECT COUNT(*) FROM dice_favourites o
    WHERE o.character_id = dice_favourites.character_id AND o.id <= dice_favourites.id
);
