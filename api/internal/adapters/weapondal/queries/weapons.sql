-- ListWeapons
SELECT id, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons ORDER BY position
;

-- GetWeapon
SELECT id, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons WHERE id = ?
;
