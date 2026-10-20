# ADR 0037: Proficiencies: weapon and non-weapon slots, the catalogue, picks and the budget refusal

**Status**: proposed 2026-10-20 (the decisions are the user's, from the triage of the proficiency
epic on 2026-10-19; this ADR writes them down as steps). Parts marked _(proposed)_ are readings
this ADR adds where the book leaves a figure open; they await the user's OK in the PR. Not
implemented. Amends [ADR 0002](0002-attack-resolution.md) (steps 2 and 12 and the "Not covered"
proficiency penalties), [ADR 0014](0014-skill-checks.md) (step 1 and the Consequences),
[ADR 0030](0030-classes-and-races.md) (steps 12, 17 and 18 and the "Not covered" proficiency
slots) and [ADR 0032](0032-character-identity-and-creation-rolls.md) (step 4.2), each on
implementation.

## Context

A character owns `proficiencies` rows (id, ability, modifier), reconciled from `seeds/skills.json`
for the seed bard only ([ADR 0014](0014-skill-checks.md) Consequences). There is no catalogue, no
slot arithmetic and no way to add or remove a proficiency in the app; a created character has
none. Weapon proficiencies are not modelled: an attack takes no penalty with any weapon
([ADR 0002](0002-attack-resolution.md), "Not covered"), and specialization is a per-weapon flag
([ADR 0030](0030-classes-and-races.md) step 12). The skill points of ADR 0030 steps 17 and 18 are
checked against their budget only as a warning, and ADR 0032 step 4.2 warns on a languages count
that the proficiency rules turn into slots.

The user's triage (2026-10-19) decided: both weapon and non-weapon proficiencies, following the
PHB; slots at the realized level; Intelligence's languages as bonus non-weapon slots; Table 37's
cost, +1 outside the class's Table 38 groups, +1 to the check per extra slot; languages as
"Languages, Modern" picks, the native one and Common free; a budget that refuses a save leaving
the character over budget and spending more, and only warns otherwise, for slots and skill
points alike; leftovers as a reminder, never a refusal; the full Table 37 as seed data; existing
rows become picks; one editor any time, each save one undoable move; weapon picks by weapon type,
the Table 34 penalty without one, class limits refusing the pick but never the wielding;
specialization a fighter's pick at the PHB cost, the weapon's flag derived from it; the migration
granting each character its owned weapon types.

Where the house rule differs from the book, the house rule wins: the PHB has filled slots never
reassigned (p.50) and unfilled initial slots lost (p.51); here the player edits picks at any time
and leftovers stay to spend.

Sources, in `critical-rulebooks` (read as reference; no book text is copied here): PHB Table 4
(Intelligence, p.16, "# of Lang." and the proficiency-slot use of that column), Table 34
(Proficiency Slots, p.50), "Effects of Weapon Proficiencies" (p.52), "Weapon Specialization" and
"Cost of Specialization" (p.52-53), Table 37 (Nonweapon Proficiency Groups, p.54-55), Table 38
(Nonweapon Proficiency Group Crossovers, p.55), the nonweapon cost and improvement paragraphs
(p.54 and p.56), and the class weapon lists of chapter 3: warriors (p.26), the fighter's
specialization (p.27), the wizard (p.30), the cleric (p.33), the thief (p.39) and the bard (p.41).

## Decision

Tests live where the step says. Every warning, reminder and refusal below is **structured data**
(a code and its numbers); the web words it in en and fr. Game logic stays in Go; the web shows the
server's figures and its live preview.

**Test names** below are written without their `Test` prefix while the test does not exist (the
`check-adr-tests` gate refuses a cited `Test…` name with no such function). The implementing item
writes `Test<name>` and adds the prefix here in the same PR.

### The tables and the catalogue

1. **The slot and crossover tables are data.** `seeds/proficiencies.json` (the fixture's copy in
   `api/testdata/seeds`) carries:
   - `slots`, PHB Table 34 (p.50), one row per class group with the weapon initial slots, the
     weapon #Levels, the non-proficiency penalty, the non-weapon initial slots and the non-weapon
     #Levels: warrior 4 / 3 / -2 / 3 / 3, wizard 1 / 6 / -5 / 4 / 3, priest 2 / 4 / -3 / 4 / 3,
     rogue 2 / 4 / -3 / 3 / 4;
   - `crossovers`, PHB Table 38 (p.55) for the five classes: fighter warrior and general; mage
     wizard and general; cleric priest and general; thief rogue and general; bard rogue, warrior,
     wizard and general.
   PHB Table 4's "# of Lang." (p.16) stays the committed `ruletablesvc.LanguageLimit` of ADR 0032.
   Tests: `TestProficiencyTables_Step1_Table38Crossovers` (seedcheck: Table 38's five rows),
   `TestProficiencies_Step2_OptionsWireShape` (composition: the catalogue and the groups on
   `GET /api/characters/options`, as `proficiencies` and `proficiencyGroups`),
   `ProficiencyTables_Step1_Table34AndTable38` (seedcheck: the four rows and five classes above,
   read from the seed file).
2. **The non-weapon catalogue.** The same file's `nonweapon` list holds every proficiency of PHB
   Table 37 (p.54-55) once: 65 rows. Each has an `id` (a stable snake_case code; the seed bard's
   eight ids are kept: `musical_instrument`, `tumbling`, `ventriloquism`, `riding_land`,
   `appraising`, `singing`, `reading_writing`, `local_history`; "Languages, Modern" is
   `languages_modern`), its `ability` (`str`, `dex`, `con`, `int`, `wis`, `cha`; absent for the two rows Table 37 gives
   no check, Blind-fighting and Mountaineering, "NA", whose modifier is then 0), its check
   `modifier`, its `slots` and its `groups` (`general`, `priest`, `rogue`, `warrior`, `wizard`):
   a proficiency Table 37 prints under several groups (Ancient History and Navigation under
   three, thirteen others under two) is one row listing them all, the figures being the same in
   each. Names are en/fr i18n keys by id, as the languages' are; a campaign proficiency is a data
   change (a row and its two keys), never code.
   Tests: `TestNonweaponCatalogue_Step2_Table37Rows`,
   `TestNonweaponCatalogue_Step2_SeedBardIdsKeepTheirFigures`,
   `TestNonweaponCatalogue_Step2_EveryIdHasAnEnAndFrName` (seedcheck: 65 unique ids, a valid ability
   or none, at least one known group, slots of 1 or more; the eight seed ids present with Table 37's figures;
   an i18n key per id in both locale files).

### Slots

3. **Slots available.** At the **realized** level `L` ([ADR 0030](0030-classes-and-races.md)
   step 7), with the class group's Table 34 row:
   - weapon slots = initial + `floor(L / #Levels)` (one slot at every level divisible by
     #Levels, PHB p.50);
   - non-weapon slots = initial + `floor(L / #Levels)` + Table 4's "# of Lang." for the sheet's
     Intelligence (the score as stored, no effect applied, as [ADR 0014](0014-skill-checks.md)'s
     Consequences read abilities).
   The derived block serves each pool with its terms (initial, levels gained, the Intelligence
   bonus for the non-weapon pool, available, spent, left) so the screen shows why. Examples: the
   PHB's dwarf fighter at level 1, Intelligence 11: 4 weapon and 3 + 0 + 2 = 5 non-weapon slots;
   Kelsen (rogue, level 15, Intelligence 14): 2 + 3 = 5 weapon and 3 + 3 + 4 = 10 non-weapon slots.
   Tests: `ProficiencySlots_Step3_ByGroupAndLevel` (ruletablesadpt: each group at levels 1, the
   first gain level, one below it and 20), `Derive_Step3_SlotsFollowTheRealizedLevel`
   (charactersvc: a pending level-up grants nothing; the Intelligence term; Kelsen's 5 and 10).
4. **Cost of a non-weapon pick.** Table 37's slots when one of the proficiency's groups is among
   the class's Table 38 groups, one more otherwise (PHB p.54). A bard pays 1 for Tumbling
   (rogue), a fighter 2.
   Test: `NonweaponCost_Step4_CrossoverAddsOne` (proficiencysvc: in and out of the class's
   groups, a proficiency of several groups, each of the five classes).
5. **Extra slots improve the check.** A non-weapon pick spends at least its cost; each slot spent
   beyond it adds +1 to its check target (PHB p.56). Amends [ADR 0014](0014-skill-checks.md)
   step 1: the target is the ability score + the catalogue's modifier + the extra slots; the
   check's payload gains `extraSlots`. A natural 20 still fails (ADR 0014 step 2).
   Tests: `NonweaponPick_Step5_ExtraSlotsRaiseTheTarget` (proficiencysvc), `SkillCheck_Step5_TargetCountsExtraSlots`
   (skillsvc: one d20; cost, cost + 1, cost + 3; an entry logged before reads 0).
6. **Languages are picks.** Every language of the character's list
   ([ADR 0032](0032-character-identity-and-creation-rolls.md) step 4) other than the native one
   and Common counts as one "Languages, Modern" pick of 1 slot in the non-weapon pool (the general
   group: 1 for every class). The list stays ADR 0032's storage; `languages_modern` is never
   picked on its own (`not_allowed`), and a language pick has no check. Supersedes ADR 0032 step
   4.2: Table 4's count is now the bonus slots of step 3, and `languages_over_limit` is no longer
   served; any save that changes the languages, from whichever editor, is checked by step 10 on
   the non-weapon pool.
   Tests: `LanguagePicks_Step6_EachExtraLanguageCostsOneSlot` (proficiencysvc: native and Common
   free, a half-elf whose native is Common, a dwarf with Dwarvish and Common free),
   `Languages_Step6_EditCheckedByTheBudget` (detailssvc: no `languages_over_limit` any more).

### Weapon proficiencies

7. **A weapon pick is a weapon type.** One pick per weapon type code of
   [ADR 0030](0030-classes-and-races.md) step 23, at most one per type, 1 slot (step 9 adds the
   specialist's). A class may pick a type when its PHB list allows a weapon of that type, read as
   [ADR 0033](0033-equipment.md) step 20 reads them; any other type is refused
   (`weapon_not_allowed`). _(proposed: a type is allowed when the class's step 20 list names it or
   names a weapon of it, and, for the cleric, when some catalogue weapon of that type deals
   bludgeoning damage, a launcher by its ammunition.)_ That gives:
   - fighter (warriors, PHB p.26) and bard (p.41): every type;
   - mage (p.30): `dagger`, `dart`, `sling`, `other_melee` (knife, quarterstaff);
   - cleric (blunt weapons only, p.33): `other_melee` (club, maces, flails, morning star,
     quarterstaff, warhammer), `sling`, `other_missile` (staff sling);
   - thief (p.39): `dagger`, `dart`, `hand_crossbow`, `short_bow`, `sling`, `short_sword`,
     `long_sword`, `other_melee` (club, knife, broad sword, quarterstaff), `other_thrown` (lasso).
   Wielding stays allowed: ADR 0033 step 20 still only warns.
   Tests: `WeaponPick_Step7_ClassLimitsRefuseThePick` (proficiencysvc: each class's allowed and
   one refused type, a duplicate type, an unknown code), `ClassWeaponTypes_Step7` (itemsvc: the
   lists above from the step 20 table and the catalogue).
8. **The non-proficiency penalty.** An attack with a weapon whose type has no pick takes Table
   34's penalty for the class group (warrior -2, wizard -5, priest -3, rogue -3; PHB p.50 and
   p.52) as a named to-hit modifier `nonproficient`, not magical. Amends
   [ADR 0002](0002-attack-resolution.md) steps 2 and 12: it is one of the to-hit modifiers and the
   preview shows it, listed after the buffs and before the situational modifier. A spell's to-hit
   roll takes none. _(proposed: a weapon with no weapon type takes no penalty and carries the
   warning `weapon_type_missing`, so the player sets its type; the app never guesses one.)_ The
   related-weapons half penalty is deferred (Consequences).
   Tests: `ResolveAttack_Step8_NonproficientPenaltyByGroup` (combatsvc: each group; with a pick,
   none; a launcher by its own type), `PreviewAttack_Step8_ShowsThePenalty`,
   `ResolveAttack_Step8_NoTypeNoPenaltyWarned` (combatsvc).
9. **Specialization is a weapon pick.** Fighters only (`derived.specializes`, ADR 0030 step 12;
   another class `not_allowed`), in **one** weapon type at most (PHB p.52: the fighter chooses a
   single weapon; a second `not_allowed`). The pick spends, in all (PHB p.53): **2 slots** for a
   melee weapon or a crossbow (`dagger`, `short_sword`, `long_sword`, `other_melee`,
   `light_crossbow`, `heavy_crossbow`, `hand_crossbow`), **3 slots** for a bow (`long_bow`,
   `short_bow`). _(proposed: 2 slots for `dart`, `sling`, `other_thrown` and `other_missile`, which
   the book's cost paragraph does not name.)_ A weapon pick of any other number of slots is
   refused (`out_of_range`). Amends ADR 0030 step 12: a weapon is specialized when its type has
   the specialization pick, derived on read; the per-weapon flag is no longer written or read
   (the item editor drops it), and every effect of step 12 (the bonuses, point-blank, Table 35's
   attacks of step 11) follows the derived value.
   Tests: `Specialization_Step9_FighterOnlyOneTypeAtThePhbCost` (proficiencysvc: 2 and 3 slots,
   1 or 4 refused, a mage refused, a second type refused), `Specialized_Step9_DerivedFromThePick`
   (itemsvc: two weapons of the type both specialized, the stored flag ignored),
   `ResolveAttack_Step9_SpecializationFromThePick` (combatsvc).

### The budget

10. **Refused only when over budget and spending more.** For each pool (weapon slots; non-weapon
    slots, language picks included; the skill points of [ADR 0030](0030-classes-and-races.md)
    steps 17 and 18 in total; and each thief skill's per-skill cap of step 17), a save is
    **refused** when, after it, spent > available **and** spent > the spent before it. Any other
    save is stored: over budget it is a warning (step 11), under it a reminder. A character over
    budget (existing data, an undone level-up, a lowered Intelligence) may therefore only reduce,
    or keep, the pool. Refusal: the sentinel `ErrOverBudget`, HTTP 409, code `over_budget`, a row
    in `internal/render` and an `apiError.over_budget` key (en, fr); nothing is written. Amends
    ADR 0030 steps 17 and 18 (their budget was a warning only) and their creation: at creation
    the spent before is 0, so a new character can never be created over budget.
    Tests: `Budget_Step10_RefusedWhenOverAndGrowing` (proficiencysvc: under, at, over and growing
    refused, over and equal stored, over and shrinking stored; each pool),
    `SetPoints_Step10_SkillPointsRefusedOverAndGrowing` (skillsvc: total, a thief's per-skill cap,
    the bard's total), `Create_Step10_OverBudgetRefused` (charactersvc), `ErrorMapping_Step10_OverBudget`
    (render).
11. **Warnings and reminders, never refusals.** Served on `GET /api/character` (`warnings`) and
    with every editor preview and save answer:
    - `slots_over_budget` `{pool, spent, available}` (`pool` `weapon` or `nonweapon`);
    - `slots_to_spend` `{pool, left}`: the reminder that slots wait to be spent, at creation and
      after a level-up alike;
    - the skill points keep ADR 0030's over-budget codes `skill_points_total` and
      `skill_points_per_skill`, and gain the reminder `skill_points_to_spend` `{left}`.
    Tests: `Warnings_Step11_OverBudgetAndToSpend` (charactersvc: each code with its numbers, none
    when the pool is spent exactly).

### Picks, the editor and the action log

12. **Picks are stored and edited as one list.** A character's picks are stored per character:
    the kind (`weapon` or `nonweapon`), the catalogue id or weapon type, and the slots spent. The
    ability and modifier are read from the catalogue, never stored. `PUT /api/proficiencies`
    takes the whole state, `{weapon: [{type, slots}], nonweapon: [{id, slots}], languages}`; a
    `POST /api/proficiencies/preview` of the same body answers the pools, each pick's cost, extra
    slots and check target, the warnings and the refusal it would meet, writing nothing (the
    editor's live preview). Refusals, by field: `unknown` (no such id or type, or language),
    `duplicate`, `out_of_range` (fewer slots than the cost, step 5; a weapon pick's count, step
    9), `not_allowed` (steps 6 and 9), `weapon_not_allowed` (step 7), then step 10's
    `over_budget`. The same editor serves creation, a level-up, catching up and a correction.
    Tests: `SetPicks_Step12_RefusalsWriteNothing`, `PreviewPicks_Step12_SameAnswerAsTheSave`
    (proficiencysvc), `Proficiencies_Step12_WireShape` (router).
13. **Each save is one undoable move.** A save that changes the picks or the languages is one
    `proficiencies` entry of the action log with a `proficiencies` snapshot (the picks and the
    languages before and after), as the `skill_points` move of ADR 0030 step 17: undo writes the
    state before back, redo the one after; a save that changes nothing logs nothing.
    Tests: `SetPicks_Step13_LogsOneMoveWithASnapshot`, `SetPicks_Step13_RestorerWritesThePicksBack`
    (proficiencysvc), `Proficiencies_Step13_PickUndoRedo` (composition).
14. **Creation takes picks.** The creation form uses the editor of step 12; its picks and
    languages are checked by the same code, with an empty state before (step 10). Leftover slots
    are allowed and reminded (step 11).
    Test: `Create_Step14_PicksCheckedAsTheEditor` (charactersvc).
15. **The level-up says what it grants.** The pending level-up prompt and the level-up answer
    ([ADR 0030](0030-classes-and-races.md) step 7) carry what the level grants beyond hit points:
    the weapon and non-weapon slots gained (available at the level minus available at the level
    below) and, for a rogue, the skill points gained (thief 30, bard 15). The web links to the
    editors; nothing blocks the level-up or waits for the spend.
    Tests: `LevelUp_Step15_GrantsSlotsAndPoints` (levelupsvc: a rogue at levels 4 and 5, a warrior
    at 3, a thief's and a bard's points).

### Existing data

16. **One data migration, run once.** Additive and idempotent:
    1. each `proficiencies` row becomes a non-weapon pick of its id at its cost for the class
       (step 4); the seed bard's eight rows map one to one. A row whose id is no catalogue id is
       logged (character and id) and not converted. The old rows stay in their table, read by
       nothing, until a later item drops it. `seeds/skills.json`'s `proficiencies` list is no
       longer reconciled at start ([ADR 0014](0014-skill-checks.md) Consequences): a new seed
       character gets its picks from the seed file's picks instead;
    2. each character gets one weapon pick (1 slot) per distinct weapon type among its weapons, in
       any state; a weapon without a type gives none;
    3. a fighter with a specialized weapon gets that type's pick as the specialization (step 9's
       slots); with several specialized types, the first in list order (the others plain picks,
       logged). Another class's flag is dropped (ADR 0030 step 12 refused it on every save);
    4. nothing is refused: a character left over budget sees step 11's warning.
    Tests: `Migrate_Step16_ProficiencyRowsBecomePicks`, `Migrate_Step16_OwnedWeaponTypesBecomePicks`,
    `Migrate_Step16_SpecializedFlagBecomesAPick` (storage/postgres), `Kelsen_Step16_PicksAndPools` (seedcheck:
    Kelsen's picks and pools as in the Consequences).

## Consequences

- **Kelsen** (half-elf bard, level 15, Intelligence 14) comes out of the migration at 11 of 10
  non-weapon slots: his eight proficiencies cost 1 each (all in his groups), and Elvish, Orcish
  and Dwarvish are three language picks (a half-elf's native language is Common, ADR 0032 step
  4.1). He sees `slots_over_budget` and may only reduce. His weapons give five types
  (`other_melee` for the bastard sword and the mace, `long_sword`, `short_bow`, `dagger`, `dart`):
  5 of 5 weapon slots.
- **One pick covers a whole weapon type.** The PHB gives one slot to one weapon of Table 44
  (p.51); the triage chose the ADR 0030 step 23 codes, so a pick of `other_melee` covers every
  weapon of that type (46 catalogue rows: maces and polearms, the bastard and the two-handed
  sword), and a specialization in it every one of them. A finer pick (by catalogue weapon code
  for the three `other_*` types) would be a new decision.
- The per-weapon `specialized` flag becomes derived; ADR 0030 step 12's refusal on that field
  (`not_allowed`) is replaced by step 9's on the pick.
- Every number in a warning or the preview comes from the server; the web words the codes and
  shows the terms.
- Not covered: the related-weapons half penalty (PHB p.52, optional, deferred to its own item);
  training time and teachers; secondary skills (Table 36); a proficiency's special abilities
  beyond its check (tracking's modifiers, Table 39); proficiencies granted free by a class; double
  specialization; multi- and dual-class (Table 34's best line).
