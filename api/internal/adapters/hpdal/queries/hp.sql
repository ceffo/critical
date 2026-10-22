-- name: GetCharacterMaxHP :one
SELECT hp_max FROM characters WHERE id = $1;

-- name: GetCharacterMaxPSP :one
SELECT psp_max FROM characters WHERE id = $1;
