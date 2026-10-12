-- Weapons and items catalogue, seeded from the campaign sheet (seeds/weapons.json,
-- seeds/items.json). New tables only. Weapons are static reference data, items
-- carry a counter (count) that changes at the table: ammo, charges, flasks.
CREATE TABLE IF NOT EXISTS weapons (
    id            TEXT    PRIMARY KEY,
    position      INTEGER NOT NULL,
    name          TEXT    NOT NULL,
    damage_sm     TEXT    NOT NULL,
    damage_l      TEXT    NOT NULL,
    magic_to_hit  INTEGER NOT NULL DEFAULT 0,
    magic_damage  INTEGER NOT NULL DEFAULT 0,
    kind          TEXT    NOT NULL CHECK (kind IN ('melee', 'missile')),
    two_handed    INTEGER NOT NULL DEFAULT 0 CHECK (two_handed IN (0, 1)),
    ammo_kind     TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS items (
    id            TEXT    PRIMARY KEY,
    position      INTEGER NOT NULL,
    name          TEXT    NOT NULL,
    kind          TEXT    NOT NULL CHECK (kind IN ('ammo', 'charged', 'consumable')),
    magic_bonus   INTEGER NOT NULL DEFAULT 0,
    ammo_kind     TEXT    NOT NULL DEFAULT '',
    count         INTEGER NOT NULL CHECK (count >= 0),
    max_count     INTEGER NOT NULL CHECK (max_count >= 0),
    per_day       INTEGER NOT NULL DEFAULT 0 CHECK (per_day IN (0, 1)),
    effect_dice   TEXT    NOT NULL DEFAULT '',
    effect_save   TEXT    NOT NULL DEFAULT '',
    notes         TEXT    NOT NULL DEFAULT ''
);
