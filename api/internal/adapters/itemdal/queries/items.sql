-- name: ListItems :many
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items ORDER BY position;

-- name: GetItem :one
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items WHERE id = ?;

-- name: DecrementItem :one
UPDATE items SET count = count - 1 WHERE id = ? AND count > 0
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers;

-- name: IncrementItem :one
UPDATE items SET count = count + sqlc.arg(n) WHERE id = sqlc.arg(id)
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers;

-- name: ResetDailyItems :exec
UPDATE items SET count = max_count WHERE behaviour = 'daily';

-- name: SetItemCount :execrows
UPDATE items SET count = ? WHERE id = ?;

-- name: DeleteItem :one
DELETE FROM items WHERE id = ?
RETURNING name;

-- name: InsertItem :exec
INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateItem :one
UPDATE items SET position = ?, type = ?, behaviour = ?, magic_bonus = ?, ammo_kind = ?, count = ?, max_count = ?, effect_dice = ?, effect_save = ?, notes = ?, powers = ?
WHERE id = ?
RETURNING name;

-- name: ItemNames :many
SELECT text_id, lang, body FROM localized_texts WHERE text_id IN (SELECT name FROM items);
