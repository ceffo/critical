-- Writer lock (ADR 0025 step 4): every write transaction takes it first; the commit or the
-- rollback releases it. One fixed key for the whole database.
-- name: LockWriters :exec
SELECT pg_advisory_xact_lock(7240531);

-- name: CountCharacters :one
SELECT COUNT(*) FROM characters;

-- The sheet's level, THAC0 and saves are the figures the imported sheet states (ADR 0030 step 24),
-- compared with the derived values, never read as inputs.
-- name: InsertCharacter :one
INSERT INTO characters (
    name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
    save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell, languages, race_code,
    skill_points, alignment, sex
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
RETURNING id;

-- The race of a character stored before races had codes (migration 007): only while it has none,
-- so a rerun changes nothing and a race set since is kept.
-- name: SetRaceIfUnset :exec
UPDATE characters SET race_code = sqlc.arg(race_code) WHERE id = sqlc.arg(id) AND race_code = '';

-- The identity migration (013): every character's stored alignment and sex, to map to codes.
-- name: ListCharacterIdentity :many
SELECT id, alignment, sex FROM characters ORDER BY id;

-- The mapped (or cleared) alignment and sex of a character.
-- name: SetCharacterIdentity :exec
UPDATE characters SET alignment = sqlc.arg(alignment), sex = sqlc.arg(sex) WHERE id = sqlc.arg(id);

-- The identity numbers migration (015): every character's stored free-text height and weight,
-- to parse into centimetres and kilograms.
-- name: ListCharacterMeasures :many
SELECT id, height, weight FROM characters ORDER BY id;

-- The parsed height and weight of a character (0: none, the text was not recognised).
-- name: SetCharacterMeasures :exec
UPDATE characters SET height_cm = sqlc.arg(height_cm), weight_kg = sqlc.arg(weight_kg) WHERE id = sqlc.arg(id);

-- The languages migration (014): every character's stored language list, to map to codes.
-- name: ListCharacterLanguages :many
SELECT id, languages FROM characters ORDER BY id;

-- The mapped language list of a character (a JSON list of codes).
-- name: SetCharacterLanguages :exec
UPDATE characters SET languages = sqlc.arg(languages) WHERE id = sqlc.arg(id);

-- The seed character's alignment and sex when it lacks them (migration 013): a value set since is
-- kept, so a rerun changes nothing.
-- name: SetIdentityIfUnset :exec
UPDATE characters
SET alignment = CASE WHEN alignment = '' THEN sqlc.arg(alignment) ELSE alignment END,
    sex = CASE WHEN sex = '' THEN sqlc.arg(sex) ELSE sex END
WHERE id = sqlc.arg(id);

-- The skill points of a character stored before bards entered them (migration 012): only while it
-- has none, so a rerun changes nothing and points spent since are kept.
-- name: SetSkillPointsIfUnset :exec
UPDATE characters SET skill_points = sqlc.arg(skill_points) WHERE id = sqlc.arg(id) AND skill_points = '{}';

-- name: InsertSpellSlots :exec
INSERT INTO spell_slots (character_id, slots) VALUES ($1, $2);

-- name: InsertActiveBuffs :exec
INSERT INTO active_buffs (character_id, buffs) VALUES ($1, '[]');

-- name: InsertRoundState :exec
INSERT INTO round_state (character_id, round_number) VALUES ($1, 0);

-- The character the seed files belong to: the one named in character.json (the lowest id when
-- two share the name).
-- name: SeedCharacterID :one
SELECT id FROM characters WHERE name = $1 ORDER BY id LIMIT 1;

-- Rows from before each character owned its equipment (migration 002), or written by a release
-- without it during a rolling deploy, have no owner: the migration run gives them to the seed
-- character just before migration 004 makes the owner required (ownRows). Afterwards none exist.
-- These queries are typed against the latest schema but run on the schema before 004: a migration
-- that changes the columns they touch deletes them with ownRows instead, once every environment is
-- past 004 (ADR 0025 step 2.1.1).
-- name: CountUnowned :one
SELECT ((SELECT COUNT(*) FROM weapons WHERE character_id IS NULL)
      + (SELECT COUNT(*) FROM items WHERE character_id IS NULL)
      + (SELECT COUNT(*) FROM proficiencies WHERE character_id IS NULL)
      + (SELECT COUNT(*) FROM bard_skills WHERE character_id IS NULL))::bigint AS count;

-- name: AdoptWeapons :exec
UPDATE weapons SET character_id = sqlc.arg(character_id)::bigint WHERE character_id IS NULL;

-- name: AdoptItems :exec
UPDATE items SET character_id = sqlc.arg(character_id)::bigint WHERE character_id IS NULL;

-- name: AdoptProficiencies :exec
UPDATE proficiencies SET character_id = sqlc.arg(character_id)::bigint WHERE character_id IS NULL;

-- name: AdoptBardSkills :exec
UPDATE bard_skills SET character_id = sqlc.arg(character_id)::bigint WHERE character_id IS NULL;

-- The weapons stored before migration 005 have no type: the data step run just after it gives the
-- seeded ones theirs, never replacing a type already set (typeSeedWeapons). Delete it with its step
-- once every environment is past 005 (ADR 0025 step 2.1.1).
-- name: TypeWeaponIfUntyped :exec
UPDATE weapons SET weapon_type = @weapon_type WHERE id = @id AND weapon_type = '';

-- name: CountItems :one
SELECT COUNT(*) FROM items WHERE character_id = sqlc.arg(character_id)::bigint;

-- name: CountSkills :one
SELECT ((SELECT COUNT(*) FROM proficiencies p WHERE p.character_id = sqlc.arg(character_id)::bigint)
      + (SELECT COUNT(*) FROM bard_skills b WHERE b.character_id = sqlc.arg(character_id)::bigint))::bigint AS count;

-- A piece of equipment (ADR 0033 step 1), every column: the seed and the conversion write it.
-- name: InsertItem :exec
INSERT INTO items (character_id, id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count,
                   effect_dice, effect_save, notes, powers, category, magic_kind, catalogue_code, component,
                   state, weapon_type, weapon_kind, size, damage_types, speed, damage_sm, damage_l, magic_to_hit,
                   magic_damage, two_handed, fires, specialized, base_ac, skill_column, armor_row, shield_kind,
                   ac_melee, ac_missile)
VALUES (sqlc.arg(character_id)::bigint, @id, @position, @name, @type, @behaviour, @magic_bonus, @ammo_kind, @count,
        @max_count, @effect_dice, @effect_save, @notes, @powers, @category, @magic_kind, @catalogue_code, @component,
        @state, @weapon_type, @weapon_kind, @size, @damage_types, @speed, @damage_sm, @damage_l, @magic_to_hit,
        @magic_damage, @two_handed, @fires, @specialized, @base_ac, @skill_column, @armor_row, @shield_kind,
        @ac_melee, @ac_missile);

-- name: ItemNameKey :one
SELECT name FROM items WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;

-- name: InsertReview :exec
INSERT INTO equipment_review (character_id, item_id, reason) VALUES (sqlc.arg(character_id)::bigint, @item_id, @reason)
ON CONFLICT DO NOTHING;

-- The conversion of migration 016 (ADR 0033 step 26, convertEquipment): every character that holds
-- items or weapons, their rows as stored before it, and the item rows rewritten as pieces. Delete
-- them with their step once every environment is past 016 (ADR 0025 step 2.1.1).
-- name: EquipmentOwners :many
SELECT character_id FROM items UNION SELECT character_id FROM weapons ORDER BY character_id;

-- name: ListLegacyItems :many
SELECT id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice, effect_save,
       notes, powers
FROM items WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position, id;

-- name: ListLegacyWeapons :many
SELECT id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind,
       weapon_type, specialized
FROM weapons WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position, id;

-- name: DeleteCharacterItems :exec
DELETE FROM items WHERE character_id = sqlc.arg(character_id)::bigint;

-- The ids the action log still holds item and weapon snapshots of (the conversion of migration 016
-- keeps an item and a weapon of one id apart even when one of them is only in the log).
-- name: LegacySnapshotKeys :many
SELECT DISTINCT s.state_key::text AS state_key
FROM action_snapshots s JOIN action_log a ON a.id = s.action_id
WHERE a.character_id = sqlc.arg(character_id)::bigint
  AND (s.state_key LIKE 'items:%' OR s.state_key LIKE 'weapons:%');

-- The chosen ammunition follows an item the conversion gave a new id.
-- name: RenameAttackFormAmmo :exec
UPDATE attack_forms SET ammo_item_id = @new_id
WHERE character_id = sqlc.arg(character_id)::bigint AND ammo_item_id = @old_id;

-- The snapshots stored before the codec's envelopes (migration 019), with their character, for the
-- re-encoding (reencodeSnapshots, ADR 0018 step 9).
-- name: SnapshotsToReencode :many
SELECT s.id, a.character_id, s.state_key, s.before_state, s.after_state
FROM action_snapshots s JOIN action_log a ON a.id = s.action_id
WHERE s.before_envelope IS NULL OR s.after_envelope IS NULL
ORDER BY s.id;

-- name: SetSnapshotEnvelopes :exec
UPDATE action_snapshots
SET before_envelope = sqlc.arg(before_envelope)::text, after_envelope = sqlc.arg(after_envelope)::text
WHERE id = @id;

-- name: InsertItemRename :exec
INSERT INTO item_id_renames (character_id, old_id, new_id) VALUES (sqlc.arg(character_id)::bigint, @old_id, @new_id);

-- The id the conversion gave an item that shared its id with a weapon; no row when it kept its id.
-- name: RenamedItemID :one
SELECT new_id FROM item_id_renames WHERE character_id = sqlc.arg(character_id)::bigint AND old_id = @old_id;

-- name: SeedOffers :many
SELECT lang FROM seeded_name_languages WHERE kind = $1 AND id = $2;

-- name: RecordSeedOffer :exec
INSERT INTO seeded_name_languages (kind, id, lang) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: UpsertProficiency :exec
INSERT INTO proficiencies (character_id, id, position, ability, modifier)
VALUES (sqlc.arg(character_id)::bigint, @id, @position, @ability, @modifier)
ON CONFLICT (character_id, id) DO UPDATE SET position = excluded.position, ability = excluded.ability,
                                             modifier = excluded.modifier;

-- name: UpsertBardSkill :exec
INSERT INTO bard_skills (character_id, id, position, percent) VALUES (sqlc.arg(character_id)::bigint, @id, @position, @percent)
ON CONFLICT (character_id, id) DO UPDATE SET position = excluded.position, percent = excluded.percent;

-- name: CountSpellbookEntries :one
SELECT COUNT(*) FROM spellbook_entries WHERE character_id = $1;

-- name: InsertSpellbookEntry :exec
INSERT INTO spellbook_entries (character_id, spell_key, position) VALUES ($1, $2, $3);

-- name: NextTextKey :one
SELECT (COALESCE(MAX(text_id), 0) + 1)::bigint AS next_key FROM localized_texts;

-- name: TextLanguages :many
SELECT lang, body FROM localized_texts WHERE text_id = $1;

-- name: AllTexts :many
SELECT text_id, lang, body FROM localized_texts;

-- name: InsertText :exec
INSERT INTO localized_texts (text_id, lang, body) VALUES ($1, $2, $3);

-- name: UpsertText :exec
INSERT INTO localized_texts (text_id, lang, body) VALUES ($1, $2, $3)
ON CONFLICT (text_id, lang) DO UPDATE SET body = excluded.body;

-- name: DeleteText :exec
DELETE FROM localized_texts WHERE text_id = $1;

-- name: DeleteTextLanguage :exec
DELETE FROM localized_texts WHERE text_id = $1 AND lang = $2;

-- name: CatalogueVersion :one
SELECT format, version FROM catalogue_meta WHERE id = 1;

-- name: RecordCatalogueVersion :exec
INSERT INTO catalogue_meta (id, format, version) VALUES (1, $1, $2)
ON CONFLICT (id) DO UPDATE SET format = excluded.format, version = excluded.version;

-- name: CountKeyedSpells :one
SELECT COUNT(*) FROM spells WHERE key IS NOT NULL;

-- name: SpellIdentities :many
SELECT s.id, s.key, COALESCE(t.body, '')::text AS name, s.class, s.level
FROM spells s LEFT JOIN localized_texts t ON t.text_id = s.name AND t.lang = $1
ORDER BY s.id;

-- name: SetSpellKey :exec
UPDATE spells SET key = $1 WHERE id = $2;

-- name: SpellTextKeys :many
SELECT key::text AS key, name, description, school, sphere, casting_time, range, duration, area_of_effect,
       components, saving_throw
FROM spells WHERE key IS NOT NULL;

-- name: UpsertSpell :exec
INSERT INTO spells (key, class, level, source, stats, name, description, school, sphere, casting_time, range,
                    duration, area_of_effect, components, saving_throw)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (key) DO UPDATE SET
    class = excluded.class, level = excluded.level, source = excluded.source, stats = excluded.stats,
    name = excluded.name, description = excluded.description, school = excluded.school,
    sphere = excluded.sphere, casting_time = excluded.casting_time, range = excluded.range,
    duration = excluded.duration, area_of_effect = excluded.area_of_effect,
    components = excluded.components, saving_throw = excluded.saving_throw;

-- name: DeleteSpell :exec
DELETE FROM spells WHERE key = $1;

-- name: SpellStatTexts :many
SELECT s.id, COALESCE(s.stats, '')::text AS stats,
       COALESCE((SELECT lt.body FROM localized_texts lt WHERE lt.text_id = s.casting_time AND lt.lang = $1), '')::text AS casting_time,
       COALESCE((SELECT lt.body FROM localized_texts lt WHERE lt.text_id = s.range AND lt.lang = $1), '')::text AS range,
       COALESCE((SELECT lt.body FROM localized_texts lt WHERE lt.text_id = s.duration AND lt.lang = $1), '')::text AS duration,
       COALESCE((SELECT lt.body FROM localized_texts lt WHERE lt.text_id = s.area_of_effect AND lt.lang = $1), '')::text AS area_of_effect,
       COALESCE((SELECT lt.body FROM localized_texts lt WHERE lt.text_id = s.components AND lt.lang = $1), '')::text AS components,
       COALESCE((SELECT lt.body FROM localized_texts lt WHERE lt.text_id = s.saving_throw AND lt.lang = $1), '')::text AS saving_throw
FROM spells s ORDER BY s.id;

-- name: SetSpellStats :exec
UPDATE spells SET stats = $1 WHERE id = $2;

-- name: SpellSlotLists :many
SELECT character_id, slots FROM spell_slots ORDER BY character_id;

-- name: ActiveBuffLists :many
SELECT character_id, buffs FROM active_buffs ORDER BY character_id;

-- name: SpellbookRefs :many
SELECT character_id, spell_key FROM spellbook_entries ORDER BY character_id, position, spell_key;

-- name: SnapshotsOf :many
SELECT id, before_envelope, after_envelope FROM action_snapshots WHERE state_key = $1 ORDER BY id;

-- name: SnapshotsWithPrefix :many
SELECT id, state_key, before_envelope, after_envelope FROM action_snapshots
WHERE state_key LIKE sqlc.arg('prefix')::text || '%' ORDER BY id;

-- name: LearnFailureRefs :many
SELECT character_id, spell_key FROM spellbook_learn_failures ORDER BY character_id, spell_key;

-- name: SpellEntryPayloads :many
SELECT id, payload FROM action_log WHERE kind = ANY(sqlc.arg('kinds')::text[]) ORDER BY id;

-- name: SpellCorrectionPayloads :many
SELECT c.id, COALESCE(c.payload, '')::text AS payload
FROM action_corrections c JOIN action_log l ON l.id = c.action_id
WHERE l.kind = ANY(sqlc.arg('kinds')::text[]) ORDER BY c.id;

-- The skill armour carry-over of migration 020 (ADR 0033 step 14, carrySkillArmour): each
-- character's class and entered skill armour, all its items as the carry-over reads them (every
-- item, armour or not: an added piece goes after the last position), and a carried piece put on. Delete them with their step once every environment is past 020 (ADR 0025
-- step 2.1.1); the seed keeps CharacterSkillArmour, CarryOverItems and WearItem.
-- name: SkillArmourCharacters :many
SELECT id, class, skill_armor FROM characters ORDER BY id;

-- name: CharacterSkillArmour :one
SELECT class, skill_armor FROM characters WHERE id = sqlc.arg(id)::bigint;

-- name: CarryOverItems :many
SELECT id, position, category, state, skill_column
FROM items WHERE character_id = sqlc.arg(character_id)::bigint ORDER BY position, id;

-- name: WearItem :exec
UPDATE items SET state = 'worn' WHERE character_id = sqlc.arg(character_id)::bigint AND id = @id;
