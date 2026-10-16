-- An item's own powers (ADR 0016 steps 9 to 16): effects in the buffs'
-- vocabulary with an optional trigger, stored on the item row as a JSON list
-- (the encoding of itemsvc.EncodePowers; '[]' for none). Powers come from the
-- seed and from data migrations only, never from the item editor.
ALTER TABLE items ADD COLUMN powers TEXT NOT NULL DEFAULT '[]';

-- The Power Gem (step 16): one item, its daily Dispel Magic (the item's use,
-- unchanged) and its thunder hammer, on at effective STR 18 or more, melee
-- only: the weapon becomes +5 and its raw damage dice are doubled. The row
-- keeps its id (log entries and snapshots refer to it), position and count.
-- A database without the row is untouched; on a new database the seed,
-- loaded after the migrations, carries the same item.
UPDATE items
SET powers = '[{"id":"thunder_hammer","trigger":{"min_str":18},"effects":{"weapon_buff":5,"multiplier":2,"weapon_kind":"melee"}}]'
WHERE id = 'power_gem_dispel';

-- Renamed only while it still has its seed name: a name the player changed
-- is kept.
UPDATE items
SET name = 'Power Gem'
WHERE id = 'power_gem_dispel' AND name = 'Power Gem: Dispel Magic';

-- The old note said the +5 was handled by buffs. Replaced only while it is
-- still the seed's note, word for word: a note the player edited is kept.
UPDATE items
SET notes = 'Dispel Magic once a day (10 m radius), at any STR. Thunder hammer at effective STR 18 or more: a melee weapon becomes +5 and its damage dice are doubled. Not modelled: the trident of submission, the undead-destroying mace, the hammer''s stun.'
WHERE id = 'power_gem_dispel'
  AND notes = '1/day big Dispel Magic. The weapon becomes +5 while the gem is used (STR 18 gate, handled by buffs).';
