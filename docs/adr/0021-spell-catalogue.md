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
   - `source` is the provenance of the English text: a book code (`PHB`, `TOM`, …), or `WEB` for a
     spell whose text comes from the original web scrape and that no book has been matched to
     (the scrape holds 885 spells, the two ingested books fewer; no spell is dropped for lack
     of a book). The French text is tied to no code: `fr` present means a French source exists.
   - A description is a **small Markdown subset**: blocks separated by exactly one blank line
     (`\n\n`); a block is a **paragraph** (one line, no hard wrap, not starting with `- `, `|` or
     `#`), a **bullet list** (consecutive lines starting with `- `) or a **pipe table** (consecutive
     lines starting and ending with `|`, the second one the `| --- |` delimiter row). No headings,
     no HTML, no emphasis markers (the builder checks both: the schema cannot), no leading or trailing whitespace. Tables stay tables (Prismatic
     Spray's colour table is not flattened into prose). The schema checks the block structure with an
     RE2-safe pattern (Go's default regex engine); the builder additionally checks that each table
     has a delimiter row and a constant column count. The UI renders exactly this subset.
   - Adding or renaming a field is a new `format`, with an amendment of this ADR; importers refuse
     a format they do not know rather than guess.
2. **Reference data and game state are separate.** The importer owns the catalogue tables and nothing
   else; play never writes to them. Game state refers to a spell by `id` only, and keeps working
   across catalogue versions as long as step 5 holds.
3. **Ids are stable, kept in a ledger, and assigned once.** `id` starts as the key `spellsvc.Keys`
   assigns: the snake_case slug of the English name (`spelleffectsvc.Slug`), `_l<level>` for a
   higher-level spell of the same name and class, `_priest` for a Priest spell sharing a Wizard
   spell's name. But `Keys` depends on the whole set (adding a Wizard "X" would turn Priest `x` into
   `x_priest`), so the **builder never recomputes published ids**:
   - the builder keeps a committed **published-id ledger** in `critical-rulebooks`, one entry per
     spell, `(class, level, English name) -> id`, seeded from the 885 ids in use today (so every id in
     `spellbook.json`, `spell_effects.json` and saved slots stays valid);
   - a spell found in the ledger takes its id from it; a **new** spell takes the `Keys` candidate,
     which is then added to the ledger; if the candidate equals any published or other id, the
     **build fails**;
   - the build also **fails when a ledger entry matches no spell in the build**, unless that entry
     is explicitly marked `"removed": true`. This is how a rename is caught: a spell whose English
     name or level was corrected looks new, and its old entry is left unmatched. A human then either
     **re-keys** the entry (edits its `(class, level, name)` to the corrected values, keeping the
     id: a renamed spell keeps its id) or marks it `"removed": true` (the spell really left the
     catalogue; its id stays reserved and is never reused). A removed id still trips step 5.4.3 if
     game state references it;
   - an id is never changed and never reused for a different spell; ids are unique in the artifact;
   - the API **stores and serves the artifact's id** and stops calling `Keys` (the catalogue DAL's
     `catalogKeys`); `Keys` lives on only in the builder.
4. **Where it lives and how it ships.** The builder (`critical-rulebooks`, private) writes
   `dist/catalogue.json` and commits it there. The artifact is **never committed to the public repo**.
   - `just catalogue` (like `just rulebooks`, and never part of `just validate`) fetches it into
     the gitignored `.catalogue/catalogue.json`, from `CATALOGUE_REMOTE` (default: the rulebooks
     remote) at `CATALOGUE_REF` (default: the default branch), by a shallow git fetch of that one
     file, and says plainly if the repo is not accessible.
   - The API reads the file at the path in `CRITICAL_CATALOGUE` (default `.catalogue/catalogue.json`,
     relative to the repo root; a relative value is resolved against the repo root too). The seeds directory no longer holds spells.
   - The deployment fetches the same file with a deploy key (read-only, scoped to the rulebooks
     repo) at build or deploy time and sets `CRITICAL_CATALOGUE`; hosting details belong to the
     hosting item.
   - CI and contributors without access run on the small fixture `api/testdata/catalogue.json`, in
     the same format and valid against the schema, holding
     **every spell id that the `api/testdata/seeds` spellbook and spell effects reference** (so the
     e2e suite and service tests keep resolving them), some with `fr` and some without, one with a
     list and one with a table, and **invented placeholder text, never book text**. `just dev` and the e2e suite point
     `CRITICAL_CATALOGUE` at the real file when present and at the fixture otherwise. A Go test
     validates the fixture against the schema, and the builder validates its output against the same
     schema before writing it.
5. **Import rules** (run by the API at startup, before serving):
   1. No catalogue file at `CRITICAL_CATALOGUE`: refuse to start, naming the path and `just catalogue`.
   2. Parse and validate against the format-1 rules above (unknown `format`, schema violation,
      duplicate `id`): refuse to start, listing the first offences.
   3. If `version` equals the version recorded in the database, do nothing.
   4. Otherwise, in **one transaction**:
      1. **First import on an existing database.** The `spells` table already has an integer primary
         key `id`, so the artifact id goes in a new nullable, unique column **`key`**. Rows with a
         NULL `key` are backfilled before anything else: `Keys` is run over the existing rows,
         which reproduces the ids in use today, and each row is matched to its key by
         `(name, class, level)`. A database can hold the same `(name, class, level)` twice (an older,
         uncleaned catalogue): only the **oldest row** (lowest `id`) takes the key; the others keep a
         NULL `key`, are never updated, never served and never referenced (state refers to keys),
         and are left in place. This is the last use of `Keys` in the API and is removed with the
         old seed loader. Without it the first import would insert all spells a second time.
         Before the upsert, **each backfilled key is verified**: the row's `(name, class, level)`
         must equal that of the artifact spell with the same `key` (or of its ledger entry, when the
         artifact corrected the name or level). A live database may hold rows outside the set the
         ledger was seeded from (`seedSpells` never deleted), which shifts `Keys` suffixes; a
         mismatch would let the upsert silently overwrite a different spell. On any mismatch
         the importer refuses to start and lists the keys, with both triples.
      2. Upsert every artifact spell by `key` (insert if new, overwrite every catalogue field if
         present).
      3. Delete the spells whose `key` is no longer in the artifact, **only if nothing references
         them**; the importer scans all of: the spellbook and spell-effect seeds, the
         `spell_slots` table, the `before_state` and `after_state` JSON of `action_snapshots` (they
         hold `spell_slots`, which undo and redo restore) and every spell id in `action_log` payloads (`spellId` of cast and
         exposure entries, `spells[].id` of spell_prep entries). If any removed spell is referenced, roll back and refuse to start, listing
         every such `key` and where it is used. No spell is silently lost.
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
- The importer needs a migration (French name and description columns, the `key` column of step 5.4.1,
  a catalogue metadata row); that is that item's work, not this ADR's.
- French names come from the catalogue's `name.fr`, the single source of truth. The web i18n `spells`
  namespace (French names, #138) and the spellbook seed's `sheet` names are retired when the UI item
  (y9e0.7) lands.
- The description subset is part of the format: the UI renders paragraphs, bullet lists and pipe
  tables; anything richer is a new `format`.
- Other reference tables (weapons, items, rule tables, crit table, skills) keep loading from
  `seeds/` until they move to the catalogue in a later format or artifact; this ADR covers spells.
- Only English and French exist. A third language is a new `format`.
- Corrections to book text are a new artifact version; nothing in the app edits catalogue rows.

## Implementation notes (the importer, 2026-10-15)

Where the importer had to settle a detail the decision above leaves open, or reads differently:

- **Step 4, `just catalogue`**: it reuses the rulebooks clone (`just rulebooks`, remote `RULEBOOKS_REMOTE`,
  directory `RULEBOOKS_DIR`) and takes `dist/catalogue.json` at `CATALOGUE_REF` (default: the checked-out
  branch) with `git show`, instead of a separate shallow fetch from a `CATALOGUE_REMOTE`. The deployment
  fetch stays the hosting item's.
- **Step 4, which file runs where**: Go tests and the e2e suite **always** use the fixture (its counts and
  search results are what the specs assert; CI has no access anyway). `just dev` uses the real file when
  `.catalogue/catalogue.json` exists, and otherwise the fixture **together with** `api/testdata/seeds`
  (the campaign spellbook and spell effects refer to spells only the real artifact holds), on a database
  of their own, `.dev/fixture.db`: the dev database `.dev/critical.db` is only ever opened with the real
  file, so a missing `.catalogue` can never replace its spells with the fixture's. The tests of
  the real seeds against the real artifact (`seedcheck`) skip when it is absent. The repo root of a
  relative `CRITICAL_CATALOGUE` is the nearest directory above the working directory with a `.git`
  entry; outside a checkout the path stays relative to the working directory.
- **Step 5.4.1, verification**: the artifact does not carry the builder's ledger, so a backfilled key is
  verified against the artifact spell with that key only. A first import onto a database whose spell was
  renamed in the same artifact is therefore refused; no artifact renames anything relative to the 885 ids
  it was seeded from (checked on the real artifact, which adds one new spell), so this cannot happen
  today, and later renames happen on databases that are already keyed.
- **Step 5.4.3, scanned references**: also the payloads of the corrections (`action_corrections`) of
  cast, exposure and spell_prep entries, since an amend replaces the entry's payload.
  Its refusal also names the catalogue the database was built from (the version in `catalogue_meta`)
  and `just catalogue`, for a database started on the wrong catalogue.
- **Spell effects seed**: every `spell_effects.json` id is checked against the catalogue at startup, as
  the spellbook is (an id the catalogue does not hold would never apply); unknown ids refuse the start,
  all listed (`TestStartup_RefusesSpellEffectsTheCatalogueDoesNotHold`,
  `TestRealCatalogue_AnUnknownSpellEffectIDRefusesTheStartup`).
- **Storage**: `spells.key` (unique, NULL allowed), `name_fr`, `description_fr`, `source` (migration 027)
  and the one-row `catalogue_meta (format, version)`. The fixture's version is `1970-01-01`, so switching
  between the fixture and a real artifact always imports
  (`TestRealCatalogue_SwitchingADatabaseBetweenTheFixtureAndTheRealCatalogue`).
- **REST**: spell payloads keep `name` and `description` (English) and add `name_fr` and
  `description_fr`, omitted when the catalogue has no French text, and `source`. The client picks the
  language (fallback per field); no `lang` query parameter.
- **Tests per step**: 5.1 `TestLoad_Step1_MissingFileNamesThePathAndTheRecipe`; 5.2
  `TestParse_Step2_Refusals`; 5.3 `TestImportCatalogue_Step3_SameVersionWritesNothing`; 5.4.1
  `TestImportCatalogue_Step4_1_BackfillsKeysOfAnExistingDatabase` and
  `TestImportCatalogue_Step4_1_RefusesKeysThatDoNotMatchTheArtifact`; 5.4.2
  `TestImportCatalogue_Step4_NewVersionUpserts`; 5.4.3
  `TestImportCatalogue_Step4_3_DeletesAnUnreferencedRemovedSpell`,
  `TestImportCatalogue_Step4_3_RefusesRemovingReferencedSpells` and `TestStateSpellRefs_ScansEveryPlace`;
  5.5 the two refusal tests (nothing changes, the old version stays recorded); 5.6
  `TestImportCatalogue_Step6_TypedStatsParsedAtImport`; the upgrade of a database seeded before the
  catalogue `TestImportCatalogue_UpgradesADatabaseSeededBeforeTheCatalogue`, and of a live one
  `TestRealCatalogue_UpgradesACopyOfALiveDatabase`.

## Implementation notes (the UI, 2026-10-15)

- **French names have one source.** The web i18n `spells` namespace, `web/src/i18n/phbSpellIds.json`
  (and its Go check) and the spellbook seed's `sheet` names are gone. The client shows `name_fr` /
  `description_fr` in the French UI, English per missing field.
- **Spells known only by id** (spellbook, slots, journal payloads, which record the English name at
  write time) are named from `GET /api/spells/names` (`[{id, name, name_fr?}]`, every catalogue
  spell), loaded once by the client; payloads are not rewritten.
- **Search** matches `q` against both names on the server (case, accents and ligatures ignored,
  best tier of the two), in either UI language; the client-side French match and the `match_exact`
  / `match_prefix` / `match_contains` parameters are retired (`TestSearch_RankingTiers`).
- **Descriptions** are rendered by a small parser of exactly the step 1 subset
  (`web/src/lib/spellMarkdown.ts`) into React text, never HTML; a block that does not fit the subset
  is shown as text rather than dropped.
