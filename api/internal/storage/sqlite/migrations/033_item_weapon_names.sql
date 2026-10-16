-- Item and weapon names as localized texts (ADR 0023 steps 5 and 9):
-- items.name and weapons.name hold a key into localized_texts instead
-- of the text. Every existing name becomes the 'en' row of a new key (the
-- names so far were the sheet's English ones, or what the player typed,
-- which had no language either); the seed loader adds the other languages of
-- the names it still holds unchanged (seedItems, seedWeapons).
--
-- Keys follow the highest stored one, in list order (items first, then
-- weapons). A blank name, which the editors never allowed, takes the row's
-- id as its text, so every key has a row. Every row keeps its id, position,
-- count and every other field; game state refers to items and weapons by id
-- and is untouched. SQLite cannot change a column's type in place, so both
-- tables are rebuilt and renamed (no other table references them).
CREATE TABLE items_v33 (
    id            TEXT    PRIMARY KEY,
    position      INTEGER NOT NULL,
    name          INTEGER NOT NULL,
    type          TEXT    NOT NULL DEFAULT '',
    behaviour     TEXT    NOT NULL CHECK (behaviour IN ('stock', 'charges', 'daily')),
    magic_bonus   INTEGER NOT NULL DEFAULT 0,
    ammo_kind     TEXT    NOT NULL DEFAULT '',
    count         INTEGER NOT NULL CHECK (count >= 0),
    max_count     INTEGER NOT NULL CHECK (max_count >= 0),
    effect_dice   TEXT    NOT NULL DEFAULT '',
    effect_save   TEXT    NOT NULL DEFAULT '',
    notes         TEXT    NOT NULL DEFAULT '',
    powers        TEXT    NOT NULL DEFAULT '[]'
);

INSERT INTO items_v33 (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count,
    effect_dice, effect_save, notes, powers)
SELECT id, position,
       (SELECT COALESCE(MAX(text_id), 0) FROM localized_texts) + ROW_NUMBER() OVER (ORDER BY position, id),
       type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items;

INSERT INTO localized_texts (text_id, lang, body)
SELECT n.name, 'en', CASE WHEN trim(o.name) <> '' THEN o.name ELSE o.id END
FROM items_v33 n JOIN items o ON o.id = n.id;

DROP TABLE items;
ALTER TABLE items_v33 RENAME TO items;

CREATE TABLE weapons_v33 (
    id            TEXT    PRIMARY KEY,
    position      INTEGER NOT NULL,
    name          INTEGER NOT NULL,
    damage_sm     TEXT    NOT NULL,
    damage_l      TEXT    NOT NULL,
    magic_to_hit  INTEGER NOT NULL DEFAULT 0,
    magic_damage  INTEGER NOT NULL DEFAULT 0,
    kind          TEXT    NOT NULL CHECK (kind IN ('melee', 'missile')),
    two_handed    INTEGER NOT NULL DEFAULT 0 CHECK (two_handed IN (0, 1)),
    ammo_kind     TEXT    NOT NULL DEFAULT ''
);

INSERT INTO weapons_v33 (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind,
    two_handed, ammo_kind)
SELECT id, position,
       (SELECT COALESCE(MAX(text_id), 0) FROM localized_texts) + ROW_NUMBER() OVER (ORDER BY position, id),
       damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons;

INSERT INTO localized_texts (text_id, lang, body)
SELECT n.name, 'en', CASE WHEN trim(o.name) <> '' THEN o.name ELSE o.id END
FROM weapons_v33 n JOIN weapons o ON o.id = n.id;

DROP TABLE weapons;
ALTER TABLE weapons_v33 RENAME TO weapons;

-- The languages the seed has offered an item's or a weapon's name, by id
-- (ADR 0023 step 10): the seed offers each of its languages once, so a
-- language the player removed from a name (step 11) is not put back on the
-- next start. Keyed by id, not by text key: a name keeps its record when the
-- row is removed and put back under a new key (undo). Empty here; the first
-- start after 033 offers the seed's languages and records them.
CREATE TABLE seeded_name_languages (
    kind  TEXT NOT NULL CHECK (kind IN ('item', 'weapon')),
    id    TEXT NOT NULL,
    lang  TEXT NOT NULL,
    PRIMARY KEY (kind, id, lang)
);
