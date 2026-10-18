-- Every query is one character's: weapons belong to a character (character_id). The selected
-- columns are the whole row, so sqlc maps them to the Weapon model.

-- name: ListWeapons :many
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind, character_id
FROM weapons WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position;

-- name: GetWeapon :one
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind, character_id
FROM weapons WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;

-- name: InsertWeapon :exec
INSERT INTO weapons (character_id, id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind,
                     two_handed, ammo_kind)
VALUES (sqlc.arg(character_id)::bigint, @id, @position, @name, @damage_sm, @damage_l, @magic_to_hit, @magic_damage,
        @kind, @two_handed, @ammo_kind);

-- name: UpdateWeapon :one
UPDATE weapons SET position = @position, damage_sm = @damage_sm, damage_l = @damage_l, magic_to_hit = @magic_to_hit,
                   magic_damage = @magic_damage, kind = @kind, two_handed = @two_handed, ammo_kind = @ammo_kind
WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id
RETURNING name;

-- name: DeleteWeapon :one
DELETE FROM weapons WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id
RETURNING name;

-- name: WeaponNames :many
SELECT text_id, lang, body FROM localized_texts
WHERE text_id IN (SELECT name FROM weapons WHERE character_id = sqlc.arg(character_id)::bigint);
