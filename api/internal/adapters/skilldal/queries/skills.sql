-- name: ListProficiencies :many
SELECT id, ability, modifier FROM proficiencies ORDER BY position, id;

-- name: ListBardSkills :many
SELECT id, percent FROM bard_skills ORDER BY position, id;
