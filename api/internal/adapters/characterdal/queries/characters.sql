-- name: GetCharacter :one
SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
       class, race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = $1;

-- name: GetFirstCharacterID :one
SELECT id FROM characters ORDER BY id LIMIT 1;

-- name: GetCharacterDetails :one
SELECT race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = $1;

-- name: UpdateCharacterDetails :execrows
UPDATE characters
SET race = $1, alignment = $2, age = $3, sex = $4, height = $5, weight = $6, hair = $7, eyes = $8, languages = $9
WHERE id = $10;
