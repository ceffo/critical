-- name: GetCharacter :one
SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
       class, race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = ?;

-- name: GetFirstCharacterID :one
SELECT id FROM characters ORDER BY id LIMIT 1;

-- name: GetCharacterDetails :one
SELECT race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = ?;

-- name: UpdateCharacterDetails :execrows
UPDATE characters
SET race = ?, alignment = ?, age = ?, sex = ?, height = ?, weight = ?, hair = ?, eyes = ?, languages = ?
WHERE id = ?;
