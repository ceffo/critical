-- The attack form's last choices, one row per character: the weapon in hand,
-- the previous weapon (one-click switch back), the ammunition, the target AC
-- and size and the situational modifier with its reason. New table only, so an
-- existing DB needs no data migration: a missing row reads as the defaults.
-- Ids are not foreign keys: a weapon or item removed later is simply ignored
-- when the form is read. (The migrator splits on semicolons: none in comments.)
CREATE TABLE IF NOT EXISTS attack_forms (
    character_id       INTEGER PRIMARY KEY REFERENCES characters(id),
    weapon_id          TEXT    NOT NULL DEFAULT '',
    previous_weapon_id TEXT    NOT NULL DEFAULT '',
    ammo_item_id       TEXT    NOT NULL DEFAULT '',
    target_ac          INTEGER NOT NULL DEFAULT 5,
    target_size        TEXT    NOT NULL DEFAULT 'sm' CHECK (target_size IN ('sm', 'l')),
    situational_value  INTEGER NOT NULL DEFAULT 0,
    situational_label  TEXT    NOT NULL DEFAULT ''
);
