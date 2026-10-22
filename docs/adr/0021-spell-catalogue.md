# ADR 0021: Reference data: the spell catalogue artifact, storage and deployment

**Status**: accepted, 2026-10-15 (critical-y9e0.1; the contract between the catalogue builder in the
private `critical-rulebooks` repo and the API importer). Amended by [ADR 0023](0023-localized-text.md):
no per-language column or field; the `name_fr` / `description_fr` wording below is superseded.
Step 10 (the class and race tables, format 4) is _proposed_ (critical-reul.9).
Step 11 (the racial level limits of DMG Table 7, format 5) is _proposed_ (critical-reul.20).
Step 12 (the spheres, format 6) is **decided as built**, 2026-10-18 (critical-reul.15, accepted by the
user; the API reads formats 5 and 6, the builder publishing format 6 is a separate item).
Step 13 (the equipment, format 7) is **decided as built**, 2026-10-18 (CRI-33; the API reads formats 6 and 7).
Step 14 (the languages, format 8) is **decided as built**, 2026-10-18 (the API reads formats 7 and 8).
Step 15 (the gameplay values of a spell text, marked by the API, no format change) is **decided as built**, 2026-10-19.
Step 16 (the proficiency descriptions, format 9; the API reads format 9 only) is **decided as built**, 2026-10-20 (user triage of 2026-10-20).
Step 17 (Table 5's magical defence adjustment on the wisdom rows, format 10; the API reads format 10 only) is **decided as built**, 2026-10-21:
the column is decided by [ADR 0006](0006-saving-throws.md) step 9.1 (user triage of 2026-10-21) and step 1's rule makes it a format.
Step 9 (the PHB rule tables in the artifact, format 3) is **accepted**, 2026-10-16 (proposed in
critical-y9e0.9.1, accepted by the user; implemented by critical-y9e0.9.2, .9.3 and .9.6, which retired
format 1). Step 6 (the storage engine) amended 2026-10-16 by [ADR 0025](0025-postgresql-storage.md):
PostgreSQL now.

_Amended by [ADR 0033](0033-equipment.md) (equipment, accepted 2026-10-18), on implementation: a step 13 (format 7: equipment reference data, `equipment` and two rule tables, ADR 0033 steps 21 to 23)._

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

   - _Amended by [ADR 0023](0023-localized-text.md): in the database and the API these language maps
     are `localized_texts` rows and language maps, never per-language fields._
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
   - _Amended by step 9: format 3 is format 1 plus the PHB rule tables; format 2 is never reused._
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
      _Amended by step 9: against the rules of the artifact's format (1 or 3), including step 9.3's
      table rules for format 3._
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
   *Amended 2026-10-16 by [ADR 0025](0025-postgresql-storage.md):* the engine is PostgreSQL now,
   before the first deployment, because free hosting on fly allows no volume for a SQLite file
   ([ADR 0024](0024-deployment.md) step 5); it is not tied to the multi-user game, which stays a
   later design. The DAL is generated by sqlc, not bob. Reference data stays relational, in the app
   database, and the artifact stays engine-neutral JSON.
7. **No book text in the public repo.** The schema, this ADR, the fixture and the code carry shape and
   invented text only. `seeds/spells.json` is deleted when the importer lands; the rule of
   `CLAUDE.md` is unchanged, this step is the mechanism that makes it hold. French and English
   text, OCR and the scripts stay in `critical-rulebooks`.

8. **Stats are typed values; wording, units and language are the UI's.** The catalogue, the database
   and the API carry the six stats as the book's English text plus their typed form (step 5.6), and
   never a translated or reworded stat text (no `*_fr` stat field, column or format). The client
   words the typed values in the UI language:
   - **In the books' language (English)** the stat line is the book's text, as before (its common
     values labelled), and the value at the caster level keeps the book's units.
   - **In another language (French)** every stat is worded from its typed value through i18n keys
     (`lookup.unitWords`, `nouns`, `shapes`, `conditions`, `stat.*`): units and nouns agree in
     number by the language's plural rules, per-level terms read "/niveau", "/2 niveaux", shapes
     and the components and save kinds are translated, and so is the value at the caster level.
   - **Lengths are converted where the language uses metres** (`lookup.lengths`, French: metric),
     by the French books' convention, not the exact factors: **1 yd = 1 m**, **1 ft = 0.3 m**,
     1 mi = 1.5 km, 1 in = 2.5 cm. The French Tome of Magic's stat lines turn yards into metres one
     for one and feet into 0.3 m in every range and area, a square yard into a square metre, a mile
     into 1.5 km (all three of its miles; the French PHB keeps paces and feet, and gives one mile as
     1.5 km once and as a fraction of a league once), and a square mile into 2.25 km². Areas
     are the squares of the lengths; a **cubic foot is 27 litres** (the books' own figure) and a
     cubic yard 1 m³, a cubic inch 15.625 cm³. Numbers use the language's decimal mark ("4,5");
     lengths and areas keep at most two decimals, volumes three significant digits and every
     integer digit ("156 cm³", "540 litres"), so a small volume is not rounded away. A word agrees
     with the converted number ("27 litres"). Dice are never converted (a length with dice keeps its
     unit); times are never converted.
   - **No rounding beyond the convention; under a metre in centimetres.** The converted number is
     the convention's product, never rounded further: 930 ft is **279 m**, not 280 m. The French
     books do the same, level-scaled values included ("75 feet" at level 15 is "22,5 mètres", 13 ×
     20 feet "78 mètres", 13 × 2 feet "7,8 m"), and no French book prints a figure rounded off a
     converted one. A length shown in metres that is **under 1 m is written in centimetres**
     (1 ft is "30 cm", 3 ft "90 cm", "1 ft/niveau" is "30 cm/niveau"): the French Tome of Magic
     writes 30, 60 and 90 cm over 30 times and never "0,x m", while 5 ft stays "1,5 m". A range
     ("1-5 ft") takes the unit of its larger number; areas and volumes keep their units.
     Evidence (2026-10-15), checked by `statTextBook.test.ts` when the rulebooks clone is present:
     of the French stat lines (range, area) that give metric lengths, **159** are the convention's
     numbers exactly, 4 cannot be read number for number (a number in words, a side given once,
     dice), and 7 differ, all slips of the book: two yards read as feet (3 m for 10 yd, 9 m for 30
     yd), one wrong number (18 m for a 30 ft radius), the two square-foot lines that disagree with
     each other (10 m² for 100 sq ft, 28 m² for 1000 sq ft; the convention keeps 0.09), and a cube
     of feet and a cubic foot read as the other (27 litres, a 30 cm side). The French PHB keeps feet
     and paces; its one mile is 1,5 km. In the descriptions a minority of translators use 1 ft ≈ ⅓ m
     (25 ft as 8 m, 12 ft as 4 m) or 1 mi ≈ 1.6 km (5 mi as 8 km); the stat lines never do, and they
     are what the conversion follows.
   - **The book's text is the fallback, only for what is not typed**: a field the parser left
     special (other than the book's own "Special", which is a label), a word the language has no
     key for (an unlisted noun, unit or condition, a non-level divisor such as "/recipient"), and
     an area reading the parser marks **partial** (`partial`: the text has an alternative, a
     number, die or word its typed form does not hold, such as "1 creature or item" or "One
     creature/two levels"). Its common values are labelled as before. A partial area has no value
     at the caster level either: the server computes and serves none, so nothing from the lossy
     reading is shown or rolled. Text is wrapped, never cut.
   - **Area wording (2026-10-16)**, generic rules, never per spell; the French term of an unlisted
     noun is the one the French books print for it (`lookup.nouns`):
     1. An extent in dice ("4d4 sq. mi.") and the cap of an extent ("5 yds./level (90 yds.
        maximum)", "30 ft. radius, max. 60 ft.") are read whole: the dice and the `max` stay typed
        (`TestAreaOfEffect_DiceAndCapOfAnExtent`; French `words dice and a cap in an extent`).
     2. "1 or more creatures" is a count that is a floor (`or_more`), worded "1 créature ou plus"
        and with no count at a caster level (`TestAreaOfEffect_OrMoreIsAFloor`; French `words "1 or
        more" after the noun`).
     3. A non-level divisor of an extent or a count ("square/priest", "creature/rd.") is kept as
        the term's `per`; a language that lists it (`lookup.perWords`) words it "/prêtre", any other
        stays the book's text (`TestAreaOfEffect_KeepsANonLevelDivisor`; French `words a
        non-level divisor the language lists, and no other`).
     4. A path and a line are shapes (`TestAreaOfEffect_PathAndLineAreShapes`; the shapes test
        of `areaText`).
     5. "Up to" on an extent bounds the figure, not the length inside it: "cube de 6 m maximum",
        not "cube de jusqu'à 6 m"; on a count it stays "jusqu'à 6 créatures" (French `words "up to"
        on the figure, not on the length inside it`).
     6. "Per level" spelled out ("1 creature per level", "per 2 levels") is the per-level term
        (`TestAreaOfEffect_PerLevelSpelledOut`).
     7. "Sphere with diameter of 1 ft./level" is a diameter of a sphere
        (`TestAreaOfEffect_DiameterOfAFigure`; French sphere "de 30 cm/niveau de diamètre", the
        shapes test of `areaText`).
     8. A group's size is typed beside its noun: "One unit of up to 200 individuals" is one unit
        (`members`, up to 200, `members_noun`), worded "1 unité de 200 individus maximum"
        (`TestAreaOfEffect_GroupSize`; French `words the size of a group after its noun`).
     9. An extent after "within" bounds the targets ("1 animal within 30 ft.", `within`): worded
        "1 animal dans un rayon de 9 m"; within a shape the book's text stays
        (`TestAreaOfEffect_WithinBoundsTargets`; French `words targets within a distance, and an
        area around what is touched`).
     10. An area "around creature touched" keeps its extent and says what it is centred on
         (`around_touched`; `TestAreaOfEffect_AroundWhatIsTouched`, same French test).
     11. Wide, long, high (tall), thick and deep name the axis of an extent (`axis`), worded
         "de large", "de long", "de haut", "d'épaisseur", "de profondeur" after the figure, which
         is gender-free: "chemin de 3 m de large et de 18 m de long". An axis word with no extent
         for it (a cone "wide at base") keeps the book's text
         (`TestAreaOfEffect_AxisOfAnExtent`; French `words the axis of each extent`).
     12. The unlisted nouns pebble ("galet", PHB *Pierre magique*) and eye ("œil", no French book
         entry for the spell) are worded (French `has a French noun for pebbles and eyes`), and the
         area texts that name no extent (hearing range, line of sight, the caster's hand, staff,
         shadow or weapon, "Variable") are common values labelled in `lookup.values.area`.
     13. A note the book adds to an area is kept beside it: "+ special", "or special" and
         "(special)" are `special` (plus, or, note), "(usually caster)" is `usually`; worded
         "rayon de 1,5 m + spéciale", "… ou spéciale", "… (spéciale)", "1 individu (généralement le
         lanceur)". Only after a whole area or targets reading
         (`TestAreaOfEffect_Annotated`; French `words several areas, a sum of readings and the
         notes the book adds`).
     14. A number word before an extent and a plural figure counts the figures ("Two 10-ft.
         cubes/level": `count` 2, one 10-ft./level cube), worded "2 cubes de 3 m/niveau"
         (`TestAreaOfEffect_SeveralAreas`, same French test).
     15. "A + B" is a sum of whole readings (kind `together`, `parts`), worded part by part
         ("cube de 12 m + cube de 3 m/niveau", "Le lanceur + 1 personne/8 niveaux"); it has no
         value at a caster level. A part that holds an extent and a count stays the book's text,
         the extent may bound the other parts too (`TestAreaOfEffect_Sum`, same French test).
     16. "Wide at base" / "at the end" says where an extent is measured (`site`, base or end), and
         "and" separates the extents of one text: "cône de 60 m de long et de 30 m de large à la
         base" (`TestAreaOfEffect_WhereOnTheFigure`; French `words where on the figure an extent
         is measured, and a bounded line of sight`). This replaces the book's text kept by item 11
         for a cone "wide at base".
     17. "Line of sight" is a shape and a leading "max." bounds the extent after it: "ligne de vue
         de 18 m maximum" (`TestAreaOfEffect_BoundedLineOfSight`, same French test).
     18. A count of something and its size ("1 object up to 10 ft. long", "1 page, up to 2 ft.
         square") is targets with `size`: the extents are each target's, not an area of their own
         ("1 objet, 3 m de long maximum"). Without `size` they would read as an area beside the
         count (`TestAreaOfEffect_SizeOfATarget`; French `words the size of a target, a level times
         an area, a wave and a radius with a column`).
     19. "Caster's level x 100 sq. ft." is 100 square feet per level (the self reading of "caster"
         no longer takes it), worded "9 m²/niveau"; "wave" is a shape ("vague de 60 cm de haut et
         de 3 m/niveau de long"); a radius with a second extent and a figure is that figure's other
         extent ("colonne de 1,5 m de rayon sur 9 m"; with more extents the book's text stays)
         (`TestAreaOfEffect_LevelTimesAndWave`, same French test).
     20. In a choice, a bare alternative of several words ("1 speaking creature or written text")
         has a qualifier of its own and takes the count before it; one word may share the
         qualifier ("1 dead human or demihuman") and is not read
         (`TestAreaOfEffect_QualifiedAlternatives`, same French test).
     21. Lines left in the book's English text (no generic rule; 18 of 886 spells by
         `just stat-coverage`, 2026-10-16 catalogue): a cap or divisor on a whole choice or list
         ("1 creature or object, max. 1 cu. ft./level", "1 creature or 1 cu. ft. of
         substance/2 levels", "One plant, seed, or tree/level", "1 small creature or 1 cubic foot
         per level (up to 10 …)"), a qualifier that may be shared ("1 dead human or demihuman"),
         an area within an area ("10 sq. ft/level within a 40-ft. radius", "One creature/5 levels
         of the caster within a 20-foot cube"), a figure with several measures ("60-ft. cone,
         30-ft. diameter at end, 5-ft. at base", "10 ft. sq./level, 1 spike/sq. ft.", "1d4 sticks +
         1 stick/level in a 10-ft. cube", "10-foot square/level;96-inch thickness/level"), a size
         before the noun or an approximate one ("3-ft. long blade", "One chest, about 2 x 2 x 3
         ft."), what is created unnamed ("Creates 1d4+1", "Creates a sphere"), "10 or 100 (TS)
         fire", and "5 x 20 x" (truncated in the book text: fixed in the rulebooks repo).

9. **The PHB rule tables are reference data, carried by the artifact from format 3** (_accepted,
   2026-10-16_). User, 2026-10-15, on moving the other `seeds/` tables: _"ADR 0021 covers all
   reference data; the epic migrates spells only."_ Of the families still in `seeds/`, only the rule
   tables (`seeds/rule_tables.json`) are reference data as this ADR defines it, read-only at runtime
   and rebuilt offline (scoper, 2026-10-16, from the code): the crit table is a house table edited
   in the app ([ADR 0001](0001-critical-hits.md) step 4) and its seed is an unread `[]`; items,
   weapons and skills are the campaign character's possessions and scores, edited at the table
   ([ADR 0016](0016-inventory.md), [ADR 0023](0023-localized-text.md) steps 10 and 11), which step 2
   forbids for catalogue data. This step does not move them; whether they get a reference catalogue
   is a separate question.
   1. **Format 3 is format 1 plus `rule_tables`.** Every rule of steps 1 to 8 on `format`, `version`
      and `spells` is unchanged; the artifact gains one required top-level object:

      ```json
      { "format": 3, "version": "2026-10-20", "spells": [ "…" ],
        "rule_tables": { "strength": [ "…" ], "dexterity": [ "…" ], "bard_spells": [ "…" ], "bard_experience": [ "…" ] } }
      ```

      Format 2 is **not reused**: it named the retracted `{en, fr}` stat shape (see "Implementation
      notes (typed stats in the UI language)", History). A format number names one shape forever, so
      a stray format-2 file is refused (step 5.2), never misread. A format-1 artifact carries no
      `rule_tables`. _Amended 2026-10-16 (step 9.1 amendment below): the schema describes format 3
      only, `format` is the constant 3 and `rule_tables` is required
      (`Schema_Step9_Format3RuleTables (retired by step 16)`); it described both formats until format 1 was retired._
   2. **Four tables, all required, numbers only.** The keys and rows are exactly those of
      `seeds/rule_tables.json` today, so the seed adapter's parser reads them unchanged:

      | Key | PHB table | Row fields (every one required, integers) |
      |---|---|---|
      | `strength` | Table 1 | `str_min`, `str_max` (1 to 25); `exc_min`, `exc_max` (0 to 100, the 18/xx percentile, 100 = 18/00, 0 and 0 = no exceptional part); `hit`, `dmg` |
      | `dexterity` | Table 2 | `dex_min`, `dex_max` (1 to 25); `reaction`, `missile`, `defensive` |
      | `bard_spells` | Table 32 | `level` (1 to 20); `per_spell_level`, exactly 6 counts of 0 or more (spell levels 1 to 6); exactly 20 rows |
      | `bard_experience` | Table 25, Thief/Bard column | `level` (1 to 20); `xp` (0 or more, the XP that reaches the level); exactly 20 rows |

      No other property anywhere in `rule_tables`: no table title, column header, note or
      `_source` comment (the seed's `_source` is dropped). Only the columns the engine reads are
      shipped; the book's other columns (weight allowance, open doors, the giants' names) stay in
      `critical-rulebooks` (`tables/phb-001`, `-002`, `-025`, `-032`). Rows are in ascending order. A
      new table, column or field is a new format (step 1).
   3. **Validation, where each rule can be checked.** The schema checks the shape, the integer
      types, the bounds and row counts above. JSON Schema cannot express the rest, which the builder
      checks before it writes (a violation fails the build) and the importer checks at start with
      the rules `ruletablesvc.New` applies to the seed today (a violation refuses the start, naming
      the table, like step 5.2): Strength and Dexterity rows cover scores 1 to 25 in
      order, contiguously and without overlap; score 18 is split into contiguous percentile ranges
      from the plain 18 (0) to 18/00 (100); `bard_spells` holds each level 1 to 20 once;
      `bard_experience` holds levels 1 to 20 in order, starting at 0 XP and strictly increasing.
   4. **Read into memory at every start, not imported into the database.** The API parses
      `rule_tables` into `ruletablesvc.Tables` at each start, as the seed adapter does today, before
      serving. Consumers keep the port `ruletablesvc.Lookup` (and `Tables.BardXPThreshold`); only its
      source changes. No migration, no table, nothing added to step 5.4's transaction or to
      `catalogue_meta`. Nothing in game state refers to a table row (no id, no join, so step 5.4.3's
      removal check has nothing to protect), the reasons of step 6 for SQLite (relational data joined
      to state) do not apply, and its 79 rows parse in well under a millisecond. Step 2 holds: play
      never writes them.
   5. **A table change takes effect at the next start.** A corrected number is a new artifact
      `version` (the builder never reuses one). Since the tables are read at every start, step 5.3's
      "same version, do nothing" governs only the spells' import: every start uses the artifact's
      tables, whatever the recorded version. This is how an edit of the seed applies today.
   6. **Rollout: no start on the live database ever fails.** In this order:
      1. **The API reads formats 1 and 3.** Format 3 takes the tables from `rule_tables` and does
         not read `<seeds>/rule_tables.json`; format 1 takes them from `<seeds>/rule_tables.json`
         as today. Any other format is refused (step 5.2). The fixture moves to format 3 (step 9.7)
         and `api/testdata/seeds/rule_tables.json` is deleted; the campaign `seeds/rule_tables.json`
         stays. This lands on `main` first, so the format-1 `.catalogue/catalogue.json` and
         `.dev/critical.db` keep starting with no action.
      2. **The builder publishes format 3** (`dist/catalogue.json` on the rulebooks default branch)
         only once 9.6.1 is on `main`. It may be built earlier on a branch, which the API item tests
         against with `CATALOGUE_REF`. From then `just catalogue` and the release build
         ([ADR 0024](0024-deployment.md)) fetch format 3, which the API of 9.6.1 reads.
      3. **Format 1 is retired**, in a later item, once the dev checkout's
         `.catalogue/catalogue.json` and the production image are format 3: the API then reads
         format 3 only (a format-1 file is refused by step 5.2, naming `just catalogue`) and
         `seeds/rule_tables.json` is deleted.

      Rejected: the API refusing format 1 as soon as it reads format 3, after the builder. The live
      `.catalogue/catalogue.json` stays format 1 until someone runs `just catalogue`, so `just dev`
      would refuse to start after a pull; and while the builder is ahead, the format-1-only API
      refuses any format-3 artifact fetched by `just catalogue` or a release build.
   7. **The fixture carries the PHB's real numbers.** `api/testdata/catalogue.json` (format 3 from
      9.6.1) holds the four tables with the same numbers as `seeds/rule_tables.json` today; its
      spells keep invented text (step 4). The numbers are game mechanics, not book text (step 7
      keeps names, descriptions, notes and headers out of this repo, and 9.2 ships none); they are
      public in `seeds/` already; and the service tests and the e2e suite assert values derived from
      them (attack adjustments, spells per day, XP thresholds), checked against the PHB pages
      (`docs/backend.md`, "Rule tables"). Invented numbers would make those tests check nothing.
   8. **`seeds/` then holds only campaign bootstrap.** After 9.6.3 it holds the character, its
      inventory (items, weapons), its skills, its spellbook and the spell effects, pending the
      question on items, weapons and skills; the unread crit-table seed is deleted separately.

10. **The class and race tables, format 4** (_proposed_, critical-reul.9; the tables are the list of
    [ADR 0030](0030-classes-and-races.md) step 23, the keys and rows below are this step's). Built by
    `scripts/class_tables.py` in `critical-rulebooks` from its hand-checked `tables/`, never transcribed.
    1. **Format 4 is format 3 plus 24 keys in `rule_tables`**, every one required; format 3 may not
       carry any of them (the schema says both). Numbers and **language-neutral codes** only (ADR 0023):
       no title, header, note or label text. Codes: classes `fighter mage cleric thief bard`; groups
       `warrior priest rogue wizard`; races `dwarf elf gnome half_elf halfling` (humans have no row);
       abilities `str dex con int wis cha`; skills `pick_pockets open_locks find_remove_traps
       move_silently hide_in_shadows detect_noise climb_walls read_languages`; the save categories of
       the character's columns `para_poison rod_staff_wand petri_poly breath spell`.
    2. **Keys and rows** (every field an integer unless named; level bands are `level_min`/`level_max`
       within 1 to 20, an open book range such as `13+` ends at 20):

       | Key | PHB table | Row |
       |---|---|---|
       | `experience` | 14, 20, 23, 25 | `level`, and the XP that reaches it per class: `fighter`, `mage`, `cleric`, `thief`, `bard` (Table 25's one column gives both rogues); 20 rows |
       | `hit_dice` | 14, 20, 23, 25 (Hit Dice) | `group`, `die`, `name_level` (the last level that rolls it), `gain_after` (fixed gain per level after it); 4 rows |
       | `constitution` | 3 | `con_min`, `con_max`, `hp`, `hp_warrior` (the bracketed value), `low_die_min` (the value a low die counts as, 0 = none) |
       | `intelligence` | 4 | `int_min`, `int_max`, `max_spell_level` (0 = none), `learn_chance`, `max_spells` (absent = the book's "All", no maximum) |
       | `wisdom` | 5 | `wis_min`, `wis_max`, `bonus_spells` (the spell levels this score adds, not cumulative), `spell_failure` |
       | `con_save_bonus` | 9 | `con_min`, `con_max`, `bonus` (scores outside the rows: none) |
       | `race_requirements` | 7 | `race`, `ability`, `min`, `max`; 30 rows |
       | `race_adjustments` | 8 | `race`, `ability`, `adjustment` (absent pair: 0) |
       | `thac0` | 53 | `level`, `warrior`, `priest`, `rogue`, `wizard`; 20 rows |
       | `saving_throws` | 60 | `group`, a level band, the five save targets (the warriors' level 0 and the 21+ rows are not shipped) |
       | `warrior_attacks` | 15 | a level band, `attacks` per `rounds` |
       | `specialist_attacks` | 35 | `weapon` (`melee light_crossbow heavy_crossbow thrown_dagger thrown_dart other_missile`), a level band, `attacks`, `rounds` |
       | `rate_of_fire` | 45 (ROF) | `weapon_type` (ADR 0030 step 23's codes with a rate: `long_bow short_bow hand_crossbow light_crossbow heavy_crossbow dagger dart sling`), `attacks`, `rounds` |
       | `wizard_spells` | 21 | `level`, `per_spell_level` (9 counts); 20 rows |
       | `priest_spells` | 24 | `level`, `per_spell_level` (7 counts); 20 rows |
       | `priest_spell_wisdom` | 24 (notes) | `spell_level`, `wis_min` |
       | `thief_skill_base` | 26 | `skill`, `base`; 8 rows |
       | `thief_skill_race` | 27 | `race`, `skill`, `adjustment`; 40 rows |
       | `thief_skill_dexterity` | 28 | `dex_min`, `dex_max`, one field per skill the table has (`pick_pockets` to `hide_in_shadows`) |
       | `thief_skill_armor` | 29 | `armor` (`none elven_chain padded_studded_leather`; leather is the baseline), `skill`, `adjustment`; 24 rows |
       | `bard_armor_penalty` | 29 (note) | `armor` (`metal`), `adjustment`: added to the elven chain column (ADR 0030 step 1) |
       | `bard_skill_base` | 33 | `skill`, `base`; 4 rows |
       | `backstab` | 30 | a level band, `multiplier` |
       | `turning_undead` | 61 | `undead` (`skeleton zombie ghoul shadow wight ghast wraith mummy spectre vampire ghost lich special`), `hd_min`/`hd_max` (both absent: by type only; `hd_max` absent: and more), `by_level`: 20 cells, one per priest level, each either `{"roll": n}` (a d20 at or above n) or `{"outcome": "turn" / "destroy" / "destroy_more" / "cannot"}`; 13 rows |

    3. **Validation.** The schema checks the shape, the codes, the bounds and the fixed row counts. The
       builder (before it writes) and the importer (at start, `ruletablesvc.NewClassTables`, a refusal
       naming the table like step 9.3) check the rest: levels 1 to 20 once and in order; XP from 0 and
       strictly increasing per class; ability rows ascending, without overlap, and covering 1 to 25 for
       Constitution, Intelligence and Wisdom; every level band table covering 1 to 20 per group and per
       weapon; every code combination (race x ability, race x skill, armour x skill, group, weapon type,
       undead) exactly once; one turning cell kind per cell. A missing table is a startup error.
    4. **Rollout, as step 9.6.** The API reads formats 3 and 4 first (format 3: no class tables,
       `Tables.Classes()` is false); it lands on `main` before the builder publishes format 4 on the
       rulebooks default branch, so a dev checkout's format-3 `.catalogue/catalogue.json` and the release
       build keep starting. Format 3 is retired by a later item once the published artifact and the dev
       checkouts are format 4, before any consumer of the class tables relies on them. _(Amended
       2026-10-18, ADR 0030's level, THAC0 and saves.)_ Format 3 is retired: the published artifact is
       format 5 and every character's level, THAC0 and saves read the class tables, so a format-3
       artifact refuses the start (`ruletablesvc.ErrNoClassTables`, naming `just catalogue`); a dev
       checkout fetches the current artifact with `just catalogue`. Test:
       `TestStartup_Step10_3_RefusesAMissingOrGappedClassTable` ("format 3 is refused").
    5. **The fixture carries the real numbers** (as step 9.7): `api/testdata/catalogue.json` is format 4,
       its class tables the real artifact's row for row (`TestRealCatalogue_Step10_5_…`, skipped without a
       format-4 `.catalogue/catalogue.json`).
    6. **Not in format 4**: DMG Table 7 (racial level limits; the rulebooks copy is an unverified OCR, to be
       hand-checked from the page image first; shipped by format 5, step 11), the classes each race allows
       (PHB chapter 2 prose), the race trait texts (localized catalogue text) and the priest spells' sphere
       codes with the cleric's sphere access (ADR 0030 steps 15 and 23). Each is a later format. The weapon
       type is not catalogue data (ADR 0030 step 23): it belongs to the character's weapons.

11. **The racial level limits, format 5** (_proposed_, critical-reul.20; ADR 0030 steps 21 and 23). Built by
    `scripts/class_tables.py` in `critical-rulebooks` from its hand-checked
    `tables/dmg-007-racial-class-and-level-limits.md` (typed cell by cell from the page image, DMG p.15), never
    transcribed. A new table is a new format (step 9.2), hence format 5 rather than a format-4 addition: a
    format-4 API refuses an unknown key, and the format number is what tells an importer which tables to require.
    1. **Format 5 is format 4 plus one key in `rule_tables`, `race_level_limits`**, required; formats 3 and 4 may
       not carry it (the schema says so). Numbers and the codes of step 10.1 only.
    2. **Rows**: `race` (`dwarf elf gnome half_elf halfling`), `class` (`fighter mage cleric thief bard`) and
       `max_level`: the printed number (1 to 20), **0 for the book's dash** (the race cannot belong to the
       class), **absent for the book's U** (no racial limit), the conventions of step 10.2's `max_spell_level`
       (0 = none) and `max_spells` (absent = no maximum). Exactly 25 rows, one per race and class. Humans have
       no row (U in every class; the builder fails if the book's human column is anything else). The book's
       Druid, Illusionist, Paladin and Ranger rows are not shipped: the app has no such class (ADR 0030, "Not
       covered"); the gnome's illusionist limit is therefore not the gnome mage's, whose cell is the dash.
    3. **Validation.** The schema checks the codes, the bounds (0 to 20) and the 25 rows. The builder and the
       importer (at start, `ruletablesvc.NewLevelLimits`, a refusal naming the table like step 9.3) check that
       every race x class combination is there exactly once. A format-5 artifact without the table is a
       startup error.
    4. **Rollout, as step 10.4.** The API reads formats 3, 4 and 5 first; it lands on `main` before the builder
       publishes format 5 on the rulebooks default branch, so a dev checkout's format-4 `.catalogue/catalogue.json`
       and the release build keep starting. Formats 3 and 4 are retired by a later item, before any consumer
       relies on the level limits.
    5. **The fixture carries the real numbers** (as step 10.5): `api/testdata/catalogue.json` is format 5, its
       `race_level_limits` the real artifact's cell for cell (`TestRealCatalogue_Step11_5_…`, skipped without a
       format-5 `.catalogue/catalogue.json`).
    6. **Not in format 5**: the † note of Table 7 (a character without exceptional prime requisites cannot pass
       the limit) and DMG Table 8 (bonus levels for high prime requisites, the optional "Exceeding Level Limits"
       rule): the limits are shown, not enforced (ADR 0030 step 21). Whether the dash rows replace the PHB
       chapter 2 prose as the source of "the classes each race allows" (step 10.6) is that item's decision; they
       agree for the five classes.

12. **The spheres, format 6** (**decided as built** 2026-10-18, critical-reul.15; ADR 0030 steps 15 and 23). Format 6 is format 5
    plus two things, both numbers-and-codes only (no book text):
    1. **Each priest spell's `spheres`**: the codes of its spheres (`^[a-z]+(_[a-z]+)*$`, at least one, each
       once), one per sphere of the raw `sphere` text (which stays, localized, never parsed): `all`,
       `astral`, `charm`, `combat`, … , Elemental as its subsphere codes (`elemental_fire`; a spell printed
       "Elemental" alone carries the four), the *Tome of Magic*'s as theirs (`thought`, `numbers`, …).
       Required on every Priest spell, absent on Wizard spells.
    2. **`rule_tables.sphere_access`**: rows `{class, sphere, access}` (`access` `major` or `minor`), one per
       class and sphere; a sphere a class has no row for is closed to it. The cleric's are ADR 0030 step
       15.1's twelve major spheres and the four Elemental subspheres minor. Minor opens levels 1 to 3 (an
       engine rule, `ruletablesvc.MinorSphereMaxLevel`, not data).
    3. Formats 3 to 5 may carry neither (schema); the importer refuses a class code it does not know, an
       unknown access and a class and sphere twice (`ruletablesvc.NewSphereAccess`).
    4. **Rollout, as step 11.4**: the API reads formats 5 and 6; a format-5 catalogue starts, its clerics
       prepare nothing. The fixture is format 6 (ten invented-text priest spells with their codes and the
       cleric's access). The builder (`critical-rulebooks`) publishing format 6 is its own item.
    Tests: `Schema_Step12_Format6Spheres (retired by step 16)` (cataloguecheck), `Parse_Step12_1_ReadsFormat6 (retired by step 16)`,
    `Parse_Step12_2_SpheresBelongToFormat6 (retired by step 16)` (catalogueadpt),
    `Startup_Step12_RefusesABrokenSphereAccessAndStillReadsFormat5 (retired by step 16)` (composition).

13. **The equipment, format 7** (**decided as built** 2026-10-18; [ADR 0033](0033-equipment.md) steps 21 to 23).
    Format 7 is format 6 plus a top-level `equipment` object and two `rule_tables` keys, numbers and codes
    only, names as language maps (English required, French optional, step 1, ADR 0023). Formats 3 to 6 may
    carry none of the three (schema).
    1. **`equipment`**, required in format 7, four non-empty lists, each row with a `code`
       (`^[a-z0-9]+(_[a-z0-9]+)*$`, unique across the four lists) and a `name`:
       - `weapons`: PHB Table 44's weapon rows (polearms and lances included, the arquebus and the helmets
         left out): `weapon_type` (ADR 0030 step 23's thirteen codes), `kind` (`melee` or `missile`; a
         hurled weapon is `missile`), `size` (`s` `m` `l`), `damage_types` (zero to two of `b` `p` `s`: the
         mancatcher, whip and scourge have none), `speed` (0 to 20), `damage_sm` and `damage_l` (as
         printed, `1d6`, `1d4+1`, `1d3-1`, `1`; both or neither), `two_handed` (for a medium character,
         PHB p.73) and, for a launcher, `fires`: its ammunition kinds. A launcher has no damage of its
         own; its ammunition rows carry the dice. **`fires` is a list, not the single kind of ADR 0033 step
         21**: the blowgun fires two kinds (the barbed dart and the needle), and every other launcher one.
         The list is non-empty and each kind is carried by some ammunition row (importer).
       - `ammunition`: `ammo_kind` (`arrow` `quarrel` `bullet` `needle` `blowgun_dart`), `damage_types`
         (one or two), `damage_sm`, `damage_l`.
       - `armour`: PHB Table 44's armours and elven chain mail: `base_ac` (Table 46 without a shield,
         -10 to 10), `skill_column` (`none` `leather` `padded_studded` `elven_chain` `metal` `none_allowed`:
         ADR 0033 step 5; `metal` is every armour a bard may wear that a thief may not) and `armor_row`
         (Table 52's eleven rows).
       - `shields`: `kind` (`buckler` `small` `medium` `body`), `ac_melee`, `ac_missile` (0 to 5). The book gives a missile figure only for the body shield (1 melee,
         2 missile); the builder reads every other shield as improving the AC by 1 against both melee and
         missiles. The buckler's single parry per round and the facing limits are not modelled.
    2. **`rule_tables.weapon_vs_armor`**: PHB Table 52, exactly one row `{armor_row, s, p, b}` per armour row.
       **`rule_tables.initiative_item`**: PHB Table 56's magic item rows `{kind[, modifier]}`, one per magic kind
       (`potion scroll ring rod staff wand misc`); the scroll's row has no `modifier` (the spell's casting
       time counts), every other kind's has one. Both required in format 7.
    3. **Validation.** The schema checks the shapes, the codes and the bounds. The builder and the importer
       (at start, `ruletablesvc.NewEquipment`, `NewWeaponVsArmor`, `NewInitiativeItems`, a refusal naming the
       row or table like step 9.3) check what it cannot: a code names one row across the four lists; a
       launcher is a missile weapon with no damage of its own and fires kinds some ammunition row carries; a
       weapon with a damage type has damage or fires; Table 52 and Table 56 have each of their rows once.
       A format-7 artifact missing the equipment or one of the tables is a startup error.
    4. **Rollout, as step 12.4**: the API reads formats 6 and 7; a format-6 catalogue starts and offers no
       equipment. The builder (`critical-rulebooks`) publishing format 7 is a separate change in that repo.
    5. **The fixture is format 7**: the equipment's real rows and numbers with invented names (every
       row `Fixture <code>`), Tables 52 and 56 real
       (`TestRealCatalogue_Step13_5_TheFixtureCarriesTheRealArtifactsEquipment`, skipped without a
       format-7 `.catalogue/catalogue.json`).
    6. **Nothing is imported into the database** (no migration): like the rule tables, the equipment is read
       from the artifact at every start (ADR 0033 step 23: a piece copies a row and keeps its code, so no
       row is ever referenced by game state). The catalogue import of spells (step 5) is unchanged.
    Tests: `Schema_Step13_Format7Equipment (retired by step 16)` (cataloguecheck), `Parse_Step13_1_ReadsFormat7 (retired by step 16)`,
    `Parse_Step13_2_EquipmentBelongsToFormat7 (retired by step 16)` (catalogueadpt), `Parse_Step13_1_ReadsFormat7 (retired by step 16)Equipment`,
    `TestParse_Step13_2_EquipmentIsRequired`, `TestParse_Step13_3_RefusesBrokenEquipment`
    (ruletablesadpt), `Startup_Step13_RefusesBrokenEquipmentAndStillReadsFormat6 (retired by step 16)` (composition).
14. **The languages, format 8** (**decided as built** 2026-10-18; [ADR 0032](0032-character-identity-and-creation-rolls.md) step 4).
    Format 8 is format 7 plus a top-level `languages` list and one `rule_tables` key; formats 3 to 7 carry neither (schema).
    1. **`languages`**, required, non-empty: rows `{code, name}`, `code` as `^[a-z0-9]+(_[a-z0-9]+)*$` (unique), `name` a language map
       (English required, ADR 0023). The PHB racial lists plus Common give eleven: `common dwarvish elvish gnomish halfling goblin
       hobgoblin kobold orcish gnoll burrowers`; a campaign language is one more row, no code change.
    2. **`rule_tables.race_languages`**, required: rows `{race, language[, native: true]}`, one per race and language of its PHB list
       (chapter 2); `native` marks the race's own language (Common for human and half-elf).
    3. **Validation** (`ruletablesvc.NewLanguages`, a refusal naming the race or language): a code once, with an English name; a racial list
       naming an unknown language or listing one twice is refused; every one of the six races has exactly one native language.
       A format-8 artifact missing either part is a startup error.
    4. **Rollout, as step 13.4**: the API reads 7 and 8; a format-7 catalogue starts and offers no language (creation options carry none,
       any language code is `unknown`). Publishing format 8 is a change in `critical-rulebooks`. Merge order: this change first; until the
       published artifact is format 8, a deployed app has no language catalogue and shows the stored codes as they are.
    5. **Nothing is imported into the database**; the data migration of stored free text uses committed spellings whose codes a test holds
       equal to the fixture's (`TestLanguageSpellings_Step8_1_AreCatalogueCodes`).
    Tests: `Schema_Step14_Format8Languages (retired by step 16)` (cataloguecheck), `Parse_Step14_1_ReadsFormat8 (retired by step 16)`, `Parse_Step14_2_LanguagesBelongToFormat8 (retired by step 16)`
    (catalogueadpt), `Parse_Step14_1_ReadsFormat8 (retired by step 16)Languages`, `TestParse_Step14_2_LanguagesAreRequired`,
    `TestParse_Step14_3_RefusesABrokenLanguageCatalogue` (ruletablesadpt), `Startup_Step14_RefusesAnUnknownLanguageAndStillReadsFormat7 (retired by step 16)` (composition).

15. **Gameplay values are marked by the API when it serves a spell** (**decided as built** 2026-10-19; user, 2026-10-19: at serve time, not at import).
    The artifact (format 8) and the database do not change.
    1. **Six kinds**, in English and French: dice (`1d12`, `3d6+2`), times (`2 turns/level`, `1 round`, `2 rounds/niveau`), levels (`15th level`,
       `per level`, `12e niveau`), distances and areas (`10 feet`, `60-foot arc`, `3 m`), percentages (`10%`) and modifiers (`+3`, `-2`). A saving
       throw is named in words, so it is not a value; neither are a year, a number in words, a "+" in prose or a digit in a spell name.
    2. **Where: at serve time.** `wire.SpellToDTO` calls `spellvalues.Mark(lang, text)` (pure Go, `domain/spellvalues`) on the description and on
       the six stat texts, in every language the spell has. One call site (`markValues`), so moving it to the import later only moves the call
       and adds storage. Nothing is stored, so a better rule applies at the next release without a catalogue version.
    3. **How it is served: `marked`**, an additive field of the spell payload (lookup, spellbook, cast result): by field name
       (`description`, `casting_time`, `range`, `duration`, `area_of_effect`, `components`, `saving_throw`), then language code (ADR 0023: the
       language is a key, never a field), a list of spans `{text, kind?}` that concatenate to exactly the served text. Only a text with a value
       is listed; any other is shown plain. The texts themselves (`description`, the stat texts) are unchanged.
    4. **The vocabulary is parsing input.** The unit words in `spellvalues` ("round", "feet", "tour", "pieds"...) are matching data for the
       books' words, like the stat parser's (`spellstats`): never emitted, never displayed. A language without a vocabulary gets only the
       language-neutral kinds (dice, percentages, modifiers). Recognition is generic: no rule names a spell.
    5. **The client recognises nothing.** The description subset's parser (step 1) is unchanged; the spans travel through it as marks and are
       rendered as React text, each value in one style (the theme's highlight accent, semibold), a table cell or a list item as in a paragraph.
       The stat tiles show the book's text with the API's spans; the stat text the client words from the typed stats (step 8, French, and the
       value at the caster level) marks the pieces it composes (unit terms, dice, "per level"), which is composition from typed values, not
       recognition. The kind is kept on the wire and in a data attribute, so a style per kind stays possible.
    Tests: `TestMark_FindsTheSixKindsInEnglishAndFrench`, `TestMark_LeavesNonValuesAlone`, `TestMark_TheFixtureCatalogueLosesNothing` (and the real
    catalogue's, when fetched), `TestSpellToDTO_MarksTheValuesOfTheDescriptionAndTheStatsPerLanguage` (api); `value marks`, `stats worded with
    their values marked`, `SpellDescription (gameplay values)` (web).
16. **The proficiency descriptions, format 9** (**decided** 2026-10-20, as built 2026-10-20; user triage of 2026-10-20: "extract them from phbs both
    in French and English, make sure they are properly formatted"; [ADR 0037](0037-proficiencies.md) step 19 shows them). Format 9 is format 8 plus
    one top-level list. It covers the 65 non-weapon proficiencies of ADR 0037 step 2; weapon proficiencies have no text of their own, and the thief
    and bard skills' descriptions are another item. English **and** French are required for every row (spells require English only, step 1).
    1. **`proficiency_descriptions`**, required, exactly 65 rows `{id, description}`: `id` a proficiency id of ADR 0037 step 2's catalogue
       (`^[a-z0-9]+(_[a-z0-9]+)*$`, once), `description` a language map with **both** `en` and `fr`, each in step 1's Markdown subset (paragraphs, bullet
       lists, pipe tables kept as tables; no headings, HTML or emphasis; no leading or trailing whitespace). Formats 3 to 8 may not carry it (schema).
       No other property: the name stays an i18n key by id (ADR 0037 step 2), the figures stay in `seeds/proficiencies.json`.
    2. **Extraction and the builder** (`critical-rulebooks`; no agent reads pages). A committed script reads the PHB OCR (English) and the
       *Manuel des Joueurs* OCR (French) and writes step 1's subset; hand fixes go in an `OVERRIDES` file keyed by id and language, applied after
       the extraction and listed by the builder. The builder **refuses to publish**, naming the id and language, when: a catalogue id has no row; a
       row's id is not in the catalogue; a row lacks `en` or `fr`, or has an empty one; a text breaks step 1's lint (the block rules, a table without its
       delimiter row or with a ragged column count, emphasis or HTML, surrounding whitespace); an id repeats. The catalogue's 65 ids are a committed copy
       refreshed with the schema by `scripts/fetch-ref` (supervisor ruling 2026-10-20, from the user's triage). The output is validated against the schema before it is written, as step 4 says.
    3. **Import** (step 5 amended). In the same transaction as the spells (step 5.4), each description becomes `localized_texts` rows, `en` and `fr`
       (ADR 0023: a key, never a column per language), keyed from the proficiency id (one table `proficiency_descriptions (proficiency_id
       primary key, description text key)`, no foreign key to the seeds, as `localized_texts` has none; supervisor ruling 2026-10-20). Rows are upserted by id and a row the
       artifact no longer lists is deleted with its texts: nothing in game state refers to a description (a pick stores the id, ADR 0037 step 12), so
       there is no step 5.4.3 refusal. The import also refuses, naming the id and language, what the builder refuses, and a catalogue id of
       `seeds/proficiencies.json` that has no description or the reverse. A second import of the same artifact changes nothing (step 5.3, same
       `format` and `version`); a failed import changes nothing (step 5.5), the texts included.
    4. **The fixture is format 9** (`api/testdata/catalogue.json`): all 65 ids with **invented** `en` and `fr` text, never book text (step 7), at least
       one with a list and one with a table, and the fixture still holds every spell id the seeds reference.
    5. **Rollout, as "Implementation notes (format 1 retired)".** The API reads **format 9 only**: a format-8 artifact (or any other) is
       refused at start as an unknown format, the message naming the format found, `this API reads format 9`, followed by the advice to run
       `just catalogue` (as format 1 was refused, step 9.6.3). Merge order: the builder publishing format 9 first (an item in `critical-rulebooks`),
       then the API change; the domain lists one format and `schemas/catalogue.schema.json` accepts format 9 only (as the amendment of 2026-10-16).
    6. **Served by the API, never by the seeds** (ADR 0037 step 19): the description is read from `localized_texts` at request time as a language map.
    7. **As built.** (a) *Import*: `importProficiencyDescriptions` (storage/postgres), in the spells' transaction, table
       `proficiency_descriptions` (migration 029); a row keeps its text key across imports and only changed texts are written. (b) *The id check*
       (a catalogue id with no description, a description for an unknown id, an id twice, a language missing) is
       `catalogue.CheckProficiencyDescriptions`, run by the composition root before the migration's seed and the import, like the spell effects' and the
       spellbook seed's checks, so a mismatch changes nothing at all. (c) *Serving shape*: a lookup by id, `GET /api/proficiencies/{id}/description`
       ([ADR 0037](0037-proficiencies.md) step 19.4; `profdescsvc`, `profdeschdl`), not a `description` on each `GET /api/characters/options` row:
       65 descriptions with tables would weigh several tens of kilobytes on a payload the creation flow reads on every open. The server reads the
       texts back from `localized_texts` **once after the import** (`DB.ProficiencyDescriptions`) and serves them from memory, as the spell catalogue
       is (a pop-up costs the database nothing, ADR 0024 step 2). (d) *Formats*: the domain lists format 9 only and the schema accepts format 9 only
       (and requires everything the formats 3 to 8 added, so its per-format conditionals are gone); `ruletablesadpt.Parse` takes no format: it reads the
       current format's tables, each one required, and the per-format branches are gone (the refusal of an older artifact is `catalogueadpt`'s, step 5.2). (e) *Switch-over*: `just catalogue` takes the rulebooks
       clone's `dist/catalogue.json` at `CATALOGUE_REF` (default the checked-out branch), so nothing in this repo is pinned: once the
       builder's format 9 artifact is on the rulebooks default branch, `just catalogue` then `just dev` import the descriptions (the recorded
       format and version differ, so the import runs). A format-8 `.catalogue/catalogue.json` is refused at start with the `just catalogue` advice.
    Tests: `TestSchema_Step16_Format9ProficiencyDescriptions`, `Schema_Step16_OnlyFormat9IsValid` (retired by step 17: `TestSchema_Step17_OnlyFormat10IsValid`) and
    `TestFixture_Step16_ProficiencyDescriptionsAreInventedForEveryId` (cataloguecheck: the 65 rows, both languages, the lint, no other format),
    `TestParse_Step16_ReadsTheProficiencyDescriptions`, `TestParse_Step16_RefusesFormat8WithTheCatalogueAdvice` (catalogueadpt),
    `TestImportCatalogue_Step16_DescriptionsInLocalizedTexts`, `TestImportCatalogue_Step16_ReimportChangesNothing`,
    `TestImportCatalogue_Step16_FailedImportChangesNothing` (storage/postgres), `TestCheckProficiencyDescriptions_Step16_RefusesAMissingLanguageOrIdMismatch`
    (domain/catalogue), `TestStartup_Step16_RefusesFormat8AndNamesJustCatalogue` and `TestStartup_Step16_RefusesADescriptionIdMismatch`
    (composition), `TestRealCatalogue_Step16_EveryDescriptionPresentInBothLanguages` (seedcheck; skipped without a format-9 `.catalogue/catalogue.json`);
    in `critical-rulebooks`: `Build_Step16_RefusesAMissingLanguageUnknownIdOrBadText` and `Extract_Step16_OverridesAreApplied`.
17. **The magical defence adjustment, format 10** (**decided as built** 2026-10-21; [ADR 0006](0006-saving-throws.md) step 9.1 decided the column,
    step 1 makes a new column a new format). Format 10 is format 9 plus one column.
    1. **`magic_defence` on every `wisdom` row** of `rule_tables` (PHB Table 5's Magical Defense Adjustment column), required, an integer from -6
       to +4 (the book's range: -6 at Wisdom 1, +4 from 18). Nothing else changes; format 9 may not carry it (the schema accepts format 10 only).
    2. **The builder** (`critical-rulebooks`) reads the column from the hand-checked Table 5 file like the table's other columns, and its gate
       spot-checks values of it. **The fixture** (`api/testdata/catalogue.json`) is format 10 with the real column (step 9.7).
    3. **Rollout, as step 16.5.** The API reads **format 10 only**: a format-9 artifact (or any older one) is refused at start as an unknown format,
       naming `this API reads format 10` and `just catalogue`. Merge order: the builder publishing format 10 first (its own PR in
       `critical-rulebooks`), then the API change; a dev checkout runs `just catalogue` once after both are merged.
    4. **Read as rules data** (ADR 0006 step 9.1): `ruletablesvc.Tables.MagicDefenceFor`, derived for every character as
       `charactersvc.Derived.MagicDefence`.
    Tests: `TestParse_Step17_ReadsFormat10WithTheMagicDefence`, `TestParse_Step17_RefusesFormat9WithTheCatalogueAdvice` (catalogueadpt),
    `TestSchema_Step17_OnlyFormat10IsValid` (cataloguecheck), `TestStartup_Step16_RefusesFormat8AndNamesJustCatalogue` (composition, formats 3 to 9),
    the rules adapter's `TestWisdomFor_Step9_MagicDefence` (ADR 0006 step 9); in `critical-rulebooks`: `scripts/check`'s Table 5 spot checks.

## Consequences

- The builder item and the importer item can proceed in parallel against this ADR and the schema:
  the builder's output must validate; the importer's tests run on the fixture and cover each of
  step 5's rules (version unchanged, new version upserts, removal of an unreferenced spell,
  refusal for a referenced one with the listing, unknown format, missing file, duplicate id).
- A deployment without the artifact cannot start. That is deliberate: a silently empty spell list
  is worse than a clear error.
- _Amended by ADR 0023: the next two bullets describe `name_fr` / `description_fr` columns, which
  migration 031 turned into `localized_texts` rows (see "Storage" in `docs/backend.md`)._
- The importer needs a migration (French name and description columns, the `key` column of step 5.4.1,
  a catalogue metadata row); that is that item's work, not this ADR's.
- French names come from the catalogue's `name.fr`, the single source of truth. The web i18n `spells`
  namespace (French names, #138) and the spellbook seed's `sheet` names are retired when the UI item
  (y9e0.7) lands.
- The description subset is part of the format: the UI renders paragraphs, bullet lists and pipe
  tables; anything richer is a new `format`.
- Other reference tables (weapons, items, rule tables, crit table, skills) keep loading from
  `seeds/` until they move to the catalogue in a later format or artifact; this ADR covers spells.
  _Amended by step 9: the rule tables move in format 3; the crit table, items, weapons and skills are
  not reference data (step 9's classification) and stay in `seeds/` for now._
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
  Since buffs carry their spell's id ([ADR 0004](0004-buffs.md) step 17), the scan also reads the
  `spell_id` of the active buffs (`active_buffs`) and of the buff snapshots (`state_key = 'buffs'`),
  the `spellId` of `buff_change` entries and the `expiredSpellIds` of `round_advance` and
  `time_passed` entries (and of their corrections): a catalogue that drops a spell still on a buff,
  live or restorable, is refused like any other reference (`TestStateSpellRefs_ScansEveryPlace`).
  Its refusal also names the catalogue the database was built from (the version in `catalogue_meta`)
  and `just catalogue`, for a database started on the wrong catalogue.
- **Spell effects seed**: every `spell_effects.json` id is checked against the catalogue at startup, as
  the spellbook is (an id the catalogue does not hold would never apply); unknown ids refuse the start,
  all listed (`TestStartup_RefusesSpellEffectsTheCatalogueDoesNotHold`,
  `TestRealCatalogue_AnUnknownSpellEffectIDIsRefused`, on the real seeds and artifact).
- _Amended by [ADR 0023](0023-localized-text.md): the Storage, REST and UI notes below name
  `name_fr` / `description_fr` and `/api/spells/names` with `name_fr`. Since migration 031 the ten
  text columns key `localized_texts` rows, every text field of a spell payload is a language map
  (`{"en", "fr"?}`) and `/api/spells/names` is `[{id, name: {en, fr?}}]`; the current shape is in
  `docs/backend.md` ("Spell catalogue") and ADR 0023 "Implementation". Kept as the record of what
  shipped._
- **Storage**: `spells.key` (unique, NULL allowed), `name_fr`, `description_fr`, `source` (migration 027)
  and the one-row `catalogue_meta (format, version)`. The fixture's version is `1970-01-01`, so switching
  between the fixture and a real artifact always imports
  (the Go test of the switch was retired with the tests that open a database, [ADR 0025](0025-postgresql-storage.md) step 6; the import tests above cover the refusal).
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
  catalogue `TestImportCatalogue_UpgradesADatabaseSeededBeforeTheCatalogue`, and, retired by [ADR 0025](0025-postgresql-storage.md) step 6, the check of a live one.

## Implementation notes (the UI, 2026-10-15)

- _Amended by ADR 0023: no `name_fr` / `description_fr` field; the client reads a language map with
  `localized(lang, text)` (`web/src/i18n/localized.ts`), see `docs/frontend.md`. Search matches the
  names in every language._
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

## Implementation notes (typed stats in the UI language, 2026-10-15)

- **History.** A first take (catalogue format 2, migration 029) shipped the French books' stat
  texts as `{en, fr}` and served them as `*_fr` fields. It broke the rule of step 8 (data holds typed
  values; conversion and translation happen at render) and was undone: the catalogue is format 1
  again, the reader never knew format 2, and migration 030 drops the six `*_fr` columns on a
  database that has 029 (a past migration: the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3).
- **Re-typing.** Migration 030 also forgets the recorded artifact (`catalogue_meta`), so the next
  startup imports it again and re-parses every spell's stats with the current parser (step 5.6).
  _Since 2026-10-15 no migration is needed for that:_ a start on the recorded version (step 5.3)
  parses every spell's stored English stat texts again and rewrites the `stats` of the rows the
  current parser reads differently, in one transaction, and writes nothing when the parser is
  unchanged (`TestImportCatalogue_Step6_AStartReTypesStatsAParserChangeMadeStale`,
  `TestImportCatalogue_Step3_SameVersionWritesNothing`, and `TestRepoSeeds_EverySpellStatParses` on the real catalogue; the real-catalogue restart test that counted rows with a trigger was retired by [ADR 0025](0025-postgresql-storage.md) step 6). Re-parsing on every start was chosen over a parser
  version recorded in `catalogue_meta`: a version has to be bumped by hand and a forgotten bump is the
  silent staleness this replaces; the `catalogue_meta` contract is unchanged, and a whole start on the
  real catalogue stays around 0.1 s. The step 5.4.3 refusal names the catalogue the database was built
  from as read before the import keys anything: without a recorded version, keyed spells are "an
  earlier spell catalogue" (the state 030 leaves), not the seeds that predate the artifact
  (`TestImportCatalogue_RefusalWithoutARecordedVersionNamesAnEarlierCatalogue`).
- **Partial area readings** are marked by the parser: the text's numbers and dice must all be the
  typed form's, and its words a length unit, a shape, a word of the noun or a connector ("x", "in
  a", "level") (`spellstats.AreaOfEffect.Partial`,
  `TestAreaOfEffect_PartialWhenTheTextSaysMore`), and serve no value at a caster level
  (`TestAreaOfEffect_PartialReadingHasNoValueAtACasterLevel`; the client's `spellValues.test.ts`
  "a partial area reading").
- **Alternatives** ("1 creature or object", "One doorway, portal, or small object", "10-ft. cube or
  one item", "20 or 40 ft. radius") are an area kind of their own, `alternatives`, whose
  `alternatives` are each a whole area or targets reading; a bare noun takes the count of the
  reading before it unless that one's noun is qualified ("1 dead human or demihuman" stays partial),
  a bare number the unit and shape of the reading after it. The client words each reading and joins
  them with `lookup.stat.or`, or shows the book's text when one cannot be worded; like a partial
  reading, alternatives have no value at a caster level, since the book does not say which one the
  caster picks (`TestAreaOfEffect_Alternatives`, `statText.test.ts` "words a choice between
  readings"). "Up to" bounds the extent or count it precedes (`Quantity.UpTo`), "circle" is a shape,
  and extents read per-level steps ("/2 levels", "/lvl", "/two levels"). A radius or diameter carries
  the figure it measures (`AreaOfEffect.Figure`: "10-ft. radius sphere" is "sphère de 3 m de rayon",
  never just a radius), and a shape word a reading carries neither as its shape nor as its figure
  makes it partial (`TestAreaOfEffect_RadiusCarriesItsFigure`); with several extents a figure is not
  worded (the book's text). Over the real catalogue
  `TestRealCatalogue_AreaReadingsAndPerLevelDurations` pins the alternatives and partial readings.
- **The wording** is `web/src/lib/statText.ts` (unit tests for every unit, noun, shape and
  condition key, the conversion and each fallback in `statText.test.ts`); the lookup's grid and meta
  line use it, and the French lookup e2e checks one spell's stats ("210 m", "60 m + 10 m/niveau").
  The conversion and the volume digits are in `statText.test.ts` ("converts lengths to metres in
  French as the French books do", "does not round a converted length, as the French books do not",
  "writes a length under a metre in centimetres, as the French books do", "keeps a volume's
  significant digits"); the pairing with the French books' stat lines is `statTextBook.test.ts`.
- **Coverage** over a catalogue: `just stat-coverage [path]` prints, per field, how many stats the
  French UI words from the typed value, shows as a label, or shows as the English text, with the
  most common English texts left (`api/cmd/spellstats` prints the typed stats it reads).

## Implementation notes (the PHB rule tables, 2026-10-16)

Step 9.6.1, the API side of the rollout (formats 1 and 3 read; format 1 still takes its tables from
`seeds/rule_tables.json`):

- **Where the tables are parsed**: `catalogueadpt.LoadArtifact` checks the artifact against the schema and
  returns an `Artifact`: the spells (`catalogue.Catalogue`, which stays spells-only: a domain primitive
  depends on no service) and, in format 3, the `rule_tables` object as written. The composition root
  (`loadRuleTables`), at every start and before serving, parses it with the seed adapter's parser
  (`ruletablesadpt.Parse`, step 9.2: same keys and rows), or reads the seed for format 1, so
  `ruletablesvc.New`'s rules apply and a refusal is `spell catalogue <path>: rule_tables: <table> table: …`
  (step 9.3). `ruletablesadpt` stays the one reader of rule-table JSON; no adapter imports another.
- **The fixture** kept its version `1970-01-01` when it moved to format 3. The recorded catalogue is
  compared on format and version (step 5.3), so a database last started on the format-1 fixture imports
  it once more (the same spells; nothing removed) and then writes nothing.
- **Tests per step** (as shipped by 9.6.1; the format-1 tests went with step 9.6.3, see below): 9.1
  `Parse_Step9_1_ReadsFormat3 (retired by step 16)` and the schema's `Schema_Step9_Format3RuleTables (retired by step 16)`; 9.2
  `Parse_Step9_2_TablesBelongToFormat3Only (retired by step 16)`; 9.3
  `TestStartup_Step9_3_RefusesTablesThatBreakTheRulesNamingTheTable` and
  `TestParse_Step9_3_RejectsBrokenTables`; 9.4 and 9.5 `TestStartup_Step9_5_TheTablesAreReadAtEveryStart`
  (a corrected table on the same version applies, a broken one refuses the start). The PHB-page value
  tests (Strength, Dexterity, bard spells per day, XP thresholds, attack resolution) run on the fixture's
  tables.

## Implementation notes (format 1 retired, 2026-10-16)

Step 9.6.3, once the rulebooks builder published format 3 (`2026-10-16.5`) and the dev checkout's
`.catalogue/catalogue.json` was format 3 (no staging or production existed yet; [ADR 0024](0024-deployment.md)
builds them from the published artifact):

- **Format 1 is refused like any unknown format** (step 5.2), before the schema: `unknown format 1: this
  API reads format 3`, followed by the advice to run `just catalogue`. The domain lists one format,
  `catalogue.FormatRuleTables`; the tables always come from the artifact, and `ruletablesadpt` has only
  `Parse` (no seed reader). `seeds/rule_tables.json` is deleted; a `rule_tables.json` left in a seeds
  directory is never read.
- **The schema still described formats 1 and 3** when 9.6.3 landed (step 9.1's original wording); the
  amendment of 2026-10-16 below narrows it.
- **Step 9.7 without the seed**: the fixture's numbers were checked against the seed's by 9.6.1; they are
  now checked against the real artifact's, lookup by lookup (every Strength score and percentile, every
  Dexterity score, every bard level), since the builder keeps one row per book line where the fixture
  merges equal rows (same numbers, different rows). Skipped without the real artifact; on CI the
  PHB-page value tests carry 9.7.
- **The live database**: nothing it stores refers to the format, so a database started on a format-1
  artifact starts on the format-3 one; the recorded catalogue is compared on format and version (step
  5.3), so the first start imports once (same spells) and the next writes nothing.
- **Tests**: 9.6.3 `Parse_Step9_6_3_RefusesTheRetiredFormat1 (retired by step 16)` and
  `TestStartup_Step9_6_3_TheTablesAreTheArtifactsAndFormat1IsRefused` (a format-1 artifact refuses the
  start even with a valid seed file beside it; a broken seed file is ignored under format 3); 9.7
  `TestRealCatalogue_Step9_7_TheFixtureCarriesTheRealArtifactsTables`; the real seeds booting without
  any rule-table seed `TestRepoSeeds_LoadThroughTheServersAdapters`.

## Amendment (schema narrowed to format 3, 2026-10-16)

Step 9.1 said the schema describes both formats. Since step 9.6.3 the API refuses format 1, and the
builder publishes format 3 only, so one document describing the artifact is enough and the builder can no
longer validate a retired shape by mistake:

- `schemas/catalogue.schema.json` accepts format 3 only: `format` is the constant 3, `rule_tables` is
  required, and the format-conditional rules are gone. A format-1 file, with or without `rule_tables`,
  fails validation, as does format 2 (`Schema_Step9_Format3RuleTables (retired by step 16)`).
- The API's embedded copy stays identical to it (the existing test), and the rulebooks builder's copy is
  refreshed from it by `scripts/fetch-ref`.

## Implementation notes (the class and race tables, format 4, 2026-10-18)

Step 10, critical-reul.9: the builder's `scripts/class_tables.py` and `scripts/check` (fresh-build
equality, the rules of 10.3, spot values against the PHB pages and ADR 0030's Kelsen example, fault
injection on the parser, the rules and the schema); in the API, `catalogue.FormatClassTables`,
`ruletablesadpt.Parse(format, raw)` and `ruletablesvc.NewClassTables`. Tests per step: 10.1
`Parse_Step10_1_ReadsFormat4 (retired by step 16)`, `TestParse_Step10_1_ReadsTheClassTables` (PHB-page values) and the
schema's `Schema_Step10_Format4ClassTables (retired by step 16)`; 10.2 the schema test and
`Parse_Step9_2_TablesBelongToFormat3Only (retired by step 16)`'s format-4 cases; 10.3
`TestParse_Step10_3_RejectsAMissingOrBrokenClassTable` and
`TestStartup_Step10_3_RefusesAMissingOrGappedClassTable`; 10.4 `Parse_Step10_4_Format3HasNoClassTables (retired by step 16)`
and the startup test's "format 3 still starts"; 10.5
`TestRealCatalogue_Step10_5_TheFixtureCarriesTheRealArtifactsClassTables`.

## Implementation notes (the racial level limits, format 5, 2026-10-18)

Step 11, critical-reul.20: the builder's `scripts/class_tables.py` (`build_race_level_limits`) and
`scripts/check` (fresh-build equality, every one of the 25 cells compared with a second reading of the page
image, Kelsen the half-elf bard unlimited, fault injection on the parser and the schema); in the API,
`catalogue.FormatLevelLimits`, `ruletablesadpt.Parse`'s format-5 case and `ruletablesvc.NewLevelLimits` /
`LevelLimits.Limit`. Tests per step: 11.1 `Parse_Step11_1_ReadsFormat5 (retired by step 16)`,
`TestParse_Step11_1_ReadsTheLevelLimits` (every cell against DMG p.15) and the schema's
`Schema_Step11_Format5LevelLimits (retired by step 16)`; 11.2 the schema test and `Parse_Step9_2_TablesBelongToFormat3Only (retired by step 16)`'s
format-5 cases; 11.3 `TestParse_Step11_3_RejectsMissingOrBrokenLevelLimits` and
`TestStartup_Step11_3_RefusesMissingOrBrokenLevelLimits`; 11.4 `Parse_Step11_4_Format4HasNoLevelLimits (retired by step 16)` and
the startup test's "format 4 still starts"; 11.5 `TestRealCatalogue_Step11_5_TheFixtureCarriesTheRealArtifactsLevelLimits`.
