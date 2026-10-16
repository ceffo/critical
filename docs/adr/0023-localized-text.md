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
- Follow-ups, not part of this ADR: the schema, API and client change (critical-v3r5) and the check of
  step 8 (critical-s6z0). Until they land, the code and `docs/backend.md` / `docs/frontend.md` still
  describe `name_fr` / `description_fr`.
