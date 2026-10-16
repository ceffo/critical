-- Dice composer favourites. A favourite is a formula the player saved to roll
-- again, kept per character on the server so every client sees the same row.
-- The formula is stored in its normal form (merged and ordered by the parser)
-- so the same formula can only be saved once. Free rolls are action log
-- entries, so no roll is stored here.
CREATE TABLE IF NOT EXISTS dice_favourites (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL REFERENCES characters(id),
    formula      TEXT    NOT NULL,
    UNIQUE (character_id, formula)
);
