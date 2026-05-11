-- GetCharacter
SELECT id, name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
       save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell
FROM characters WHERE id = ?
;

-- GetFirstCharacterID
SELECT id FROM characters ORDER BY id LIMIT 1
;
