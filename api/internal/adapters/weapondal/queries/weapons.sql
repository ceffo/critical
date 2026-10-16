-- name: ListWeapons :many
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons ORDER BY position;

-- name: GetWeapon :one
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind
FROM weapons WHERE id = ?;

-- name: InsertWeapon :exec
INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateWeapon :one
UPDATE weapons SET position = ?, damage_sm = ?, damage_l = ?, magic_to_hit = ?, magic_damage = ?, kind = ?, two_handed = ?, ammo_kind = ?
WHERE id = ?
RETURNING name;

-- name: DeleteWeapon :one
DELETE FROM weapons WHERE id = ?
RETURNING name;

-- name: WeaponNames :many
SELECT text_id, lang, body FROM localized_texts WHERE text_id IN (SELECT name FROM weapons);
