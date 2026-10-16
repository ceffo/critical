# ADR 0023: Localized text: one row per language, no per-language field

**Status**: accepted, 2026-10-15 (critical-sikx). Amends [ADR 0021](0021-spell-catalogue.md).

## Context

User, 2026-10-15: _"we have ZERO localized text in the backend nor in the database. what is going on?
why this cardinal rule keeps being overriden? hunt and eliminate contradictions"_, then, once the data
layout was explained: _"it's ok for the contents to be in the database but i would do it differently.
name and description do not hold the text but they hold a foreign key to localization table where you
have one row per language"_.

The rule was never written down. The hard rules only forbade composing text in Go, ADR 0021 prescribed
`name_fr` / `description_fr` columns and fields, and the review checklist said stored book text in a
language is presentation text. Agents followed those and overrode the rule twice (the French stat
columns of #147, then `name_fr` / `description_fr`).

## Decision

1. **No column, field or property per language, anywhere.** Not in the schema, the Go types, the API
   payloads, the TypeScript types, nor the catalogue artifact (where a text is a language map, never
   `name_fr`-style keys). Adding a language adds rows, never columns, DTO fields or code paths.
2. **A text in a human language is a key into a localization table**: `localized_texts`, one row per
   `(text_id, lang)`. The owning column (`name`, `description`, …) holds the key, not the text.
3. **English is a language like any other**: its texts are `lang='en'` rows. `en` is the fallback
   language.
4. **Every human-language text column of reference data goes through the table.** For spells: `name`,
   `description`, `school`, `sphere` and the six stat texts (`casting_time` … `saving_throw`).
5. **One generic table for all reference data** (items and weapons later), not one per entity.
6. **The API serves a language map per text field**, e.g. `"name": {"en": "…", "fr": "…"}`. The client
   picks the language and falls back per field. There is no `lang` query parameter.
7. **Server-side name search stays** and matches the name rows of every language (language-generic: no
   language is named in the query).
8. **Mechanical enforcement**: `just validate` runs a check that refuses a per-language column, field or
   key.

## Consequences

- Still allowed: error `message` strings (already excepted by the hard rules); text the player typed
  (character languages, notes, the log `summary`); source-language names recorded in journal payloads at
  write time (unchanged, payloads are not rewritten).
- Search cost: search joins only the name rows, by key through the primary-key index, so a query never
  scans descriptions. The catalogue is about 18k text rows. If it ever grows, an index on folded names is
  the next step, not a materialized view.
- UI wording of typed values (stats, units, buff names) stays at render in the frontend, as
  [ADR 0021](0021-spell-catalogue.md) step 8 says.
- Follow-ups, not part of this ADR: the schema, API and client change (critical-v3r5, implemented
  below) and the check of step 8 (critical-s6z0).

## Implementation (spells, migration 031)

- **Table and keys** (steps 2, 4, 5): `localized_texts (text_id, lang, body)`, primary key
  `(text_id, lang)`, `WITHOUT ROWID`, `body` never empty. The ten spell text columns hold the key;
  a NULL key is a text in no language. `text_id` is not unique alone, so there is no `FOREIGN KEY`
  clause; the invariant "a key has at least one row, every row is keyed" is checked by the importer
  and migration tests (`assertTextIntegrity`). Migration 031 moves every existing text (`en`, and `fr`
  from `name_fr` / `description_fr`), rebuilds `spells` with integer text columns keeping every id,
  key and the id sequence, and forgets `catalogue_meta` so the next startup imports again
  (`TestMigrate031_MovesEveryTextToLocalizedTextsKeepingEveryKey`,
  `TestMigrate031_ThenImportKeepsRowsAndTextKeys`, `TestMigrate031_FreshDatabaseHasLocalizedTexts`;
  a live database: `TestRealCatalogue_UpgradesACopyOfALiveDatabase`).
- **Import** (step 3, no empty row): one row per language present, none for an absent one; a
  language or a whole text the artifact drops loses its row(s), a removed spell its texts
  (`TestImportCatalogue_FreshDatabase`, `TestImportCatalogue_Step4_NewVersionUpserts`,
  `TestImportCatalogue_ReimportDropsAndAddsWholeTexts`). The artifact format is unchanged: its
  plain stat, school and sphere texts are the English books' (`TestLoad_Fixture`).
- **Go** (step 1): `localized.Text` (`api/internal/domain/localized`), a map from language code to
  body; `localized.Fallback` is `en` (step 3). No `*FR` field remains.
- **API** (step 6): every text field is a language map, `{}` for a text in none
  (`TestSearch_ServesTextFieldsAsLanguageMaps`, `TestNames_ServesIDsWithNameLanguageMaps`,
  `TestSpells_TextsAreLanguageMapsFromTheCatalogue`).
- **Search** (step 7 and the cost note): the name tiers run over every language's name
  (`TestSearch_RankingTiers`); the text lookup goes through the primary key, never a scan
  (`TestTextsQuery_SearchesThePrimaryKey`), and only name, school and sphere are read for the
  candidates; a page's full texts are one batched read (`TestSearch_ReadsThePageTextsInOneBatch`).
- **Client**: `LocalizedText` and `localized(lang, text)` (per-field English fallback;
  `localized.test.ts`, `spellName.test.ts`, `statText.test.ts`; the French and English lookup e2e).
