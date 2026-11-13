-- An item's abilities (ADR 0049 steps 4, 5 and 18): what the item does when used, each under a usage
-- limit, as one JSON list in items.abilities. NULL is a piece not converted yet, which is every
-- existing row: the conversion of the old behaviour and count into abilities is a boot step of a
-- later slice (ADR 0049 step 19; migrations hold no data, ADR 0025 step 2.1.1). An empty list "[]"
-- is a piece converted with none. Additive (ADR 0024 step 5.2): behaviour, max_count, effect_dice,
-- effect_save and count stay as stored and keep their meaning, and an older release ignores the
-- column. Schema only.
ALTER TABLE items ADD COLUMN abilities TEXT;
COMMENT ON COLUMN items.abilities IS 'the ability list (ADR 0049): [{"id", "name"?, "note"?, "limit": {"kind", "max"?, "left"?, "cost"?}, "effects": [{"kind": "dice", "dice", "save"?, "label"?}]}]; NULL: not converted yet, []: none';
