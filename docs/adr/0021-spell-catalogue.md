# ADR 0021: Reference data: the spell catalogue artifact, storage and deployment

**Status**: accepted, 2026-10-15 (critical-y9e0.1; the contract between the catalogue builder in the
private `critical-rulebooks` repo and the API importer). Amended by [ADR 0023](0023-localized-text.md):
no per-language column or field; the `name_fr` / `description_fr` wording below is superseded

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
  `TestRealCatalogue_AnUnknownSpellEffectIDRefusesTheStartup`).
- _Amended by [ADR 0023](0023-localized-text.md): the Storage, REST and UI notes below name
  `name_fr` / `description_fr` and `/api/spells/names` with `name_fr`. Since migration 031 the ten
  text columns key `localized_texts` rows, every text field of a spell payload is a language map
  (`{"en", "fr"?}`) and `/api/spells/names` is `[{id, name: {en, fr?}}]`; the current shape is in
  `docs/backend.md` ("Spell catalogue") and ADR 0023 "Implementation". Kept as the record of what
  shipped._
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
  database that has 029 (`TestMigrate030_DropsFrenchStatColumnsKeepingTheRows`,
  `TestMigrate030_FreshDatabaseHasNoFrenchStatColumns`).
- **Re-typing.** Migration 030 also forgets the recorded artifact (`catalogue_meta`), so the next
  startup imports it again and re-parses every spell's stats with the current parser (step 5.6).
  _Since 2026-10-15 no migration is needed for that:_ a start on the recorded version (step 5.3)
  parses every spell's stored English stat texts again and rewrites the `stats` of the rows the
  current parser reads differently, in one transaction, and writes nothing when the parser is
  unchanged (`TestImportCatalogue_Step6_AStartReTypesStatsAParserChangeMadeStale`,
  `TestImportCatalogue_Step3_SameVersionWritesNothing`, on the real catalogue
  `TestRealCatalogue_ARestartReTypesStaleStats`, which counts the rows written with a trigger: all
  of them after the stats were made stale, none on the next start). Re-parsing on every start was chosen over a parser
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
