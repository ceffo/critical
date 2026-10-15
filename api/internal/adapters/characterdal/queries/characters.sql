-- GetCharacter
SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell,
       class, race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = ?
;

-- GetFirstCharacterID
SELECT id FROM characters ORDER BY id LIMIT 1
;

-- GetCharacterDetails
SELECT class, race, alignment, age, sex, height, weight, hair, eyes, languages
FROM characters WHERE id = ?
;

-- UpdateCharacterDetails
UPDATE characters
SET race = ?, alignment = ?, age = ?, sex = ?, height = ?, weight = ?, hair = ?, eyes = ?, languages = ?
WHERE id = ?
RETURNING id
;
