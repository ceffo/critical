-- level, thac0_base and the save_* columns are the figures the imported sheet stated (ADR 0030
-- step 24): compared with the derived values, never read as inputs.
-- name: GetCharacter :one
SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
       class, race_code, race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = $1;

-- name: GetCharacterDetails :one
SELECT race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = $1;

-- name: UpdateCharacterDetails :execrows
UPDATE characters
SET race = $1, alignment = $2, age = $3, sex = $4, height = $5, weight = $6, hair = $7, eyes = $8, languages = $9
WHERE id = $10;

-- name: CharactersOf :many
-- The owner's characters they have not retired, in creation order (the service derives each level).
SELECT id, name, class FROM characters
WHERE owner_user_id = $1 AND NOT retired
ORDER BY id;

-- name: SetCurrentCharacter :execrows
-- Only a character the user owns and has not retired: none updated is an unknown character.
UPDATE users SET current_character_id = sqlc.arg(character_id)
WHERE users.id = sqlc.arg(user_id) AND EXISTS (
    SELECT 1 FROM characters c
    WHERE c.id = sqlc.arg(character_id) AND c.owner_user_id = sqlc.arg(user_id) AND NOT c.retired
);
