-- Every query is one character's: items belong to a character (character_id). The selected
-- columns are the whole row, so sqlc maps them to the Item model.

-- name: ListItems :many
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id
FROM items WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position;

-- name: GetItem :one
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id
FROM items WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;

-- name: DecrementItem :one
UPDATE items SET count = count - 1 WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id AND count > 0
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id;

-- name: IncrementItem :one
UPDATE items SET count = count + sqlc.arg(n) WHERE character_id = sqlc.arg(character_id)::bigint AND id = sqlc.arg(id)
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, character_id;

-- name: ResetDailyItems :exec
UPDATE items SET count = max_count WHERE character_id = sqlc.arg(character_id)::bigint AND behaviour = 'daily';

-- name: SetItemCount :execrows
UPDATE items SET count = @count WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;

-- name: DeleteItem :one
DELETE FROM items WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id
RETURNING name;

-- name: InsertItem :exec
INSERT INTO items (character_id, id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count,
                   effect_dice, effect_save, notes, powers)
VALUES (sqlc.arg(character_id)::bigint, @id, @position, @name, @type, @behaviour, @magic_bonus, @ammo_kind, @count,
        @max_count, @effect_dice, @effect_save, @notes, @powers);

-- name: UpdateItem :one
UPDATE items SET position = @position, type = @type, behaviour = @behaviour, magic_bonus = @magic_bonus,
                 ammo_kind = @ammo_kind, count = @count, max_count = @max_count, effect_dice = @effect_dice,
                 effect_save = @effect_save, notes = @notes, powers = @powers
WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id
RETURNING name;

-- name: ItemNames :many
SELECT text_id, lang, body FROM localized_texts
WHERE text_id IN (SELECT name FROM items WHERE character_id = sqlc.arg(character_id)::bigint);
