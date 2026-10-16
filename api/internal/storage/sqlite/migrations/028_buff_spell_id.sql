-- A buff from a spell carries the spell's catalogue id (spell_id) so the UI
-- names it in the player's language (ADR 0004 step 17). The active buffs and
-- the buff snapshots of action_snapshots (undo and redo restore them) are
-- JSON lists, so no column changes: this backfills the buffs stored before.
--
-- Backfill rule: a buff without spell_id gets one only when its name equals
-- (exactly, case included) the English name of exactly one catalogue spell
-- (a row with a key). A name no keyed spell has (a hand-made buff, a Power
-- Gem form), or that several spells share (a Wizard and a Priest spell of
-- the same name), is left alone and keeps showing its stored name. A
-- database whose spells are not keyed yet (catalogue never imported) matches
-- nothing. A list holding anything but objects is not a buff list and is
-- left as it is. Action-log payloads are not rewritten: an old entry shows
-- the name it recorded.

UPDATE active_buffs SET buffs = (
    SELECT json_group_array(json(
        CASE
            WHEN json_extract(b.value, '$.spell_id') IS NULL
             AND (SELECT COUNT(*) FROM spells s
                  WHERE s.key IS NOT NULL AND s.name = json_extract(b.value, '$.name')) = 1
            THEN json_set(b.value, '$.spell_id',
                 (SELECT s.key FROM spells s
                  WHERE s.key IS NOT NULL AND s.name = json_extract(b.value, '$.name')))
            ELSE b.value
        END) ORDER BY b.key)
    FROM json_each(active_buffs.buffs) b
)
WHERE json_valid(buffs) AND json_type(buffs) = 'array'
  AND NOT EXISTS (SELECT 1 FROM json_each(buffs) WHERE type <> 'object');

UPDATE action_snapshots SET before_state = (
    SELECT json_group_array(json(
        CASE
            WHEN json_extract(b.value, '$.spell_id') IS NULL
             AND (SELECT COUNT(*) FROM spells s
                  WHERE s.key IS NOT NULL AND s.name = json_extract(b.value, '$.name')) = 1
            THEN json_set(b.value, '$.spell_id',
                 (SELECT s.key FROM spells s
                  WHERE s.key IS NOT NULL AND s.name = json_extract(b.value, '$.name')))
            ELSE b.value
        END) ORDER BY b.key)
    FROM json_each(action_snapshots.before_state) b
)
WHERE state_key = 'buffs' AND json_valid(before_state) AND json_type(before_state) = 'array'
  AND NOT EXISTS (SELECT 1 FROM json_each(before_state) WHERE type <> 'object');

UPDATE action_snapshots SET after_state = (
    SELECT json_group_array(json(
        CASE
            WHEN json_extract(b.value, '$.spell_id') IS NULL
             AND (SELECT COUNT(*) FROM spells s
                  WHERE s.key IS NOT NULL AND s.name = json_extract(b.value, '$.name')) = 1
            THEN json_set(b.value, '$.spell_id',
                 (SELECT s.key FROM spells s
                  WHERE s.key IS NOT NULL AND s.name = json_extract(b.value, '$.name')))
            ELSE b.value
        END) ORDER BY b.key)
    FROM json_each(action_snapshots.after_state) b
)
WHERE state_key = 'buffs' AND json_valid(after_state) AND json_type(after_state) = 'array'
  AND NOT EXISTS (SELECT 1 FROM json_each(after_state) WHERE type <> 'object');
