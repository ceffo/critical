-- name: ListWeapons :many
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons ORDER BY position;

-- name: GetWeapon :one
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons WHERE id = $1;

-- name: InsertWeapon :exec
INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateWeapon :one
UPDATE weapons SET position = $1, damage_sm = $2, damage_l = $3, magic_to_hit = $4, magic_damage = $5, kind = $6, two_handed = $7, ammo_kind = $8
WHERE id = $9
RETURNING name;

-- name: DeleteWeapon :one
DELETE FROM weapons WHERE id = $1
RETURNING name;

-- name: WeaponNames :many
SELECT text_id, lang, body FROM localized_texts WHERE text_id IN (SELECT name FROM weapons);
