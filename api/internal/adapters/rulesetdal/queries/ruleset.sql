-- name: RulesetByName :one
SELECT id, name, catalogue_version, crit_placeholder, multiclass_any_race, dualclass_any_race, level_limits
FROM rulesets WHERE name = $1;

-- name: ListRulesetCritRows :many
SELECT roll_from, roll_to, multiplier
FROM ruleset_crit_rows WHERE ruleset_id = $1 ORDER BY roll_from;

-- name: ListRulesetSuccessEffects :many
SELECT proficiency_id, ac, rounds
FROM ruleset_success_effects WHERE ruleset_id = $1 ORDER BY proficiency_id;

-- name: DeleteRulesetCritRows :exec
DELETE FROM ruleset_crit_rows WHERE ruleset_id = $1;

-- name: InsertRulesetCritRow :exec
INSERT INTO ruleset_crit_rows (ruleset_id, roll_from, roll_to, multiplier)
VALUES ($1, $2, $3, $4);

-- name: SetRulesetCritPlaceholder :exec
UPDATE rulesets SET crit_placeholder = $2 WHERE id = $1;

-- name: SetRulesetRacialLift :exec
UPDATE rulesets SET multiclass_any_race = $2, dualclass_any_race = $3 WHERE id = $1;

-- name: DeleteRulesetSuccessEffects :exec
DELETE FROM ruleset_success_effects WHERE ruleset_id = $1;

-- name: InsertRulesetSuccessEffect :exec
INSERT INTO ruleset_success_effects (ruleset_id, proficiency_id, ac, rounds)
VALUES ($1, $2, $3, $4);

-- name: SetRulesetCatalogueVersion :exec
UPDATE rulesets SET catalogue_version = $2 WHERE id = $1;

-- name: InsertRulesetChange :exec
INSERT INTO ruleset_changes (ruleset_id, changed_by, rule, before_value, after_value)
VALUES ($1, $2, $3, $4, $5);

-- name: ListRulesetChanges :many
SELECT id, ruleset_id, changed_at, changed_by, rule, before_value, after_value
FROM ruleset_changes WHERE ruleset_id = $1 ORDER BY id DESC;
