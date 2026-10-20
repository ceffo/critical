# ADR 0032: Character identity and creation rolls

**Status**: accepted 2026-10-18 (decisions from the user's triage of the onboarding epic, the same
day; the details this ADR added beyond the triage were accepted by the supervisor in review). Steps 1, 2, 3 and 8 (for alignment, sex and the race detail) implemented 2026-10-18, see "As built"; step 5 (height, weight and age) and its part of step 8 implemented 2026-10-19, see "As built (step 5)"; steps 6 and 7 (the score and max HP rolls) implemented 2026-10-18, see "As built (steps 6 and 7)"; the rest is not implemented. Amends [ADR 0030](0030-classes-and-races.md) step 1 (and the sentence of
step 7.5 on level 1 hit points) and [ADR 0029](0029-users-login-and-access.md) steps 7.2 and 7.3
(character actions leave the cog menu; creation is a full-screen route).

## Context

A character's identity is free text today: race, alignment, sex, height, weight, hair, eyes and
languages are strings; age is a number from 1 to 10,000 ([ADR 0029](0029-users-login-and-access.md)
step 7.3 item 1, the `details` object). Two fields call themselves a race (the rules race of
[ADR 0030](0030-classes-and-races.md) step 1 and a free-text "race detail"), alignment and languages
accept anything, Kelsen's languages are stored as French words (against
[ADR 0023](0023-localized-text.md)), and the only help the creation form gives with the numbers is
none: scores and max HP are typed.

User, 2026-10-18 (the onboarding epic's description, verbatim in part): _"Some things should not be
free form text such as alignment and language, Sex. And what the horror is the two Race fields?"_
and, in triage, that the player can **roll or type** ability scores and max HP, that rolls are made
by the server, and that anything recognisable in the existing data is migrated.

Sources, in `critical-rulebooks` (read as reference; no rulebook text is copied here): PHB chapter 2
(races, languages), chapter 3 (the class chapters: alignment limits, hit dice), chapter 4
(alignment), Table 3 (Constitution), Table 4 (Intelligence), Table 8 (racial adjustments), Table 10
(average height and weight), Table 11 (age), Table 12 (aging effects, out of scope); DMG chapter 1
(the ability score methods I to VI). The project's ADRs and the user's rulings win where they differ
from the book; the differences are named in each step.

How the creation fits together (the screens are the design round's, not this ADR's):

```mermaid
flowchart LR
  Class["Class, race (ADR 0030)"] --> Al["Alignment (step 2)"]
  Class --> Rolls["Score rolls (step 6)"]
  Rolls --> Ab["Ability scores"]
  Ab -->|"CON"| HP["Max HP roll (step 7)"]
  Ab -->|"INT (Table 4)"| Lang["Languages (step 4)"]
  Race["Race"] --> Lang
  Race --> Num["Height, weight, age rolls (step 5)"]
  Sex["Sex (step 3)"] --> Num
  XP["Starting XP, level (ADR 0030 step 3)"] --> HP
```

## Decision

**Conventions** for every step below:

- Every refusal is a field and a code, every warning a code with its numbers (limits, counts,
  values). Both are **structured data**: the web words them in en and fr, the server sends no text
  ([ADR 0023](0023-localized-text.md)). A refusal is a 400 `invalid_input` (the existing shape of
  [ADR 0030](0030-classes-and-races.md) step 1); a warning never stops a save. The answer to a
  create or an edit, and the read of the character, carry the warnings that apply.
- A rule, a list or a range the web shows comes from the server (the form's options); the web checks
  and computes nothing, and the server checks again.
- Every roll goes through the dice port, returns a **complete breakdown** (each die, what was kept
  or dropped, the arithmetic) and writes nothing: a roll is a helper that fills a field the player
  then saves. A rolled value is validated exactly like a typed one, so the player may always
  override it, and the server does not check that a typed set of scores "could have been rolled".
- Each step gets one named test in the implementing item. Rolls are tested with every die given a
  distinct value and the exact number of dice drawn asserted, so a reused, missing or misordered
  die fails.
- This ADR describes rules, codes and dice. It does not prescribe the storage shape; the
  implementing items propose it.

### Identity

1. **The rules race is the only race.** A character's race is the code of
   [ADR 0030](0030-classes-and-races.md) step 1 (`human`, `dwarf`, `elf`, `gnome`, `half_elf`,
   `halfling`; PHB chapter 2). The free-text "race detail" is dropped from the details, the API,
   the creation and edit forms and the card, and **its stored data is discarded** (step 8). Subraces
   are out of scope. The creation's `details.race` field and its "Précision de race" label of
   [ADR 0029](0029-users-login-and-access.md) step 7.3 item 5 disappear. Refusal for a request that
   still carries it: `unknown` (unknown field, as already for a level or a THAC0).
2. **Alignment.** One of the nine PHB alignments (chapter 4): lawful good, neutral good, chaotic
   good, lawful neutral, true neutral, chaotic neutral, lawful evil, neutral evil, chaotic evil. It
   is **required at creation**, editable later from the character's details with the same check.
   The class limits come from the class chapters and are a refusal, like `race_not_allowed`:
   - **bard**: PHB chapter 3, Bard: any alignment "but must always be partially neutral", so the
     five alignments with a neutral part (lawful neutral, neutral good, true neutral, chaotic
     neutral, neutral evil) are allowed and the four others refused;
   - **thief**: PHB chapter 3, Thief: any alignment except lawful good;
   - **fighter**: any (PHB chapter 3, Fighter). **Mage**: no limit stated. **Cleric**: the book ties a priest's
     alignment to the deity's, which the app does not model; no limit is checked.

   Refusals, field `alignment`: `empty` (missing at creation, or an edit that clears it),
   `unknown` (not one of the nine), `alignment_not_allowed` (the class limit; the response names the
   class). The nine codes are `lawful_good` ... `chaotic_evil`; words and the one-line class notes
   are i18n keys. Tests: one per class limit (bard refuses each of the four, accepts each of the
   five; thief refuses lawful good only; fighter accepts nine), `empty`, `unknown`, and the edit
   path (the same check).
3. **Sex.** Required, `male` or `female`: the two rows PHB Table 10 gives for height and weight.
   Words are i18n keys. Refusals, field `sex`: `empty`, `unknown`. It is an input of the height and
   weight ranges and rolls (step 5) and has no other effect. Editable later like the alignment.
4. **Languages.** A character's languages are codes of the **language catalogue**: the languages
   of the PHB racial lists (PHB chapter 2, each race's Languages paragraph) plus Common. Free text
   is refused. A campaign language is added by adding it to the catalogue data (each with its
   localized names, [ADR 0023](0023-localized-text.md)), with no code change.
   1. **Native language, automatic.** The race's own language is always among the character's
      languages, added by the server and not removable: dwarf, elf, gnome and halfling each their own
      (PHB chapter 2); human and half-elf have none of their own (the book: humans start knowing
      their regional language, half-elves have no language of their own), so Common.
      It does not count against the limit of item 2.
   2. **The count (PHB Table 4).** The additional languages a character may know are Table 4's
      "# of Lang." for its Intelligence score (score 1: none; 2 to 8: 1; 9 to 11: 2; 12 to 13: 3;
      14 to 15: 4; 16: 5; 17: 6; 18: 7; 19 and up as the table), the native language not counted.
      Going over is a **warning, never a refusal**: `languages_over_limit` with `count` and
      `limit`. The same warning is read again when the Intelligence score is later edited below the
      languages known.
   3. **Offered, not restricted**: the form offers the whole catalogue with the
      race's PHB list marked, since the book lets the DM allow any other language; the server accepts
      any catalogue language. Refusals, field `languages`: `unknown` (not in the catalogue),
      `duplicate`.
5. **Height, weight and age.** Numbers. **Height and weight are stored metric** (centimetres and
   kilograms); the web converts to what the player's language shows (fr: metres and centimetres,
   kilograms; en: feet and inches, pounds) and back when the player types. Table 10 and Table 11
   are in inches, pounds and years, converted with 1 in = 2.54 cm and 1 lb = 0.45359237 kg.
   Aging effects (Table 12) are out of scope; hair and eyes stay free text.
   1. **The ranges.** For the race, and the sex for height and weight: height is the Table 10
      base for the sex plus the height modifier dice; weight is the base for the sex plus the
      weight modifier dice (two independent rolls); the starting age is Table 11's base age plus the
      variable dice. The range of a figure is its lowest possible total (every die 1) to its highest
      (every die at its maximum). The age range runs from the lowest starting age to the highest
      maximum age of the maximum age range column (base plus its dice at their maximum); a young
      character is thereby never warned for being young. The converted range is rounded outward to
      the stored unit, so a value the roll produces never warns. A character with no race or no sex
      (a migrated one) is checked against the union of the sexes, and with no race against nothing.
   2. **Warnings, never refusals**: `height_outside_range`, `weight_outside_range`,
      `age_outside_range`, each with `value`, `min` and `max` (stored units; age in years). The
      refusals are only about being a number: field `height`, `weight` or `age`, code
      `out_of_range` outside a sanity bound above every table value (age keeps 1 to 10,000 of the
      current rule; the implementing item proposes the two others), `not_a_number`.
   3. **The roll.** Rolls the table's dice through the dice port, one roll per figure asked:
      height rolls the race's height modifier dice (for example half-elf `2d6`), weight its weight
      modifier dice, age its variable dice. The answer is the breakdown (dice, base for the sex, the
      total in inches or pounds or years, then the stored-unit value) and fills the field; the player
      may override it. A roll needs the race (and the sex for height and weight): without them the
      refusal is `race_required` or `sex_required` on the rolled field.
6. **Ability scores: roll or type.** The player either types the six scores or picks one of the
   six methods of DMG chapter 1 (the PHB's chapter 1 lists the same six) and the server rolls. The
   method is the player's choice per character, not a campaign setting (the DMG has the
   DM choose; at a table of many players with one DM this is the table's to agree, not the app's to
   enforce). What the server rolls, and what the player arranges, per method (a "d6 roll" below is
   one die; `3d6` is three of them added):

   | Method | The server rolls | The player |
   |---|---|---|
   | I | `3d6` six times, for Strength, Dexterity, Constitution, Intelligence, Wisdom, Charisma, in that order (18 dice) | nothing; the rolls are the scores |
   | II | `3d6` twice for each of the six abilities (36 dice, in ability order, the pair for an ability together) | keeps one of the two totals of each ability |
   | III | `3d6` six times (18 dice) | assigns the six totals to the abilities |
   | IV | `3d6` twelve times (36 dice) | keeps six of the twelve totals and assigns them |
   | V | `4d6` six times (24 dice), the lowest die of each dropped, the other three added | assigns the six totals to the abilities |
   | VI | seven `d6` (7 dice); each ability starts at 8 | adds each die, whole, to one ability; no ability above 18 |

   The answer is the breakdown: every die in the order drawn, the dropped die of method V, each
   total. For I the answer is the finished scores; for II to V the totals to choose from; for VI the
   seven dice and the base of 8. The arrangement is done in the web (it is a choice, not a rule),
   and the six scores are then saved as typed values: the server validates them like any typed set,
   the range of [ADR 0030](0030-classes-and-races.md) step 1 (3 to 19) and class and race
   eligibility (`race_not_allowed`, the class minimums). Method VI's limits (the base 8, no ability above 18, and no 18 that the dice cannot make by exact
   count) come in the roll's answer as data for the screen to arrange within; they are not enforced on the
   saved scores, which may be typed. The roll shows the racial adjustments of PHB Table 8 for the chosen race next
   to the totals; the stored score is the adjusted one ([ADR 0030](0030-classes-and-races.md) step
   1). Refusals, field `method`: `unknown`. Tests: one per method, asserting the exact dice drawn
   (18, 36, 18, 36, 24 and 7) with distinct values, the order and grouping above, method V's dropped
   die being the lowest of its four (a tie drops one), and method VI's base 8.
7. **Max HP: roll or type.** The player either types the max HP (1 to 999,
   [ADR 0030](0030-classes-and-races.md) step 1) or has the server roll it. The roll takes the class,
   the Constitution score and the starting XP (the starting level `L` is the target level of
   [ADR 0030](0030-classes-and-races.md) step 3; the starting realized level, step 7.5) and, for each
   level `n` from 1 to `L` in order:
   1. up to the class group's name level (warrior and priest through 9, wizard and rogue through 10;
      [ADR 0030](0030-classes-and-races.md) step 7.1): **one hit die** of the group (warrior `d10`,
      priest `d8`, rogue `d6`, wizard `d4`) plus the Constitution hit point adjustment of PHB Table 3
      (warriors take the bracketed value, every other class at most +2), with Table 3's raised low
      dice at Constitution 20 and more, and a gain of at least 1; exactly the arithmetic of the
      level-up roll, step 7.1 of that ADR;
   2. above the name level: the **fixed gain** of step 7.2 of that ADR, no die and no Constitution
      bonus (warrior +3, priest +2, rogue +2, wizard +1).

   So a level 15 bard (rogue, name level 10) draws 10 `d6` and takes five fixed gains of 2, and a
   level 1 character draws exactly one die. The max HP is the sum of the gains. The answer is the
   breakdown, one row per level (the die rolled, the counted value, the Constitution adjustment,
   the gain, or the fixed gain), and the total; the value stays editable. This replaces the
   sentence "Level 1 hit points are entered, never rolled here" of
   [ADR 0030](0030-classes-and-races.md) step 7.5 for a created character: they may now be rolled
   (a typed value is still accepted). Refusals: field `class` `unknown` or `not_creatable`, `con`
   `out_of_range`, `xp` `out_of_range` (the ranges of step 1 of that ADR). Tests: a level 1 mage
   draws one `d4`; a level-3 fighter three `d10` in level order with the bracketed Constitution
   bonus; a level 12 bard ten dice then two fixed; Constitution 21's raised die; the gain floor of
   1; each with distinct die values and the exact count of dice.

### Existing characters

8. **Migration of the existing characters.** One data migration, run once, additive
   ([ADR 0024](0024-deployment.md) step 5.2: the release before keeps working on the schema while
   the new one is being deployed), whose mapping tables are committed data with tests, never a
   one-off script. It reads the free-text values and maps the recognisable ones, **en and fr,
   ignoring case, accents and punctuation**:
   1. alignment: the nine names in en and fr, their usual abbreviations (LG, NE, loyal bon ...) and
      "neutral" / "neutre" alone as true neutral; sex: male and female in en and fr, and "M" / "F" /
      "H"; languages: each item of a list (separated by commas, semicolons, "and" and
      "et") that names a catalogue language in either language, so Kelsen's French words become
      codes; height, weight and age: a number, with or without a recognised unit (cm, m, feet-inches,
      kg, lb, years / ans; a bare height under 3 reads as metres and otherwise as centimetres, a bare
      weight as kilograms), stored metric;
   2. **an unmapped value is cleared**, each language item on its own (the recognised ones stay),
      and the character card shows each required field left empty (alignment, sex) as **missing** and
      asks the player to pick; the rest of the card works meanwhile and nothing is blocked by it.
      A read of the character lists its missing fields as data (`missing`, a list of field codes);
   3. **the free-text race detail is discarded**, mapped or not; the rules race code
      ([ADR 0030](0030-classes-and-races.md) step 1) is untouched;
   4. a mapped alignment that the class limit of step 2 refuses (for example a thief of
      "lawful good") is **kept**, because it is the player's own recorded choice, and read with the
      warning `alignment_not_allowed`; the check refuses only a value being set;
   5. hair and eyes are untouched; the migration is idempotent (a second run changes nothing) and
      touches no character without a value to map.

   Tests: one per mapping table row family (alignment en and fr, sex, a language list with an
   unknown item, each unit of height and weight, an age), the clearing, the discarded race detail,
   idempotence, and a migration over the seed character (Kelsen keeps a race, gets languages as
   codes, and has no French word left).

## Amendments

**ADR 0030 step 1** (creation inputs). A created character also enters: its **alignment** (step 2,
required), **sex** (step 3, required, `male` or `female`), its **languages** (step 4; the native
language is automatic) and the **identity numbers** height, weight and age (step 5). The race is
the rules race alone (step 1 above); the free-text race of the details is dropped. **Rolling is
offered** for the six ability scores (step 6), the max HP (step 7) and the three numbers (step 5.3);
typing stays possible for each, and a rolled value is validated as a typed one. The refusal codes of
step 1 of that ADR gain `alignment_not_allowed`, `not_a_number` and `race_required` /
`sex_required` (rolls). Step 7.5's "Level 1 hit points are entered, never rolled here" is amended by
step 7 above for a created character. The index row says so.

**ADR 0029 steps 7.2 and 7.3.** The character actions leave the cog menu: Switch, Create, Rename and
Retire move to a **character roster screen**, and Rename is also available from the character's
details. The cog menu keeps device settings, language, account and admin. **Creating a character is a
full-screen, stepped route**, not a dialog, and onboarding (a short intro, then the first character)
is the same route; where each action sits is the design round's decision (French first,
[ADR 0027](0027-space-follows-frequency-of-use.md)). The server side of step 7.2 and 7.3 (the
endpoints, the creation making the character current, retire repairing the current character, the
`X-Character-Id` rule) is unchanged, except that the `details` object of the creation loses
`race` (step 1 above) and gains the fields of this ADR. Item 5 of the as-built amendment ("The web",
the cog menu's Character group and the creation dialog) is superseded by this paragraph when the
roster and creation items land.

## As built (steps 1, 2, 3 and 8, alignment and sex)

2026-10-18, the item that made alignment and sex codes. The rest of the ADR is not built.

- **Storage.** `characters.alignment` and `characters.sex` stay TEXT columns and hold the codes, `''`
  for missing. Migration 013 changes no column ([ADR 0024](0024-deployment.md) step 5.2: the release
  before keeps working on the schema while the new one deploys); its data step maps the stored free
  text (`mapIdentity`, the spellings in `storage/postgres/identity.go`), clears the unmapped values,
  empties `characters.race` and gives the seed character the alignment and sex of `seeds/character.json`
  when he lacks them. The new code no longer reads or writes `characters.race`. **Migration 023 drops
  it** (the contract release, once the 013 release was on staging and prod, 2026-10-19), and its data
  step runs `mapIdentity` again for the alignment and sex the release before wrote as free text after
  013 (mapped to a code, or cleared when none fits, as in 013). That release's queries list their
  columns and never name `race`, so it keeps running on the new schema; one restarted on it refuses to
  start (an unknown applied migration, [ADR 0024](0024-deployment.md) step 6.2).
- **Refusals** are field errors `details.alignment` / `details.sex` at creation (the `details.` prefix of
  that route) and `alignment` / `sex` on `PATCH /api/character/details`, codes `empty`, `unknown`,
  `alignment_not_allowed` (the web knows the class, so the response does not name it). The class
  limits are the table in `ruletablesvc/alignment.go`. A request still carrying `race` is a 400
  `bad_request` (unknown field).
- **Reads.** `GET /api/character` carries `missing` (field codes the character lacks: `alignment`,
  `sex`) and `warnings` (`{code, field}`: `alignment_not_allowed` for a kept alignment its class
  refuses, step 8.4); `race` is gone from it and from the details.
- **Options.** `GET /api/characters/options` gains `alignments` per class (the codes it admits, in the
  PHB order) and `sexes`; the creation form and the card's edit offer only those.

## As built (steps 4 and 8, languages)

2026-10-18, the item that made languages catalogue codes ([ADR 0021](0021-spell-catalogue.md) step 14 holds the artifact shape).

- **Storage.** `characters.languages` stays the JSON list; it holds codes. Migration 014 changes no column; its data step
  (`mapLanguages`, spellings in `storage/postgres/languages.go`) splits each stored item at commas, semicolons, "and" and "et", maps
  en and fr names ignoring case and accents, keeps each code once, **drops an unrecognised item and logs it** (character and word).
  Idempotent. The seed character's seed names codes.
- **Native language** is added by the server on creation and on every languages edit, and on every read (a character stored without it
  reads with it). A word that is no catalogue code (written by the release before, during a rolling deploy) is not read.
  Before a format-8 artifact is published the book is empty and nothing is hidden; an edit then accepts the codes the character already
  holds (the card sends them back with every save) and refuses any other as `unknown`, so the empty catalogue never locks an edit.
- **Count.** PHB Table 4 is committed Go data (`ruletablesvc.LanguageLimit`), applied only by the server: the form computes and shows no
  verdict; the card shows the server's warning. `GET /api/character` `warnings` gains
  `{code: "languages_over_limit", field: "languages", count, limit}`, read again at every read, so an edited Intelligence warns.
- **Refusals**, `languages` (`details.languages` at creation): `empty`, `unknown`, `duplicate`; no `too_many`/`too_long` any more.
- **Options.** `GET /api/characters/options` gains `languages` (`{code, name}`) and `raceLanguages` (`{race, native, languages}`).
  The form and the card offer the whole catalogue, the race's list first. A previous release's options lack these: no picker.

## As built (steps 6 and 7, the score and max HP rolls)

2026-10-18, the item that made the creation's rolls (backend only; the screens are the creation-flow
items'). Package `creationrollsvc`, handler `creationrollhdl`.

- **Endpoints.** `POST /api/characters/rolls/scores` (`{"method", "raceCode"}`, the race optional)
  and `POST /api/characters/rolls/hp` (`{"class", "con", "xp"}`, all three required). They need a
  session but no current character (a new player has none), roll through the dice port, write
  nothing and log nothing.
- **Methods** are the codes `I` to `VI`, the table `ruletablesvc.ScoreMethods` (rolls, dice per roll,
  the dropped lowest die, `assign`: `in_order` for I, `keep_per_ability` for II, `arrange` for III to
  V, `add_to_base` for VI; how many totals are kept; VI's `base` 8 and `max` 18). The answer carries
  the method, the rolls in the order drawn (each its dice, the index of the dropped die or null, the
  total, and the ability for I and II), PHB Table 8's `adjustments` by ability for the race given
  (null without one), and for method I alone the finished `scores`, adjustments included (the stored
  score is the adjusted one).
- **Max HP.** The starting level is ADR 0030 step 3's level at the XP; the answer is the class, that
  level, one row per level (`hitDie`, `rolled`, `counted`, `conAdjustment`, `gain`, `fixed`) and the
  `total`. One hit die's arithmetic (raised low die, the warriors' bracketed adjustment, the gain of
  at least 1) is `ruletablesvc.HitDieGain`, which the level-up roll of ADR 0030 step 7.1 uses too.
- **Refusals** are 400 `invalid_input` field errors: `method` `unknown`, `race` `unknown` (the
  `raceCode` field), `class` `unknown` / `not_creatable`, `con` and `xp` `out_of_range` (3 to 19 and
  0 to 100,000,000, step 1 of ADR 0030). A Constitution of 20 or more is thereby refused by the roll,
  as by the creation; Table 3's raised dice are tested on `HitDieGain` (Constitution 21).
- **Not here:** the method list and ADR 0034 item 4.1's level to XP table in the form's options, and
  the web types: the creation-flow item adds them with the screens.
- **Arrangement** (added by the creation-flow item: the web does no rules arithmetic, so the score a
  placement gives is the server's). `POST /api/characters/rolls/scores/arrangement`, stateless like the
  rolls: `{"method", "raceCode", "placed": {"<ability>": [values]}}`, the values being the totals the
  player put on each ability (method VI: the dice added to it; the others at most one). The answer is
  `{"method", "adjustments", "rolled", "scores"}`: `rolled` is the method's base plus the values
  (`creationrollsvc.Arrange`), `scores` that plus PHB Table 8 for the race (the stored score), by ability
  code, an ability with nothing placed and no base absent; `adjustments` null without a race. The server
  keeps no roll, so it checks each value against what one total of the method can be (3 to 18; method VI's
  die 1 to 6), not against the roll. Method VI's max applies to the dice before Table 8 (an elf's 18
  Dexterity is 19). The adjusted score is not clamped: a dwarf's Charisma 3 is 2, which the creation then
  refuses (`cha` `out_of_range`, ADR 0030 step 1). Refusals, 400 `invalid_input`: `method` / `race`
  `unknown`; `placed` `unknown` (not an ability) or `too_many` (more values than the method keeps);
  `placed.<ability>` `too_many` (two totals on one ability, II to V), `out_of_range`, `above_max` (VI).
  The web keeps only the choice of which total goes where (`lib/creationFlow.ts` `place`) and asks the
  server on every placement and on a race changed after the roll, so the scores and the Race column follow
  the race. Additive: a previous web never calls it; this web against a previous server (404) keeps the
  placement and leaves the scores to be typed.

## As built (step 5, height, weight and age)

2026-10-19, the item that made the identity numbers.

- **Reference data (accepted deviation).** The item's criterion "reference data and catalogue version updated, the loader
  refuses a malformed row" is **not met, by decision**: PHB Tables 10 and 11 are committed Go data, `ruletablesvc.IdentityRows`
  (like Table 4 and the alignment limits), converted once with the step's factors; `identity_test.go` writes every figure out
  again from the rulebook files, so a slip fails a test instead of a loader. They could move to the catalogue as a `race_identity` table (race, base heights and weights by sex,
  the five dice, the age bases); that needs the catalogue repo and a format bump, and nothing here depends on it.
- **Storage.** Migration 015 adds `characters.height_cm` and `characters.weight_kg` (`DOUBLE PRECISION NOT NULL DEFAULT 0`,
  0 = not set, kept to a tenth); the free-text `height` and `weight` stay, unread, for the release before (migration 024 dropped them). Its data step (`mapMeasures`, removed with them) parsed the recognisable texts (cm, m, `1m80`,
  feet-inches in several spellings, kg, lb; bare height under 3 = metres, 50 to 300 = cm, anything else, such as a bare `6` or
  `5.5` that may be feet, is left unset rather than read as a wrong height; bare weight = kg) and leaves the rest unset. The
  parser was `charactersvc.ParseHeight` / `ParseWeight`, used by that data step only (the compatibility keys it once served are gone, see Wire). `age` is the existing integer.
  Checked on a real PostgreSQL (throwaway database, texts `1m80`, `5'11"`, `180`, `6`, empty, `abc`, `150 lb`): 180, 180.3, 180,
  unset, unset, unset, and 68 kg for the weight; the old columns untouched; a second start changed nothing.
- **Wire (additive, step 8).** `height_cm` and `weight_kg` are JSON numbers or null in the details (PATCH, create, read); a
  non-number is 400 `not_a_number` on the field (`height`, `details.height` at creation), a value of 0 or less or above 300 cm /
  500 kg `out_of_range`. The old keys `height` and `weight` keep their string type for **one release**, so that a client of the
  release before still works against this server during the rolling deploy ([ADR 0024](0024-deployment.md) step 5.2): read as
  `"180 cm"` / `"75.5 kg"` (`""` when not set), written as any text `ParseHeight` / `ParseWeight` read, both removed since (an empty one or null
  clears it, one naming no figure is 400 `not_a_number`); when both a number key and a text key are sent, the number wins. The
  web of this release sends and reads only the number keys. Checked for real: an old-style PATCH and create against this
  server work; this release's web against the release before fails with 400 `invalid JSON body` (an unknown field), the
  unavoidable direction of a mixed window (a page served by a new machine whose request reaches an old one), seconds long.
  A release before also refuses to start on a database holding migration 015, so a rollback restores the backup (ADR 0024
  step 9.3). **Follow-up 1, done (CRI-55):** the next release removed
  the string keys `height` / `weight` from GET, PATCH and create (a body naming one is 400 `invalid JSON body`, an unknown field; a
  read no longer carries them), `FormatHeight` / `FormatWeight` and the text path of the writes with them. The web of this release sends
  and reads only the number keys, so it works against that server during the overlap. **Follow-up 2, done (CRI-84):**
  migration 024 drops the columns `characters.height` / `characters.weight`, and with them 015's data step (`mapMeasures`) and
  `charactersvc.ParseHeight` / `ParseWeight`, which could no longer be generated against a schema without the text columns (sqlc
  reads the migrations). Consequence: a database still before 015 that jumps to 024 in one run gets its heights and weights unset;
  staging, prod and current dev databases are past 015. Free text written by the release before *after* migration 015 ran lands in
  those old columns and was never parsed.
  The sanity bounds are the item's proposal (step 5.2). `GET /api/character` `warnings` gains `height_outside_range`,
  `weight_outside_range`, `age_outside_range` with `value`, `min`, `max` (stored units), read again at every read.
- **Roll.** `POST /api/characters/rolls/identity`, `{"raceCode", "sex", "figures": ["height","weight","age"]}`; the answer is
  `{"raceCode","sex","rolls":[{"figure","dice","sides","base","total","unit","value"}]}`, drawn in the order height, weight, age
  whatever the order asked. Refusals 400 `invalid_input`: `race_required` / `sex_required` on the figure that needs it, `race` /
  `sex` `unknown`, `figures` `empty` / `unknown` / `duplicate`. Needs a session, no current character; writes nothing.
- **Options** (added by the identity-step item, ADR 0034 "As built"): `raceIdentity`, each figure's dice and step 5.1 range by
  race (and sex), so the creation shows them before rolling; `POST /api/characters/preview` reads the warnings of steps 4.2 and
  5.2 on an unsaved draft.
- **Web.** `lib/measures.ts` shows and parses the numbers per language (en feet-inches and pounds, others metric); the card
  words the range warnings; the creation step and the card's edit form have a roll button per figure.

## Consequences

- The domain gains real concepts: alignment, sex, a language catalogue and measured identity numbers
  replace strings, and the form's options (alignments per class, the languages, the ranges, the
  methods) come from the server, so a campaign changes by data.
- The roll operations, the class-aware alignment check, the metric conversions and the migration
  mapping are the implementing items' work: alignment and sex (with the race detail removal), languages,
  height, weight and age, and the score and HP rolls, in that order, each citing its step of this
  ADR with one test per step. The creation route, the roster and the cog menu clean-up are the UI
  items after the design round.
- Kelsen's languages stop being French words ([ADR 0023](0023-localized-text.md)); characters whose
  free text does not map show a missing alignment or sex until the player picks, and no one is
  blocked from playing.
- A language over the Intelligence count, an unusual height, weight or age and an alignment a class
  no longer allows are warnings, so the game stays the table's; only a malformed value, a value that
  does not exist (an unknown alignment or language) and the alignment limits of a class refuse.
- Not covered: aging effects (PHB Table 12), subraces, hair and eyes as catalogue data, a cleric's
  alignment against a deity, languages learned during play (proficiency slots), dual-class and
  multi-class, the level-up flow (unchanged), and the screens' layout.
