# ADR 0021: Reference data: the spell catalogue artifact, storage and deployment

**Status**: accepted, 2026-10-15 (critical-y9e0.1; the contract between the catalogue builder in the
private `critical-rulebooks` repo and the API importer)

## Context

User, 2026-10-15: _"at some point we'll have to stop with the seeds. we need a more thoughtful way of
storing and deploying this data"_, _"the sibling repo is for offline ingestion. all data must then be in
some form or another available in the app wherever it's deployed (it won't run locally forever)"_,
_"don't lose any spell. just make them better in two languages"_ and _"it is my plan in the future to
have this app host an actual game with the dm and several players"_.

Today every reference table is a JSON file in `seeds/`, loaded at startup (spells are reconciled by
id). `seeds/spells.json` (885 spells) carries the scraped English book text in the public repo, which
the project rule forbids for book text. French names live in the web i18n `spells` namespace. Nothing
says where reference data comes from or how it ships.

Two kinds of data exist and are kept apart: **reference data** (read-only at runtime, rebuilt offline,
versioned: the spell catalogue) and **game state** (transactional, [ADR 0018](0018-transactions-and-undo.md):
characters, HP ledger, spellbook, slots). The first is replaced wholesale by a new version; the second
is only ever appended to by play.

## Decision

1. **The catalogue is one JSON artifact, format 1.** Shape, enforced by the committed JSON Schema
   [`schemas/catalogue.schema.json`](../../schemas/catalogue.schema.json) (shape only, no book text):

   ```json
   { "format": 1, "version": "2026-10-15", "spells": [ { "...": "spell" } ] }
   ```

   - `format` (required, integer): `1`. An importer accepts only the formats it knows and refuses
     any other with an error that names the format it found.
   - `version` (required, string): the UTC build date `YYYY-MM-DD`, with `.N` appended when several
     builds happen the same day (`2026-10-15.2`). It is compared for equality only, never ordered:
     "differs from the imported one" is the whole rule (a rollback to an older artifact is an
     import like any other). A date was picked over semver because content, not API, changes.
   - `spells` (required, at least one). A spell is an object with **no other properties** than:

     | Field | Req. | Notes |
     |---|---|---|
     | `id` | yes | snake_case key, step 3 |
     | `class` | yes | `"Wizard"` or `"Priest"` |
     | `level` | yes | integer 1 to 9 |
     | `school` | yes | raw book text, may hold several (`"Alteration, Evocation"`) |
     | `sphere` | no | Priest spells only, raw text; omitted for Wizard spells |
     | `casting_time`, `range`, `duration`, `area_of_effect`, `components`, `saving_throw` | yes | the six raw stat texts, exactly the fields `spellstats.Raw` / the current `seeds/spells.json` loader read; non-empty (use the book's own `"None"`/`"Special"`); the typed form is parsed by the API at import, never shipped |
     | `name` | yes | `{ "en": "…", "fr": "…" }`, `en` required, `fr` optional |
     | `description` | yes | `{ "en": "…", "fr": "…" }`, `en` required, `fr` optional |
     | `source` | yes | book of the English text, 2 to 8 upper-case characters (`PHB`, `TOM`, …) |

   - A language that is missing is **omitted**, never an empty string. A reader shows `fr` when
     present and falls back to `en`, per field, so a spell with no French source is complete.
   - A description is paragraphs separated by exactly one blank line (`\n\n`), no hard line wraps
     inside a paragraph, no leading or trailing whitespace. The schema checks this.
   - Adding or renaming a field is a new `format`, with an amendment of this ADR; importers refuse
     a format they do not know rather than guess.
2. **Reference data and game state are separate.** The importer owns the catalogue tables and nothing
   else; play never writes to them. Game state refers to a spell by `id` only, and keeps working
   across catalogue versions as long as step 5 holds.
3. **Ids are stable and computed once.** `id` is the key `spellsvc.Keys` assigns today: the snake_case
   slug of the English name (`spelleffectsvc.Slug`), `_l<level>` for a higher-level spell of the same
   name and class, `_priest` for a Priest spell sharing a Wizard spell's name. The builder reproduces
   that rule so that every id in `spellbook.json`, `spell_effects.json` and saved slots stays valid.
   Once an id has been published it never changes, even if the English name is later corrected, and
   it is never reused for a different spell. Ids are unique in the artifact.
4. **Where it lives and how it ships.** The builder (`critical-rulebooks`, private) writes
   `dist/catalogue.json` and commits it there. The artifact is **never committed to the public repo**.
   - `just catalogue` (like `just rulebooks`, and never part of `just validate`) fetches it into
     the gitignored `.catalogue/catalogue.json`, from `CATALOGUE_REMOTE` (default: the rulebooks
     remote) at `CATALOGUE_REF` (default: the default branch), by a shallow git fetch of that one
     file, and says plainly if the repo is not accessible.
   - The API reads the file at the path in `CRITICAL_CATALOGUE` (default `.catalogue/catalogue.json`
     next to the working directory). The seeds directory no longer holds spells.
   - The deployment fetches the same file with a deploy key (read-only, scoped to the rulebooks
     repo) at build or deploy time and sets `CRITICAL_CATALOGUE`; hosting details belong to the
     hosting item.
   - CI and contributors without access run on the small fixture `api/testdata/catalogue.json`, in
     the same format and valid against the schema, with a few spells (some with `fr`, some without)
     and **invented placeholder text, never book text**. `just dev` and the e2e suite point
     `CRITICAL_CATALOGUE` at the real file when present and at the fixture otherwise. A Go test
     validates the fixture against the schema, and the builder validates its output against the same
     schema before writing it.
5. **Import rules** (run by the API at startup, before serving):
   1. No catalogue file at `CRITICAL_CATALOGUE`: refuse to start, naming the path and `just catalogue`.
   2. Parse and validate against the format-1 rules above (unknown `format`, schema violation,
      duplicate `id`): refuse to start, listing the first offences.
   3. If `version` equals the version recorded in the database, do nothing.
   4. Otherwise, in **one transaction**: upsert every spell by `id` (insert if new, overwrite every
      catalogue field if present), then delete the catalogue spells whose `id` is no longer in the
      artifact **only if no game state references them**. References are the spellbook, spell
      effects and prepared-spell slots (in the database and in the seeds). If any removed spell
      is referenced, roll back and refuse to start, listing every such `id` and where it is used.
      No spell is silently lost.
   5. Record the imported `format` and `version` in the same transaction; a failed import changes
      nothing, and a second run of the same artifact changes nothing.
   6. The typed stats are parsed from the six raw texts at import (as `seedSpells` does today).
6. **Storage engine.** SQLite now, as an ordinary part of the app database: reference data is
   relational (spells are joined to the spellbook, effects and slots by id) and has one writer
   today. When the app hosts a game with a DM and several players, the engine becomes Postgres (a
   separate epic); bob supports both dialects (`bobgen-psql`), and the artifact is engine-neutral
   JSON, so only the importer's SQL changes. No document store (Mongo): there is no data that
   wants one.
7. **No book text in the public repo.** The schema, this ADR, the fixture and the code carry shape and
   invented text only. `seeds/spells.json` is deleted when the importer lands; the rule of
   `CLAUDE.md` is unchanged, this step is the mechanism that makes it hold. French and English
   text, OCR and the scripts stay in `critical-rulebooks`.

## Consequences

- The builder item and the importer item can proceed in parallel against this ADR and the schema:
  the builder's output must validate; the importer's tests run on the fixture and cover each of
  step 5's rules (version unchanged, new version upserts, removal of an unreferenced spell,
  refusal for a referenced one with the listing, unknown format, missing file, duplicate id).
- A deployment without the artifact cannot start. That is deliberate: a silently empty spell list
  is worse than a clear error.
- The importer needs a migration (French name and description columns, a catalogue metadata
  row, and an `id` column on `spells`); that is that item's work, not this ADR's.
- Other reference tables (weapons, items, rule tables, crit table, skills) keep loading from
  `seeds/` until they move to the catalogue in a later format or artifact; this ADR covers spells.
- Only English and French exist. A third language is a new `format`.
- Corrections to book text are a new artifact version; nothing in the app edits catalogue rows.
