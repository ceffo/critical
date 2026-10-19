-- Every query is one character's: items belong to a character (character_id). The selected
-- columns are the whole row, so sqlc maps them to the Item model. An item is any piece of
-- equipment (ADR 0033 step 1), weapons included.

-- name: ListItems :many
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id, category, magic_kind, catalogue_code, component, state, weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit, magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind, ac_melee, ac_missile, spell_id
FROM items WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position;

-- name: GetItem :one
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id, category, magic_kind, catalogue_code, component, state, weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit, magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind, ac_melee, ac_missile, spell_id
FROM items WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;

-- name: DecrementItem :one
UPDATE items SET count = count - 1 WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id AND count > 0
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id, category, magic_kind, catalogue_code, component, state, weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit, magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind, ac_melee, ac_missile, spell_id;

-- name: IncrementItem :one
UPDATE items SET count = count + sqlc.arg(n) WHERE character_id = sqlc.arg(character_id)::bigint AND id = sqlc.arg(id)
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id, category, magic_kind, catalogue_code, component, state, weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit, magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind, ac_melee, ac_missile, spell_id;

-- name: ResetDailyItems :exec
UPDATE items SET count = max_count WHERE character_id = sqlc.arg(character_id)::bigint AND behaviour = 'daily';

-- name: SetItemCount :execrows
UPDATE items SET count = @count WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;

-- name: DeleteItem :one
DELETE FROM items WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id
RETURNING name;

-- name: InsertItem :exec
INSERT INTO items (character_id, id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count,
                   effect_dice, effect_save, notes, powers, category, magic_kind, catalogue_code, component, spell_id, state,
                   weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit,
                   magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind,
                   ac_melee, ac_missile)
VALUES (sqlc.arg(character_id)::bigint, @id, @position, @name, @type, @behaviour, @magic_bonus, @ammo_kind, @count,
        @max_count, @effect_dice, @effect_save, @notes, @powers, @category, @magic_kind, @catalogue_code, @component, @spell_id, @state,
        @weapon_type, @weapon_kind, @size, @damage_types, @speed, @damage_sm, @damage_l, @magic_to_hit,
        @magic_damage, @two_handed, @fires, @specialized, @base_ac, @skill_column, @armor_row, @shield_kind,
        @ac_melee, @ac_missile);

-- name: UpdateItem :one
UPDATE items SET position = @position, type = @type, behaviour = @behaviour, magic_bonus = @magic_bonus,
                 ammo_kind = @ammo_kind, count = @count, max_count = @max_count, effect_dice = @effect_dice,
                 effect_save = @effect_save, notes = @notes, powers = @powers, category = @category,
                 magic_kind = @magic_kind, catalogue_code = @catalogue_code, component = @component, spell_id = @spell_id, state = @state,
                 weapon_type = @weapon_type, weapon_kind = @weapon_kind, size = @size,
                 damage_types = @damage_types, speed = @speed, damage_sm = @damage_sm, damage_l = @damage_l,
                 magic_to_hit = @magic_to_hit, magic_damage = @magic_damage, two_handed = @two_handed,
                 fires = @fires, specialized = @specialized, base_ac = @base_ac, skill_column = @skill_column,
                 armor_row = @armor_row, shield_kind = @shield_kind, ac_melee = @ac_melee, ac_missile = @ac_missile
WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id
RETURNING name;

-- name: ItemNames :many
SELECT text_id, lang, body FROM localized_texts
WHERE text_id IN (SELECT name FROM items WHERE character_id = sqlc.arg(character_id)::bigint);

-- The review list (ADR 0033 step 27).
-- name: ListReviews :many
SELECT item_id, reason FROM equipment_review WHERE character_id = sqlc.arg(character_id)::bigint
ORDER BY item_id, reason;

-- name: DeleteReviews :exec
DELETE FROM equipment_review WHERE character_id = sqlc.arg(character_id)::bigint AND item_id = @item_id;
