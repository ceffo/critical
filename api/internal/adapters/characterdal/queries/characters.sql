-- level, thac0_base and the save_* columns are the figures the imported sheet stated (ADR 0030
-- step 24): compared with the derived values, never read as inputs.
-- realized_level is the level the character has taken (ADR 0030 step 7); NULL is the sheet's level
-- (step 7.5), which stays the imported figure step 24 compares.
-- name: GetCharacter :one
SELECT id, name, str, str_exceptional, dex, con, int, wis, cha, hp_max, psp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
       class, race_code, alignment, age, sex, height_cm, weight_kg, hair, eyes, languages,
       COALESCE(realized_level, level)::BIGINT AS realized_level, skill_sets, proficiency_picks, psionic_picks
FROM characters WHERE id = $1;

-- A character's class entries in the order the classes were taken (ADR 0042 step 1).
-- name: ListClassEntries :many
SELECT class, realized_level, starting_xp, state, left_at_level, ignored_awards_through
FROM character_classes WHERE character_id = $1 ORDER BY position;

-- name: InsertClassEntry :exec
INSERT INTO character_classes (character_id, position, class, realized_level, starting_xp, state, left_at_level, ignored_awards_through)
VALUES (sqlc.arg(character_id), sqlc.arg(position), sqlc.arg(class), sqlc.arg(realized_level),
        sqlc.arg(starting_xp), sqlc.arg(state), sqlc.arg(left_at_level), sqlc.arg(ignored_awards_through));

-- Taking a new class and its undo (ADR 0042 step 3): the entries are replaced whole, in one transaction.
-- name: DeleteClassEntries :exec
DELETE FROM character_classes WHERE character_id = sqlc.arg(character_id);

-- The first active entry's class, level and XP are the characters columns' too (migration 035).
-- name: SetClassColumns :execrows
UPDATE characters SET class = sqlc.arg(class), realized_level = sqlc.arg(realized_level), xp_total = sqlc.arg(xp_total)
WHERE id = sqlc.arg(id);

-- A level-up roll and its undo (ADR 0030 step 7.3): the realized level and the max HP. The level
-- is the character's first active entry's, written to its column too (the release before the
-- class entries reads it, migration 035).
-- name: SetRealizedLevel :execrows
UPDATE characters SET realized_level = sqlc.arg(realized_level), hp_max = sqlc.arg(hp_max)
WHERE id = sqlc.arg(id);

-- A psionicist level-up and its undo (ADR 0043 step 4.2): the PSP maximum.
-- name: SetPSPMax :execrows
UPDATE characters SET psp_max = sqlc.arg(psp_max) WHERE id = sqlc.arg(id);

-- name: SetFirstActiveEntryLevel :exec
UPDATE character_classes SET realized_level = sqlc.arg(realized_level)
WHERE character_classes.character_id = sqlc.arg(character_id) AND position = (
    SELECT MIN(e.position) FROM character_classes e WHERE e.character_id = sqlc.arg(character_id) AND e.state = 'active'
);

-- A level-up roll of a multi-class character's other entry and its undo (ADR 0042 step 7): the
-- entry's level and the max HP; the column keeps the first active entry's level.
-- name: SetHPMax :execrows
UPDATE characters SET hp_max = sqlc.arg(hp_max) WHERE id = sqlc.arg(id);

-- name: SetEntryLevel :execrows
UPDATE character_classes SET realized_level = sqlc.arg(realized_level)
WHERE character_id = sqlc.arg(character_id) AND class = sqlc.arg(class) AND state = 'active';

-- A thief's skill points spent on the sheet and their undo (ADR 0030 step 17).
-- name: SetSkillPoints :execrows
UPDATE characters SET skill_sets = sqlc.arg(skill_sets) WHERE id = sqlc.arg(id);

-- A character's class (its first active entry's, ADR 0042 step 1; the column for a character with no
-- entry) and stored skill points, for the undo of a spend (no derivation).
-- name: GetSkillSets :one
SELECT COALESCE((SELECT e.class FROM character_classes e
                 WHERE e.character_id = characters.id AND e.state = 'active' ORDER BY e.position LIMIT 1), class)::TEXT AS class,
       skill_sets
FROM characters WHERE id = sqlc.arg(id);

-- The proficiency picks saved by the editor and their undo (ADR 0037 steps 12 and 13).
-- name: SetProficiencyPicks :execrows
UPDATE characters SET proficiency_picks = sqlc.arg(proficiency_picks) WHERE id = sqlc.arg(id);

-- The powers a psionicist knows, saved as one list and restored by its undo (ADR 0043 step 9).
-- name: SetPsionicPicks :execrows
UPDATE characters SET psionic_picks = sqlc.arg(psionic_picks) WHERE id = sqlc.arg(id);

-- name: GetCharacterDetails :one
SELECT COALESCE((SELECT e.class FROM character_classes e
                 WHERE e.character_id = characters.id AND e.state = 'active' ORDER BY e.position LIMIT 1), class)::TEXT AS class,
       race_code, alignment, age, sex, height_cm, weight_kg, hair, eyes, languages
FROM characters WHERE id = $1;

-- The details card's edit: every detail but the languages, which the Skills tab saves (ADR 0037
-- step 18, SetLanguages).
-- name: UpdateCharacterDetails :execrows
UPDATE characters
SET alignment = $1, age = $2, sex = $3, height_cm = $4, weight_kg = $5, hair = $6, eyes = $7
WHERE id = $8;

-- The languages saved by the Skills tab's editor and their undo (ADR 0037 steps 13 and 18).
-- name: SetLanguages :execrows
UPDATE characters SET languages = sqlc.arg(languages) WHERE id = sqlc.arg(id);

-- name: CharactersOf :many
-- The owner's characters they have not retired, in creation order (the service derives each level).
SELECT id, name FROM characters
WHERE owner_user_id = $1 AND NOT retired
ORDER BY id;

-- name: OthersCharacters :many
-- Every character not retired of an enabled user but the viewer's (ADR 0029 step 11.2), by owner
-- then id (the service derives each level). The owner's display name is NULL when unknown.
SELECT c.id, c.name, u.id AS owner_id, u.display_name AS owner_name FROM characters c
JOIN users u ON u.id = c.owner_user_id
WHERE u.id <> $1 AND NOT u.disabled AND NOT c.retired
ORDER BY u.display_name NULLS LAST, u.id, c.id;

-- name: ViewableCharacter :one
-- The character's id when it is not retired and its owner is enabled (ADR 0029 step 11.1): no row
-- for an unknown, retired or disabled-owner's character, or one nobody owns.
SELECT c.id FROM characters c
JOIN users u ON u.id = c.owner_user_id AND NOT u.disabled
WHERE c.id = $1 AND NOT c.retired;

-- name: LockUser :one
-- Serialises the writes that move a user's current character (switch, create, retire): the
-- later one waits for the earlier one's commit, then sees its effect (READ COMMITTED).
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: SetCurrentCharacter :execrows
-- Only a character the user owns and has not retired: none updated is an unknown character.
UPDATE users SET current_character_id = sqlc.arg(character_id)
WHERE users.id = sqlc.arg(user_id) AND EXISTS (
    SELECT 1 FROM characters c
    WHERE c.id = sqlc.arg(character_id) AND c.owner_user_id = sqlc.arg(user_id) AND NOT c.retired
);

-- A character created in the app (ADR 0029 step 7.2): the entered values, its owner, and as the
-- sheet's level, THAC0 and saves the values the rules derive at its starting XP (ADR 0030 step
-- 24: a created character states no sheet, so it shows no difference), realized at that level with
-- its max HP for it (step 7.5). Its details start empty.
-- name: CreateCharacter :one
INSERT INTO characters (
    name, str, str_exceptional, dex, con, int, wis, cha, hp_max, psp_max, thac0_base, level, xp_total,
    save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
    class, race_code, owner_user_id, realized_level, skill_sets, proficiency_picks, psionic_picks
) VALUES (
    sqlc.arg(name), sqlc.arg(str), sqlc.arg(str_exceptional), sqlc.arg(dex), sqlc.arg(con), sqlc.arg(int), sqlc.arg(wis),
    sqlc.arg(cha), sqlc.arg(hp_max), sqlc.arg(psp_max), sqlc.arg(thac0_base), sqlc.arg(level), sqlc.arg(xp_total),
    sqlc.arg(save_para_poison), sqlc.arg(save_rod_staff_wand), sqlc.arg(save_petri_poly),
    sqlc.arg(save_breath), sqlc.arg(save_spell), sqlc.arg(class), sqlc.arg(race_code),
    sqlc.arg(owner_user_id), sqlc.arg(realized_level), sqlc.arg(skill_sets), sqlc.arg(proficiency_picks),
    sqlc.arg(psionic_picks)
) RETURNING id;

-- A new character's state rows: no spell prepared, no buff, round 0 (the seed character's
-- start, with nothing in it).
-- name: CreateSpellSlots :exec
INSERT INTO spell_slots (character_id, slots) VALUES ($1, '[]');

-- name: CreateActiveBuffs :exec
INSERT INTO active_buffs (character_id, buffs) VALUES ($1, '[]');

-- name: CreateRoundState :exec
INSERT INTO round_state (character_id, round_number) VALUES ($1, 0);

-- name: RenameCharacter :execrows
-- Only a character the user owns and has not retired: none updated is an unknown character.
UPDATE characters SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id) AND owner_user_id = sqlc.arg(owner_user_id) AND NOT retired;

-- name: RestoreSkillArmor :execrows
-- An undo or a redo of a skill armour change logged before ADR 0033 step 14 (the column is read by
-- nothing since; migration 020): the entry already ties it to the character.
UPDATE characters SET skill_armor = sqlc.arg(skill_armor) WHERE id = sqlc.arg(id);

-- name: RetireCharacter :execrows
-- Only a character the user owns and has not retired (ADR 0029 step 7.3): nothing is deleted.
UPDATE characters SET retired = TRUE
WHERE id = sqlc.arg(id) AND owner_user_id = sqlc.arg(owner_user_id) AND NOT retired;

-- name: RepairCurrentCharacter :exec
-- The user's first character they own and have not retired becomes current, unless the current
-- one is such a character (a retired current character is replaced; none left is NULL).
UPDATE users SET current_character_id = (
    SELECT min(c.id) FROM characters c WHERE c.owner_user_id = users.id AND NOT c.retired
) WHERE users.id = $1 AND NOT EXISTS (
    SELECT 1 FROM characters c
    WHERE c.id = users.current_character_id AND c.owner_user_id = users.id AND NOT c.retired
);

-- name: CurrentCharacter :one
-- The user's current character, read through the ownership: no row when it is none, retired or
-- not theirs.
SELECT c.id FROM users u
JOIN characters c ON c.id = u.current_character_id AND c.owner_user_id = u.id AND NOT c.retired
WHERE u.id = $1;

-- The AC override (ADR 0033 step 13), NULL for none.
-- name: GetACOverride :one
SELECT ac_override, ac_override_label FROM characters WHERE id = $1;

-- Setting, clearing (NULL and an empty label) and the undo of either.
-- name: SetACOverride :execrows
UPDATE characters SET ac_override = sqlc.narg(ac_override), ac_override_label = sqlc.arg(ac_override_label)
WHERE id = sqlc.arg(id);

-- ADR 0029 step 12: a copy of a character is its rows but the action log (and what hangs from it:
-- snapshots, corrections), under a new id. Each statement below copies one table's rows; the
-- service runs them in the one transaction.

-- name: CopyCharacterRow :one
-- The copy's own row: the source's sheet and picks, a new name and owner, not retired, with the
-- ledgers' opening balances. No row when the source is unknown, retired or owned by a disabled
-- user (the same rule as ViewableCharacter, in the one statement).
INSERT INTO characters (
    name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total, save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell, class, alignment, age, sex, hair, eyes, languages, race_code, realized_level, str_exceptional, skill_armor, height_cm, weight_kg, ac_override, ac_override_label, proficiency_picks, skill_sets, psp_max, psionic_picks, owner_user_id, retired, hp_opening, psp_opening
)
SELECT sqlc.arg(name), c.str, c.dex, c.con, c.int, c.wis, c.cha, c.hp_max, c.thac0_base, c.level, c.xp_total, c.save_para_poison, c.save_rod_staff_wand, c.save_petri_poly, c.save_breath, c.save_spell, c.class, c.alignment, c.age, c.sex, c.hair, c.eyes, c.languages, c.race_code, c.realized_level, c.str_exceptional, c.skill_armor, c.height_cm, c.weight_kg, c.ac_override, c.ac_override_label, c.proficiency_picks, c.skill_sets, c.psp_max, c.psionic_picks, sqlc.arg(owner_user_id), FALSE, sqlc.arg(hp_opening), sqlc.arg(psp_opening)
FROM characters c
JOIN users u ON u.id = c.owner_user_id AND NOT u.disabled
WHERE c.id = sqlc.arg(source_id) AND NOT c.retired
RETURNING id;

-- name: CopyClassEntries :exec
INSERT INTO character_classes (character_id, position, class, realized_level, starting_xp, state, left_at_level, ignored_awards_through)
SELECT sqlc.arg(target_id), position, class, realized_level, starting_xp, state, left_at_level, ignored_awards_through
FROM character_classes s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopySpellSlots :exec
INSERT INTO spell_slots (character_id, slots) SELECT sqlc.arg(target_id), slots FROM spell_slots s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopyActiveBuffs :exec
INSERT INTO active_buffs (character_id, buffs) SELECT sqlc.arg(target_id), buffs FROM active_buffs s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopyRoundState :exec
INSERT INTO round_state (character_id, round_number, initiative_d10, initiative_entry_id, initiative_modifiers)
SELECT sqlc.arg(target_id), round_number, initiative_d10, initiative_entry_id, initiative_modifiers
FROM round_state s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopySavedBuffs :exec
INSERT INTO saved_buffs (character_id, name, duration, charges, effects, note, created_at)
SELECT sqlc.arg(target_id), name, duration, charges, effects, note, created_at
FROM saved_buffs s WHERE s.character_id = sqlc.arg(source_id) ORDER BY s.id;

-- name: CopyAttackForm :exec
INSERT INTO attack_forms (character_id, weapon_id, previous_weapon_id, ammo_item_id, target_ac, target_size, situational_value, situational_label, target_ac_known)
SELECT sqlc.arg(target_id), weapon_id, previous_weapon_id, ammo_item_id, target_ac, target_size, situational_value, situational_label, target_ac_known
FROM attack_forms s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopyXPEntries :exec
INSERT INTO xp_entries (character_id, entry_date, session, reason, base, bonus)
SELECT sqlc.arg(target_id), entry_date, session, reason, base, bonus
FROM xp_entries s WHERE s.character_id = sqlc.arg(source_id) ORDER BY s.id;

-- name: CopyDiceFavourites :exec
INSERT INTO dice_favourites (character_id, formula, position)
SELECT sqlc.arg(target_id), formula, position FROM dice_favourites s WHERE s.character_id = sqlc.arg(source_id) ORDER BY s.id;

-- name: CopySpellbookEntries :exec
INSERT INTO spellbook_entries (character_id, spell_key, position, origin, grant_level)
SELECT sqlc.arg(target_id), spell_key, position, origin, grant_level
FROM spellbook_entries s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopyLearnFailures :exec
INSERT INTO spellbook_learn_failures (character_id, spell_key, level)
SELECT sqlc.arg(target_id), spell_key, level FROM spellbook_learn_failures s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopyEquipmentReview :exec
INSERT INTO equipment_review (character_id, item_id, reason)
SELECT sqlc.arg(target_id), item_id, reason FROM equipment_review s WHERE s.character_id = sqlc.arg(source_id);

-- name: CopyItems :exec
-- Every item keeps its id (they are keyed per character) and gets its own copy of its name: the
-- name's rows of localized_texts (ADR 0023 step 9) are deleted with the item, so two items never
-- share a key. The new keys follow the highest one, as the text adapter's NextTextKey does; the
-- writer lock keeps two copies from taking the same ones.
WITH src AS (
    SELECT i.*, row_number() OVER (ORDER BY i.position, i.id) AS n FROM items i WHERE i.character_id = sqlc.arg(source_id)
), base AS (
    SELECT COALESCE(MAX(text_id), 0)::bigint AS top FROM localized_texts
), names AS (
    INSERT INTO localized_texts (text_id, lang, body)
    SELECT base.top + src.n, lt.lang, lt.body FROM src CROSS JOIN base JOIN localized_texts lt ON lt.text_id = src.name
)
INSERT INTO items (id, name, character_id, position, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save, notes, powers, category, magic_kind, catalogue_code, component, state, weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit, magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind, ac_melee, ac_missile, spell_id, strength_rating, strength_rating_exceptional, abilities)
SELECT src.id, base.top + src.n, sqlc.arg(target_id), src.position, src.type, src.behaviour, src.magic_bonus, src.ammo_kind, src.count, src.max_count, src.effect_dice, src.effect_save, src.notes, src.powers, src.category, src.magic_kind, src.catalogue_code, src.component, src.state, src.weapon_type, src.weapon_kind, src.size, src.damage_types, src.speed, src.damage_sm, src.damage_l, src.magic_to_hit, src.magic_damage, src.two_handed, src.fires, src.specialized, src.base_ac, src.skill_column, src.armor_row, src.shield_kind, src.ac_melee, src.ac_missile, src.spell_id, src.strength_rating, src.strength_rating_exceptional, src.abilities
FROM src CROSS JOIN base;
