-- One equipment model (ADR 0033 steps 1 to 9, 26 and 27): items becomes every piece of a
-- character's equipment, weapons included.
--
-- A piece has a category (weapon, ammunition, armour, shield, magic, gear; step 2), a magic kind
-- (a magic piece's potion ... misc), an optional catalogue code (the catalogue row it was picked
-- from, step 21) and a state (carried, worn, wielded; step 6), plus the fields of its category
-- (steps 3 to 5). The codes are validated by itemsvc on every write and have no CHECK, so each
-- list lives in one place (as weapons.weapon_type, migration 005). The columns a weapon had move
-- here under the same names; weapons.ammo_kind, the ammunition a launcher fires, is fires (an
-- ammunition piece's own kind stays ammo_kind). items.type, the free-text label, is kept as stored
-- (step 26.1) and no longer edited.
--
-- equipment_review is the review list of step 27: the pieces the conversion could not settle, each
-- with its reason code; a row goes when the player fixes the piece or dismisses it (not logged).
-- item_id_renames is the conversion table of step 26.3: an item whose id a weapon of the same
-- character already had takes the next free suffix, and the restorer of its old snapshots reads
-- the new id here.
--
-- Additive (ADR 0024 step 5.2, ADR 0025 step 2.2): new columns with defaults and new tables; the
-- release before this one keeps reading and writing items and weapons as before (it lists the
-- converted weapons among its items for the minutes both run). The weapons table is no longer
-- read or written and is dropped by a later migration, once no running release reads it. Schema
-- only (step 2.1): every character's items and weapons are converted by a data step run just
-- after this file (convertEquipment), in the same transaction.
ALTER TABLE items ADD COLUMN category       TEXT   NOT NULL DEFAULT 'gear';
ALTER TABLE items ADD COLUMN magic_kind     TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN catalogue_code TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN state          TEXT   NOT NULL DEFAULT 'carried';
ALTER TABLE items ADD COLUMN weapon_type    TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN weapon_kind    TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN size           TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN damage_types   TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE items ADD COLUMN speed          BIGINT NOT NULL DEFAULT 0 CHECK (speed BETWEEN 0 AND 20);
ALTER TABLE items ADD COLUMN damage_sm      TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN damage_l       TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN magic_to_hit   BIGINT NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN magic_damage   BIGINT NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN two_handed     BIGINT NOT NULL DEFAULT 0 CHECK (two_handed IN (0, 1));
ALTER TABLE items ADD COLUMN fires          TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN specialized    BIGINT NOT NULL DEFAULT 0 CHECK (specialized IN (0, 1));
ALTER TABLE items ADD COLUMN base_ac        BIGINT NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN skill_column   TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN armor_row      TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN shield_kind    TEXT   NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN ac_melee       BIGINT NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN ac_missile     BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN items.type IS 'the free-text label of before the equipment model, kept as stored, no longer edited';
COMMENT ON TABLE weapons IS 'unused since migration 014: its rows were converted into items';

CREATE TABLE equipment_review (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    item_id      TEXT   NOT NULL,
    reason       TEXT   NOT NULL,
    PRIMARY KEY (character_id, item_id, reason)
);

CREATE TABLE item_id_renames (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    old_id       TEXT   NOT NULL,
    new_id       TEXT   NOT NULL,
    PRIMARY KEY (character_id, old_id)
);
