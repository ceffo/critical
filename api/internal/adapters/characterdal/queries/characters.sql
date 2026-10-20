-- level, thac0_base and the save_* columns are the figures the imported sheet stated (ADR 0030
-- step 24): compared with the derived values, never read as inputs.
-- realized_level is the level the character has taken (ADR 0030 step 7); NULL is the sheet's level
-- (step 7.5), which stays the imported figure step 24 compares.
-- name: GetCharacter :one
SELECT id, name, str, str_exceptional, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
       class, race_code, alignment, age, sex, height_cm, weight_kg, hair, eyes, languages,
       COALESCE(realized_level, level)::BIGINT AS realized_level, skill_points, proficiency_picks
FROM characters WHERE id = $1;

-- A level-up roll and its undo (ADR 0030 step 7.3): the realized level and the max HP.
-- name: SetRealizedLevel :execrows
UPDATE characters SET realized_level = sqlc.arg(realized_level), hp_max = sqlc.arg(hp_max)
WHERE id = sqlc.arg(id);

-- A thief's skill points spent on the sheet and their undo (ADR 0030 step 17).
-- name: SetSkillPoints :execrows
UPDATE characters SET skill_points = sqlc.arg(skill_points) WHERE id = sqlc.arg(id);

-- The proficiency picks saved by the editor and their undo (ADR 0037 steps 12 and 13).
-- name: SetProficiencyPicks :execrows
UPDATE characters SET proficiency_picks = sqlc.arg(proficiency_picks) WHERE id = sqlc.arg(id);

-- name: GetCharacterDetails :one
SELECT class, race_code, alignment, age, sex, height_cm, weight_kg, hair, eyes, languages
FROM characters WHERE id = $1;

-- name: UpdateCharacterDetails :execrows
UPDATE characters
SET alignment = $1, age = $2, sex = $3, height_cm = $4, weight_kg = $5, hair = $6, eyes = $7, languages = $8
WHERE id = $9;

-- name: CharactersOf :many
-- The owner's characters they have not retired, in creation order (the service derives each level).
SELECT id, name, class FROM characters
WHERE owner_user_id = $1 AND NOT retired
ORDER BY id;

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
    name, str, str_exceptional, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
    save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
    class, race_code, owner_user_id, realized_level, skill_points, proficiency_picks
) VALUES (
    sqlc.arg(name), sqlc.arg(str), sqlc.arg(str_exceptional), sqlc.arg(dex), sqlc.arg(con), sqlc.arg(int), sqlc.arg(wis),
    sqlc.arg(cha), sqlc.arg(hp_max), sqlc.arg(thac0_base), sqlc.arg(level), sqlc.arg(xp_total),
    sqlc.arg(save_para_poison), sqlc.arg(save_rod_staff_wand), sqlc.arg(save_petri_poly),
    sqlc.arg(save_breath), sqlc.arg(save_spell), sqlc.arg(class), sqlc.arg(race_code),
    sqlc.arg(owner_user_id), sqlc.arg(realized_level), sqlc.arg(skill_points), sqlc.arg(proficiency_picks)
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
