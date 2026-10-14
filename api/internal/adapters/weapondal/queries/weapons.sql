-- ListWeapons
SELECT id, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons ORDER BY position
;

-- GetWeapon
SELECT id, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons WHERE id = ?
;

-- ListWeaponsFull
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons ORDER BY position
;

-- GetWeaponFull
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons WHERE id = ?
;

-- InsertWeapon
INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
;

-- UpdateWeapon
UPDATE weapons SET position = ?, name = ?, damage_sm = ?, damage_l = ?, magic_to_hit = ?, magic_damage = ?, kind = ?, two_handed = ?, ammo_kind = ?
WHERE id = ?
RETURNING id
;

-- DeleteWeapon
DELETE FROM weapons WHERE id = ?
RETURNING id
;
