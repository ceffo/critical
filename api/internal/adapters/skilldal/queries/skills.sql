-- Proficiencies and bard skills belong to a character (character_id).

-- name: ListProficiencies :many
SELECT id, ability, modifier FROM proficiencies WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position, id;

-- name: ListBardSkills :many
SELECT id, percent FROM bard_skills WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position, id;
