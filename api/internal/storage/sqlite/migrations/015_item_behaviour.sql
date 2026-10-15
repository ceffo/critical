-- One item model. The item kinds ammo, charged and consumable
-- and the per_day flag give way to a free-text type label and a count behaviour:
--   stock   = used and restocked at the table, stays listed at 0 (arrows, flasks)
--   charges = used only, removed at 0 (a wand)
--   daily   = used only, stays at 0, refilled to max_count by a long rest (Power Gem)
-- Existing rows migrate as ammo and consumable to stock, charged with per_day to
-- daily, other charged to charges. The type comes from the ammo kind, else from
-- the name (wand, flask, gem, potion, scroll), else from the old kind. Every row
-- and count is kept. SQLite cannot drop a CHECK column in place, so the table is
-- rebuilt and renamed (no other table references items).
CREATE TABLE items_v15 (
    id            TEXT    PRIMARY KEY,
    position      INTEGER NOT NULL,
    name          TEXT    NOT NULL,
    type          TEXT    NOT NULL DEFAULT '',
    behaviour     TEXT    NOT NULL CHECK (behaviour IN ('stock', 'charges', 'daily')),
    magic_bonus   INTEGER NOT NULL DEFAULT 0,
    ammo_kind     TEXT    NOT NULL DEFAULT '',
    count         INTEGER NOT NULL CHECK (count >= 0),
    max_count     INTEGER NOT NULL CHECK (max_count >= 0),
    effect_dice   TEXT    NOT NULL DEFAULT '',
    effect_save   TEXT    NOT NULL DEFAULT '',
    notes         TEXT    NOT NULL DEFAULT ''
);

INSERT INTO items_v15 (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes)
SELECT id, position, name,
       CASE
           WHEN ammo_kind <> '' THEN ammo_kind
           WHEN lower(name) LIKE '%wand%' THEN 'wand'
           WHEN lower(name) LIKE '%flask%' THEN 'flask'
           WHEN lower(name) LIKE '%gem%' THEN 'gem'
           WHEN lower(name) LIKE '%potion%' THEN 'potion'
           WHEN lower(name) LIKE '%scroll%' THEN 'scroll'
           ELSE kind
       END,
       CASE
           WHEN kind = 'charged' AND per_day = 1 THEN 'daily'
           WHEN kind = 'charged' THEN 'charges'
           ELSE 'stock'
       END,
       magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes
FROM items;

DROP TABLE items;

ALTER TABLE items_v15 RENAME TO items;
