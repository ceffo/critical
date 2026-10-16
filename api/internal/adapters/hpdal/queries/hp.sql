-- name: GetCharacterMaxHP :one
SELECT hp_max FROM characters WHERE id = ?;
