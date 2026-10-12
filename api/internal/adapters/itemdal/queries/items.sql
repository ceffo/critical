-- ListItems
SELECT id, name, kind, magic_bonus, ammo_kind, count, max_count, per_day, effect_dice, effect_save, notes
FROM items ORDER BY position
;

-- GetItem
SELECT id, name, kind, magic_bonus, ammo_kind, count, max_count, per_day, effect_dice, effect_save, notes
FROM items WHERE id = ?
;

-- DecrementItem
UPDATE items SET count = count - 1 WHERE id = ? AND count > 0
RETURNING id, name, kind, magic_bonus, ammo_kind, count, max_count, per_day, effect_dice, effect_save, notes
;

-- ResetPerDayItems
UPDATE items SET count = max_count WHERE per_day = 1
;
