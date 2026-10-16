-- name: ListItems :many
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items ORDER BY position;

-- name: GetItem :one
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items WHERE id = $1;

-- name: DecrementItem :one
UPDATE items SET count = count - 1 WHERE id = $1 AND count > 0
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers;

-- name: IncrementItem :one
UPDATE items SET count = count + sqlc.arg(n) WHERE id = sqlc.arg(id)
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers;

-- name: ResetDailyItems :exec
UPDATE items SET count = max_count WHERE behaviour = 'daily';

-- name: SetItemCount :execrows
UPDATE items SET count = $1 WHERE id = $2;

-- name: DeleteItem :one
DELETE FROM items WHERE id = $1
RETURNING name;

-- name: InsertItem :exec
INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: UpdateItem :one
UPDATE items SET position = $1, type = $2, behaviour = $3, magic_bonus = $4, ammo_kind = $5, count = $6, max_count = $7, effect_dice = $8, effect_save = $9, notes = $10, powers = $11
WHERE id = $12
RETURNING name;

-- name: ItemNames :many
SELECT text_id, lang, body FROM localized_texts WHERE text_id IN (SELECT name FROM items);
