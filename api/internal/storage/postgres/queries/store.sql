-- Writer lock (ADR 0025 step 4): every write transaction takes it first; the commit or the
-- rollback releases it. One fixed key for the whole database.
-- name: LockWriters :exec
SELECT pg_advisory_xact_lock(7240531);

-- name: CountCharacters :one
SELECT COUNT(*) FROM characters;

-- name: InsertCharacter :one
INSERT INTO characters (
    name, str, dex, con, int, wis, cha, hp_max, thac0_base, level, xp_total,
    save_para_poison, save_rod_staff_wand, save_petri_poly, save_breath, save_spell, languages
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING id;

-- name: InsertSpellSlots :exec
INSERT INTO spell_slots (character_id, slots) VALUES ($1, $2);

-- name: InsertActiveBuffs :exec
INSERT INTO active_buffs (character_id, buffs) VALUES ($1, '[]');

-- name: InsertRoundState :exec
INSERT INTO round_state (character_id, round_number) VALUES ($1, 0);

-- name: CountWeapons :one
SELECT COUNT(*) FROM weapons;

-- name: CountItems :one
SELECT COUNT(*) FROM items;

-- name: CountSkills :one
SELECT ((SELECT COUNT(*) FROM proficiencies) + (SELECT COUNT(*) FROM bard_skills))::bigint AS count;

-- name: InsertWeapon :exec
INSERT INTO weapons (id, position, name, damage_sm, damage_l, magic_to_hit, magic_damage, kind, two_handed, ammo_kind)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: InsertItem :exec
INSERT INTO items (id, position, name, type, behaviour, magic_bonus, ammo_kind, count, max_count, effect_dice,
                   effect_save, notes, powers)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: WeaponNameKey :one
SELECT name FROM weapons WHERE id = $1;

-- name: ItemNameKey :one
SELECT name FROM items WHERE id = $1;

-- name: SeedOffers :many
SELECT lang FROM seeded_name_languages WHERE kind = $1 AND id = $2;

-- name: RecordSeedOffer :exec
INSERT INTO seeded_name_languages (kind, id, lang) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: UpsertProficiency :exec
INSERT INTO proficiencies (id, position, ability, modifier) VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET position = excluded.position, ability = excluded.ability, modifier = excluded.modifier;

-- name: UpsertBardSkill :exec
INSERT INTO bard_skills (id, position, percent) VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET position = excluded.position, percent = excluded.percent;

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

-- name: SnapshotsOf :many
SELECT id, before_state, after_state FROM action_snapshots WHERE state_key = $1 ORDER BY id;

-- name: SpellEntryPayloads :many
SELECT id, payload FROM action_log WHERE kind = ANY(sqlc.arg('kinds')::text[]) ORDER BY id;

-- name: SpellCorrectionPayloads :many
SELECT c.id, COALESCE(c.payload, '')::text AS payload
FROM action_corrections c JOIN action_log l ON l.id = c.action_id
WHERE l.kind = ANY(sqlc.arg('kinds')::text[]) ORDER BY c.id;
