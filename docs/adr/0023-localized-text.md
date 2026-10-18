# ADR 0023: Localized text: one row per language, no per-language field

**Status**: accepted, 2026-10-15 (critical-sikx). Amends [ADR 0021](0021-spell-catalogue.md).

_Proposed amendment by [ADR 0033](0033-equipment.md) (equipment, proposed 2026-10-18), in effect once it is accepted: steps 9 to 12 (item and weapon names become equipment names, one kind, ADR 0033 step 1; the rules are unchanged)._

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
  (a past migration: the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3; the PostgreSQL baseline holds the schema it built;
  the live-database check of the time was retired with the Go tests that open a database, [ADR 0025](0025-postgresql-storage.md) step 6).
- **Import** (step 3, no empty row): one row per language present, none for an absent one; a
  language or a whole text the artifact drops loses its row(s), a removed spell its texts
  (`TestImportCatalogue_FreshDatabase`, `TestImportCatalogue_Step4_NewVersionUpserts`,
  `TestImportCatalogue_ReimportDropsAndAddsWholeTexts`). The artifact format is unchanged: its
  plain stat, school and sphere texts are the English books' (`TestLoad_Fixture`).
- **Go** (step 1): `localized.Text` (`api/internal/domain/localized`), a map from language code to
  body; `localized.Fallback` is `en` (step 3). No `*FR` field remains.
- **API** (step 6): every text field is a language map, `{}` for a text in none
  (`TestSearch_ServesTextFieldsAsLanguageMaps`, `TestNames_ServesIDsWithNameLanguageMaps`,
  `TestSpells_TextsAreLanguageMaps`, `TestSpellNames_AreIDAndLanguageMap`).
- **Search** (step 7 and the cost note): the name tiers run over every language's name
  (`TestSearch_RankingTiers`); the text lookup is the `TextsByID` query, through the
  primary key, never a scan (checked by running the app, [ADR 0025](0025-postgresql-storage.md)
  step 7), and only name, school and sphere are read for the candidates
  (`TestSearchAndDetails_MapFields`); a page's full texts are one batched read (`TestSearch_ReadsThePageTextsInOneBatch`).
- **Client**: `LocalizedText` and `localized(lang, text)` (per-field English fallback;
  `localized.test.ts`, `spellName.test.ts`, `statText.test.ts`; the French and English lookup e2e).

## Items and weapons (steps 9 to 12, migration 033)

Added 2026-10-16 (critical-a1wm). User, in triage: French item and weapon names live in the item and
weapon seeds; first need, the Power Gem shows as "Gemme de pouvoir" in French. Unlike the spells,
items and weapons are also edited at the table, so steps 10 to 12 say whose language a name is in.
Tests live under `api/internal/` unless their path starts with `web/`.

9. **Item and weapon names are localized texts** (step 5). `items.name` and `weapons.name` hold a
   key into `localized_texts`; every key has at least one row. Migration 033 moves each existing
   name as the `en` row of a new key (after the highest stored, in list order; a blank name takes
   the row's id), keeping every id, position, count and other field. The API serves `name` as a
   language map on items, weapons, the attack form's ammo choices and the items a spell can spend;
   the client picks with `localized`. A key's rows are written with the row (a new key on insert,
   replaced under the same key on update) and deleted with it.
   Tests: SQLite migration 033 moved the existing names (a past migration, the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3),
   `TestSeed_Step10_FreshDatabaseGetsEveryLanguage` (storage/postgres), `TestName_Step9_WrittenAsLocalizedTextRows`
   (itemdal), `TestItems_Step9_NameIsALanguageMap`, `TestWeapons_Step9_NameIsALanguageMap` (rest/router);
   the live-database check of the time was retired, [ADR 0025](0025-postgresql-storage.md) step 6.
10. **The seeds hold every language; a restart adds what a name lacks.** In `seeds/items.json` and
    `seeds/weapons.json`, `name` is a language map (`en` and `fr`). An empty table is filled from the
    seed. Once filled, the seed only reconciles names, and gives each of its languages to a name
    once: while every language an item's or weapon's name has still reads as the seed's, the name
    gains the seed languages it lacks that were never given, and the languages it has or gains are
    recorded as given by id and language (`seeded_name_languages`). A name the player changed in
    any language is kept as is and records nothing (it is offered again once it reads as the
    seed's, e.g. after an undo). A language the player removes after it was given (a blank body,
    step 11) is not put back on the next start, even once the row was removed and put back under a
    new key; a language added to the seed later is given once too. An id the table
    no longer has is not added back (the player removed it); no count, position or other field
    moves; a second start writes nothing. A seed name without a language is refused.
    Tests: `TestSeed_Step10_FreshDatabaseGetsEveryLanguage`,
    `TestSeed_Step10_ReconcileAddsLanguagesKeepingPlayerNamesAndRows`,
    `TestSeed_Step10_ALanguageThePlayerRemovedStaysRemoved`,
    `TestSeed_Step10_RejectsANameWithoutLanguage` (storage/postgres).
11. **The editors write the player's language.** The item and weapon bodies take `name` as a language
    map. The web form shows the name in the UI language (with the client fallback) and sends that one
    language (`editLang`: the UI language without its region), only when the player changed what it
    showed, so an untouched form never copies a fallback name into another language. A create stores
    the languages sent; an edit replaces the bodies of the languages sent and keeps the others; a
    blank body removes its language, for good (step 10 does not offer it again); a name left with no
    language, or a language code that is not a UI language (`localized.UILanguages`, the
    `web/src/i18n/locales` folders), is refused (`name` field error).
    The `item_edit` / `weapon_edit` changes list one `name` change per language whose body differs.
    A pop restores the snapshot's whole name; a snapshot written before 033 holds a plain string,
    which restores the English body only and keeps the other languages the row has now.
    Tests: `TestUpdate_Step11_NameEditKeepsTheOtherLanguages`, `TestCreate_Step11_NameInThePlayersLanguage`,
    `TestCreate_Step11_RefusesAnUnknownLanguage`, `TestUpdate_Step11_RefusesAnUnknownLanguage`,
    `TestRestorers_Step11_PlainNameSnapshotKeepsTheOtherLanguages` (itemsvc, weaponsvc),
    `TestWith_EditsOnlyTheLanguagesSent`, `TestSnap_ReadsAMapOrAPlainString`,
    `TestUILanguages_AreTheWebLocales` (domain/localized),
    `web/src/lib/itemForm.test.ts`,
    `web/src/lib/weaponForm.test.ts`.
12. **Recorded names, named again by id.** Results and log payloads (use, restock, attack, its ammo,
    a cast's spent item, change records, a breakdown's detail) keep one recorded string, as the
    Consequences allow: the English body, else the first language in code order (`Text.Any`). The
    ammunition's modifier carries its item id. The client names an item or a weapon by id from the
    current lists in the UI language while they hold it, else shows the recorded string
    (`inventoryNamer`, `itemNamer`; an item power's contribution and timeline row as in
    [ADR 0016](0016-inventory.md) step 14). A text with neither the UI language nor English shows
    its first language (a name typed in French only).
    Tests: `TestAny_FallbackElseFirstLanguage` (domain/localized), `TestResolveAttack_WeaponBuffPlusMagicAmmo`
    (combatsvc); `web/src/i18n/inventoryName.test.ts`, `web/src/i18n/localized.test.ts`,
    `web/src/lib/itemPowers.test.ts`, `web/src/lib/logSummary.test.ts`.

Known limits: an item's `type` label and `notes` stay plain text (labels and notes the player types);
a name typed before 033 in another language than English (the live "Diamant") is stored as `en` and
shows as typed in every language.
