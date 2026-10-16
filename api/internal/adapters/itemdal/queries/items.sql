-- ListItems
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items ORDER BY position
;

-- GetItem
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
FROM items WHERE id = ?
;

-- DecrementItem
UPDATE items SET count = count - 1 WHERE id = ? AND count > 0
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
;

-- IncrementItem
UPDATE items SET count = count + ? WHERE id = ?
RETURNING id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers
;

-- ResetDailyItems
UPDATE items SET count = max_count WHERE behaviour = 'daily'
;

-- SetItemCount
UPDATE items SET count = ? WHERE id = ?
RETURNING id
;

-- DeleteItem
DELETE FROM items WHERE id = ?
RETURNING id
;

-- InsertItem
INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
;

-- UpdateItem
UPDATE items SET position = ?, name = ?, type = ?, behaviour = ?, magic_bonus = ?, ammo_kind = ?, count = ?, max_count = ?, effect_dice = ?, effect_save = ?, notes = ?, powers = ?
WHERE id = ?
RETURNING id
;
