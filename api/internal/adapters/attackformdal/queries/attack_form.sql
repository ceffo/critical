-- name: GetAttackForm :one
SELECT weapon_id, previous_weapon_id, ammo_item_id, target_ac, target_ac_known, target_size, situational_value, situational_label
FROM attack_forms WHERE character_id = $1;

-- name: UpsertAttackForm :exec
INSERT INTO attack_forms (character_id, weapon_id, previous_weapon_id, ammo_item_id, target_ac, target_ac_known, target_size, situational_value, situational_label)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(character_id) DO UPDATE SET
    weapon_id = excluded.weapon_id,
    previous_weapon_id = excluded.previous_weapon_id,
    ammo_item_id = excluded.ammo_item_id,
    target_ac = excluded.target_ac,
    target_ac_known = excluded.target_ac_known,
    target_size = excluded.target_size,
    situational_value = excluded.situational_value,
    situational_label = excluded.situational_label;
