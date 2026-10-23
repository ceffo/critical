# ADR 0042: Multi-class and dual-class characters: class entries and how they combine

**Status**: accepted 2026-10-21 for what the user's triage decided (quoted in Context); the
readings where the book is silent or differs from the triage's outline (Q1 to Q16) were put to the
user and accepted as written on 2026-10-22 ("Decided with the user"). Step 1 (the class entries as the storage and wire shape, single-class
characters only) is implemented as built (see "As built" under step 1). The multi-class steps (5 to 10,
12, 14 to 17) are built 2026-10-22 (CRI-169, see each step's "As built"); the dual-class
computation (steps 8, 9, 11, 13, the dual-class slots of 15, the old class's spells and equipment
of 14 and 16, the elf of 17) is built 2026-10-22 (CRI-170, each step's "As built"). Creating and
entering a multi-class or dual-class character (steps 2 and 4) is built 2026-10-22 (CRI-171, each step's "As built");
taking a new class (step 3) is not. Amends [ADR 0030](0030-classes-and-races.md)
steps 1, 3, 5, 6, 7 and 8, [ADR 0015](0015-experience.md) steps 1 to 3,
[ADR 0037](0037-proficiencies.md) steps 3, 7 and 8 and [ADR 0040](0040-classes-are-data.md)
step 5, each in place (dated 2026-10-21) and each on implementation. Builds on the class
definitions and the lookup of ADR 0040 and the racial lift and level-limit mode of
[ADR 0041](0041-ruleset.md). The psionicist's rules as a class of a multi-class character are
[ADR 0043](0043-psionics.md)'s (steps 4.4 and 18 amend steps 10 and 16).

## Context

The table's party has, besides the half-elf bard 15 (single class): an elf fighter who became a
mage (dual-class, the mage has caught up with the fighter), a half-elf fighter/cleric around
level 13 to 14 (multi-class) and a human thief/psionicist around level 15 (multi-class). None of
them can be entered today.

Decided in triage (user, 2026-10-21):

- The book's terms: **multi-class** (several classes from the start) and **dual-class** (leaving
  one class for another; the user's "serial").
- Book rules as written (PHB chapter 3, "Multi-Class and Dual-Class Characters"; DMG chapters 3
  and 8), except: "we lifted racial restriction for multi and dual class in our game" (the
  ruleset's `multiclass_any_race` and `dualclass_any_race`, ADR 0041 step 2.2). Any other
  difference with the table is a question back to the user, not a guess.
- The outline accepted: multi-class: XP split evenly between the classes, even when one has
  reached its maximum; each class levels on its own; best THAC0, best saves, best Table 34
  proficiency line; level-up hit points averaged across the classes; a cleric multi-class keeps
  the cleric's weapon limits (warnings); wizard spells not cast in armour; thief skills limited by
  armour. Dual-class: the first class's level and XP frozen, the new class from level 1, the first
  class's abilities unusable (XP penalty if used) until the new class's level exceeds it, no hit
  points gained until then. Psionicist multi-class: the Complete Psionics Handbook's rules
  (ADR 0043).
- Racial level limits: warnings only (ADR 0041 step 5).
- Entering existing characters: level and XP per class, plus the level a dual-class left its first
  class at; max HP entered.

How a character's class, level and XP are coded today (code audit, 2026-10-21):

| What | Where it is today |
|---|---|
| The class | one code, `characters.class` (text, default `bard`, migration 001); `charactersvc.Character.Class` |
| The level | `characters.level` (the imported sheet's level) and `characters.realized_level` (migration 008, nullable, read as `COALESCE(realized_level, level)`), moved only by the level-up roll (ADR 0030 step 7) |
| The XP | `characters.xp_total`, the starting XP, plus the `xp_entries` ledger (base and a bonus flag per entry, no class); `Derived.XPTotal` is their sum (ADR 0015 step 1) |
| The target level | from `Derived.XPTotal` and the one class's XP table (ADR 0030 step 3); `Derived.TargetLevel`, `Derived.RealizedLevel` |
| Hit points | one `characters.hp_max`; the level-up adds one die of the class group plus Table 3's adjustment (`levelupsvc`, ADR 0030 step 7) |
| THAC0, saves, slots, proficiency slots | the one class group's rows at the realized level (`charactersvc`, ADR 0030 steps 5, 6, 8; ADR 0037 step 3) |
| Caster level | `Derived.RealizedLevel` (`castsvc`) |
| Class features | per-class Go lists today; ADR 0040 step 5's lookup reads them "through the class entries", one entry today |

So a character is the one-entry case of the shape this ADR decides: one class, one realized level,
one XP total.

```mermaid
erDiagram
    CHARACTER ||--|{ CLASS_ENTRY : "has 1..3"
    CLASS_ENTRY }o--|| CLASS : "is a"
    CHARACTER ||--o{ XP_ENTRY : "XP ledger"
    CHARACTER }o--|| RULESET : "plays under"
    CLASS_ENTRY {
        string class
        int realized_level
        int starting_xp
        string state "active or frozen"
        int left_at_level "frozen only"
    }
```

## Decision

Book facts are cited by file and section of `critical-rulebooks` (paths under `phb/` and `dmg/`);
no book text is copied here. "PHB multi/dual" below is
`phb/ch03-player-character-classes/15-multi-class-and-dual-class-characters.md` (PHB p.43 to 45).
**Test names** are written without their `Test` prefix while the test does not exist (the
`check-adr-tests` gate); the implementing item writes `Test<name>` and adds the prefix here.
Every warning is structured data (a code and its numbers), worded by the web.

### The class entries

1. **A character is a list of class entries.** Each entry has:
   1. `class`: a class id of the catalogue (ADR 0040 step 1.1);
   2. `realized_level`: the level taken in that class (ADR 0030 step 7's realized level, now per
      entry);
   3. `starting_xp`: the entry's XP as entered (the sheet's figure for that class); its XP is this
      plus its share of the ledger (step 4);
   4. `state`: `active` (it earns XP and levels) or `frozen` (a dual-class's old class: no XP, no
      level, PHB multi/dual, Dual-Class Benefits and Restrictions);
   5. `left_at_level`: for a frozen entry only, the level it was left at (equal to its
      `realized_level`, which never moves again).

   The entries are ordered as the classes were taken (a dual-class's first class first). A
   **single-class** character has one active entry; a **multi-class** character two or three
   active entries and no frozen one; a **dual-class** character one active entry and one or more
   frozen ones (PHB multi/dual, Dual-Class: no limit on the number of classes). Refused
   (field errors): no active entry, a class listed twice, a frozen entry without `left_at_level`
   or with one under 2 (the book requires at least 2nd level before switching), a multi-class with
   more than three classes (the book's combinations have at most three, PHB multi/dual,
   Multi-Class Combinations), a character both multi-class and dual-class _(Q1)_.
   Today's columns become the one-entry case: `class`, `COALESCE(realized_level, level)` and
   `xp_total` give one active entry, with no change of behaviour (the implementing item's data
   migration; the storage shape is its).
   Tests: `TestClassEntries_Step1_ShapesAndRefusals` (domain: single, multi, dual accepted; each
   refusal above), `TestMigrate_Step1_ACharacterBecomesOneActiveEntry` (storage, in Go without a
   database: today's class, level and XP give one entry, and every existing step test stays green).
   _As built (2026-10-22)_: the entries are the table `character_classes` (migration 035:
   `character_id`, `position`, `class`, `realized_level`, `starting_xp`, `state`, `left_at_level`;
   one row per entry, unique per character and class, with the CHECKs of the refusals above), and
   `classes.Entry` / `classes.ValidateEntries` in the domain. Its data step (`fillClassEntries`)
   runs right after the migration and gives each stored character without an entry one active
   entry (`class`, `COALESCE(realized_level, level)`, `xp_total`); a rerun changes nothing. The
   migration is additive (ADR 0024 step 5.2): `characters.class`, `level`, `xp_total` and
   `realized_level` stay and are written for the first active entry, so the release before keeps
   reading during a rolling deploy; a character that release creates meanwhile has no entry and
   the reader gives it the one its columns state. Only the default `'bard'` of `class` is gone. The
   contract step that drops the columns is a later item's, once that release is gone, and it
   must first backfill the entry-less characters that release created after the data step (the
   reader's fallback covers them until then). During the one deploy window a machine still on the
   release before writes a level-up or its undo to the columns only, so the entry of that
   character lags its column until the next write of this release (the user is the only writer;
   re-rolling the level fixes it); this is accepted. A negative level or XP in the columns (the
   app writes none) is raised to 0 by the data step, so it never aborts the boot. The wire
   serves `classes: [{class, level, startingXp, state, leftAtLevel}]` in place of `class` and
   `level` (the summary and the character); the requests (create, change) still send `class`. The
   derivation reads `classes.Primary` (the first active entry) until the multi-class math lands, so
   a single-class character derives exactly as before. `scripts/class-entries-rehearsal.sh`
   (`just class-entries-rehearsal`) proves the migration on a throwaway PostgreSQL.

2. **Who may be multi-class or dual-class, and with which classes.**
   1. **Multi-class**: the book allows only demihumans, in the combinations its race lists (PHB
      multi/dual, Multi-Class Combinations; `phb/ch02-player-character-races/`, each race). With
      the ruleset's `multiclass_any_race` true (the default, ADR 0041 step 2.2) any race may be
      multi-class, in any combination of two or three classes the catalogue defines
      _(Q2)_; with it false, the race's book combinations only (catalogue data, rows keyed
      by race, beside the level limits of ADR 0040 step 2). Each class's ability minimums and
      alignments apply (ADR 0040 step 1.4 and 1.5), and Table 7's dash is waived by the lift
      (ADR 0041 step 5).
      _(Amended 2026-10-22, user ruling: "you can't multiclass with a bard, does not make sense".)_
      **A bard is never one of a multi-class character's classes**, lift or not. The book agrees:
      only demihumans multi-class, and no race's combination names the bard (PHB multi/dual,
      Multi-Class Combinations; the bard's races are human and half-elf). The class says it in the
      catalogue (`multiclass: false`, [ADR 0021](0021-spell-catalogue.md) step 24), never by its id
      in the code: each such active entry of a multi-class character is refused at its own field
      (`classes.<i>.class`, `multiclass_class_not_allowed`), and the creation options list these
      classes (`noMultiClass`) so the form leaves them out of a multi-class character's rows.
      **Dual-class is unchanged**: the book never excludes the bard there, so a human may enter or
      leave it on step 2.2's conditions (Dexterity and Charisma, 15 to leave, 17 to enter).
      Test: `TestCreate_MultiClass_Step2_1_TheCatalogueBarsTheBard` (charactersvc: a half-elf
      thief/bard and bard/fighter/mage refused with and without the lift, a bard alone accepted, a
      human fighter-to-bard and bard-to-fighter dual-class accepted).
   2. **Dual-class**: the book allows only humans, with 15 or more in every prime requisite of the
      first class and 17 or more in every prime requisite of the new class, the current class at
      level 2 or more, and never back to a class already left (PHB multi/dual, Dual-Class Benefits
      and Restrictions). With `dualclass_any_race` true (the default) any race may; the ability,
      level and no-return conditions stay (they are not racial). The new class's alignment and
      minimums apply.
   Refusals name the field and a code (`multiclass_not_allowed`, `dualclass_not_allowed`,
   `prime_requisite_too_low`, `level_too_low`, `class_already_left`).
   Tests: `MultiClass_Step2_1_TheLiftAllowsAnyRace` (charactersvc, with a resolver mock: a human
   fighter/thief accepted with the lift, refused without it; a half-elf fighter/cleric accepted
   either way), `DualClass_Step2_2_PrimeRequisitesAndLevel` (an elf fighter to mage accepted with
   the lift; Str 14 or Int 16 refused; level 1 refused; back to a frozen class refused).

   _(As built, 2026-10-22, at creation: the body takes `classes: [{class, xp, level, leftAtLevel}]`
   in place of `class` and `xp` (both is a 400); a refusal names `classes` or
   `classes.<i>.<class|xp|level|leftAtLevel>`; the one-class body keeps its flat fields. The racial
   lift is read from the ruleset resolver (`Deps.Rulesets`, the default ruleset when nil). **The book's
   per-race combination table is not enforced**: the catalogue has no rows for it, so with the lift off a
   multi-class character needs a restricted race (demihuman) and no more, and a dual-class one a
   race that is not restricted; with the lift on DMG Table 7's dash is waived too. The prime
   requisites are checked at creation on the scores entered, 15 in the class left and 17 in the new
   one. The creation options carry `maxClasses`, `minLeftAtLevel`, `multiclassAnyRace`,
   `dualclassAnyRace`, `restrictedRaces`, `dualClassOldPrime` and `dualClassNewPrime`, so the client
   words the limits without a rule of its own. Tests, in charactersvc and, end to end with the
   party, in composition: `TestCreate_MultiClass_Step2_1_TheLiftDecidesTheRaceLimit`,
   `TestCreate_DualClass_Step2_2_PrimeRequisitesLevelAndNoReturn`,
   `TestCreate_Step1_TheShapeOfTheEntries`. The draft answer also carries `alignments` (those every
   ACTIVE class admits: a class left does not limit), `skillsClass` and `spellbookClass` (the active
   class that carries the skill points and the one that carries the starting spellbook), so the form
   shows them and computes no rule: `TestDraft_NamesTheAlignmentsAndTheCarriers`.)_

3. **Taking a new class (becoming dual-class).** A request names the new class. In one transaction
   on the action stack ([ADR 0018](0018-transactions-and-undo.md)): the active entry becomes
   frozen with `left_at_level` its realized level and its XP fixed; a new active entry starts at
   level 1 with 0 XP; the hit points and Hit Dice are kept (PHB multi/dual, Dual-Class: the new
   class starts at level 1 with no XP, the hit points are kept, the ability scores do not change).
   Undo restores the single entry. The proficiency picks, spellbook and skill points stay (the old
   class's knowledge is kept, same section).
   Tests: `TakeClass_Step3_FreezesTheOldAndStartsTheNewAtOne` (a new service: entries, XP and HP
   before and after; one action-log entry), `TakeClass_Step3_UndoRestoresTheSingleEntry`
   (composition).
   _(As built, 2026-10-22: `POST /api/character/dual-class {class}` answers `{from, to, leftAtLevel,
   xp}`; `dualclasssvc.Switch` checks step 2.2 and the new class's ability minimums and alignment
   (ADR 0040 steps 1.4 and 1.5), each refusal a 409 with its own code, then writes the whole list of
   entries and records one `class_switch` entry whose `class_entries` snapshot is that list, so undo
   and redo give the entries back exactly. The tests are in `dualclass_switch_test.go`
   (composition) and `dualclasssvc_test.go` (mocked ports).)_

4. **Entering an existing character.** A character entered from a sheet gives, per entry, its
   class, its level (the realized level) and its XP (`starting_xp`), and for each frozen entry the
   level it was left at; the max HP is entered as the sheet has it, never recomputed (decided in
   triage; ADR 0030 step 7.5). The creation form of ADR 0030 step 1 takes one to three classes;
   for a new multi-class character created in the app, the starting XP is split by step 5 and each
   entry's level comes from its share.
   Tests: `Create_Step4_EntersLevelAndXPPerEntry` (charactersvc: the party's three non-bard
   characters entered as on their sheets, levels and XP kept, max HP kept).

   _(As built, 2026-10-22: a level left out is the level the entry's XP reaches; a level entered above
   it is kept (the character is then above its target, no level-up pending); a class left with no XP
   entered takes the XP that reaches its left-at level. The skill points go to the first entry whose
   class lists rogue skills, the exceptional strength to any active entry that grants it, and the
   alignments are those every active class admits. A level entered below the one the XP reaches is
   kept as entered (the level-up is pending); a class left keeps an XP typed under the threshold of its
   left-at level (the sheet is trusted) and takes the threshold when none is typed; a level typed on a
   class left must equal its left-at level. The web form offers an optional level per active class.
   Tests:
   `TestCreate_Step4_EntersLevelAndXPPerEntry` (charactersvc and composition),
   `TestCreate_Step4_TheClassLeftsXP`,
   `TestCreate_ThePartyThroughTheDraftAndTheCreation` (composition: the elf and the half-elf
   fighter/cleric). The human thief/psionicist is created since every kind the psionicist lists
   is computed (ADR 0043 step 3, 2026-10-22: `TestCreate_Step8_ThePartyThiefPsionicistIsCreatable`).)_

### Experience and levels

5. **The XP award split** (amends [ADR 0015](0015-experience.md) step 1). The ledger stays one list
   per character (an award is the DM's to the character). Each award's amount (base plus its bonus,
   ADR 0015 step 1) goes:
   1. for a multi-class character, **in equal shares to every active entry**, even when an entry
      has reached its race's level limit (decided in triage; PHB multi/dual, first paragraph: the
      experience is divided equally between the classes). The share of an award is
      `floor(amount / n)` for `n` active entries, the remainder lost _(Q3)_;
   2. for a dual-class character, wholly to the active entry; a frozen entry's XP never changes
      (PHB multi/dual, Dual-Class: no more XP in the previous class).
   An entry's XP is its `starting_xp` plus its shares, derived on read, never stored. The ledger's
   entries, edits and undo are unchanged (ADR 0015 step 4).
   Tests: `EntryXP_Step5_1_EvenSharesEvenAtTheLimit` (xpsvc: a fighter/cleric's 1,001 XP award
   gives 500 each; a share still goes to an entry over its race's limit),
   `EntryXP_Step5_2_DualClassAllToTheActiveEntry` (the frozen entry's XP is unchanged).
   _(As built, 2026-10-22: `charactersvc.EntryXP` per entry, from the awards `xpsvc.Totals.Awards` lists
   (each with its bonus); the share is floored per award. `Derived.Classes[].XP` serves it; the character's
   `XPTotal` and the XP ledger's total and progress are the first active entry's. Both tests are in
   charactersvc, where the split is computed.)_
   _(As built for the dual-class switch, 2026-10-22: the new class starts at 0 XP by a **cut-off**, not
   by a negative starting XP: the class entry carries `ignored_awards_through` (migration 036), the id
   of the ledger's last entry when the class was taken, and earns its share of the awards with a
   greater id only. An award from before the switch, edited or removed or its removal undone, leaves
   the new class alone, and a frozen entry's XP stays its fixed starting XP. The cut-off is not on
   the wire. Test: `TakeClass_Step1_AnAwardBeforeTheSwitchLeavesTheNewClassAlone`.)_

6. **The XP bonus** (amends ADR 0015 step 2 as amended by ADR 0040 step 6). The book gives the 10%
   bonus to a character with 16 or more in all his prime requisites
   (`phb/ch03-player-character-classes/02-class-descriptions.md`, Prime Requisites). Eligible when
   every prime requisite of **every active entry's class** is 16 or more _(Q4)_; a frozen
   entry's class does not count (it earns nothing). The flag stored on each award still counts
   (ADR 0015 step 2).
   Test: `BonusEligible_Step6_EveryActiveClassesPrimeRequisites` (xpsvc: a fighter/mage of Str 16,
   Int 15 not eligible; of Str 16, Int 16 eligible; a dual-class mage of Int 16 whose frozen
   fighter's Str is 15 eligible).
   _(As built, 2026-10-22: `xpsvc` Get collects the prime requisites of every active entry's class and
   `BonusEligible` tests them all, one flag for the award.)_

7. **Target level and level-ups per entry** (amends [ADR 0030](0030-classes-and-races.md) steps 3
   and 7). Each active entry has its own target level, from its XP and its class's thresholds
   (ADR 0030 step 3, per entry), and its own pending level-ups (step 7): each class levels on its
   own (decided in triage; PHB multi/dual, the Morrison example). A frozen entry has no target
   level and nothing pending. The level-up request names the entry's class with the level
   (`level` must be that entry's lowest pending one); the pending levels the server sends carry
   their class. Several pending levels in several entries are all offered, each lowest first
   within its entry. The level and its benefits per entry follow the entry's realized level, as
   in step 7. The race's level limit is checked per entry and warns (`level_limit`, with the
   class, ADR 0041 step 5), frozen entries included.
   Tests: `Derive_Step7_PendingLevelUpsPerEntry` (charactersvc: a fighter/cleric whose fighter has
   a pending level and cleric none), `Roll_Step7_TheRequestNamesTheEntry` (levelupsvc: a level of
   the wrong entry or a frozen entry refused with `level_up_not_pending`),
   `Derive_Step7_LevelLimitWarnsPerEntry` (charactersvc).
   _(As built, 2026-10-22: `Derived.Classes[]` carries each entry's target, realized level, next threshold
   and pending levels; `Derived.LevelUp.Pending` lists them all in entry order, each `PendingLevel`
   carrying its `class` and the number of classes. The roll's body takes `class` (empty: the first active
   entry); a level of another entry, a frozen one or an unknown class is `level_up_not_pending`.
   `characterdal.SetRealizedLevel` writes the first active entry's level to the column and its row, any
   other entry's to its row only (with the max HP); the `level_up` snapshot names the class when there
   are several. The per-entry `level_limit` warning is the character's (`charactersvc.LevelLimitWarnings`, on
every read): code `level_limit`, field `level`, the entry's `class`, its level in `value` and the
race's limit in `max` (the wire's warning gains an optional `class`); it reads the ruleset's one mode,
`warn`, without asking for it. Test: `Derive_Step7_LevelLimitWarnsPerEntry` (a half-elf cleric
13/fighter 15 warned for the fighter only; a frozen elf fighter 13 warned; a human and the half-elf
bard 15 never).)_

### Combat values

8. **THAC0** (amends ADR 0030 step 5). A multi-class character uses the best (lowest) Table 53
   value among its active entries, each at its realized level (decided in triage; PHB multi/dual,
   Multi-Class Benefits and Restrictions: the most favourable combat value). A dual-class
   character uses the active entry's value while its realized level is not above every frozen
   entry's `left_at_level` (the **crossover** of step 13; PHB multi/dual, Dual-Class: the new
   class's combat and saving throw tables, the old numbers penalised). After the crossover, the
   best of the active entry and each frozen entry at its `left_at_level` _(Q5)_.
   Tests: `THAC0_Step8_BestOfTheActiveEntries` (charactersvc: a fighter 6/cleric 7 uses the
   fighter's row), `THAC0_Step8_DualClassNewClassUntilTheCrossover` (a fighter 9 to mage 9 uses the
   mage's row; at mage 10, as decided in Q5).
   _(As built, 2026-10-22, for a multi-class character: the lowest Table 53 value among the active entries
   (`charactersvc` `bestCore`). The dual-class crossover is not built (dual-class items).)_
   _(As built, 2026-10-22, for a dual-class character: `bestCore` adds the frozen entries, each at
   its left-at level, only once `classes.Crossed` holds (step 13). Test:
   `TestTHAC0_Step8_DualClassNewClassUntilTheCrossover` (charactersvc: the elf, fighter left at 9,
   uses the mage's 18 at mage 9 and the fighter's 12 at mage 10).)_

9. **Saving throws** (amends ADR 0030 step 6). For each of the five categories, the best (lowest)
   target among the active entries at their realized levels (decided in triage; PHB multi/dual,
   Multi-Class Benefits and Restrictions; `dmg/ch09-combat/14-breath-weapons.md`, saving throws:
   the most advantageous roll), category by category, so two classes' rows may mix
   _(Q6)_. A dual-class character as step 8: the active entry's rows until the crossover,
   then the best per category _(Q5)_. The race's bonus is unchanged (ADR 0030 step 22).
   _(The best-per-category is `saves.Best`, in `domain/saves`, the saves owner; the crossover and the
   dormant-entry filtering stay in `charactersvc`: [ADR 0046](0046-derived-figures.md) step 3,
   implemented 2026-10-23.)_
   Test: `Saves_Step9_BestPerCategory` (charactersvc: a fighter/mage at levels where the fighter is
   better on breath and the mage on spells takes each best).
   _(As built, 2026-10-22, for a multi-class character: per category among the active entries, in the
   same `bestCore`.)_
   _(As built, 2026-10-22, for a dual-class character: the same `bestCore` gate as step 8. Test:
   `TestSaves_Step9_DualClassBestPerCategoryAfterTheCrossover` (charactersvc).)_

### Hit points

10. **Multi-class hit points** (amends ADR 0030 step 7.1, 7.2 and 7.5).
    1. **At creation** (a new multi-class character; ADR 0032 step 7 rolls it): one hit die per
       class, summed, divided by the number of dice, rounded down, then the Constitution
       adjustment added once; if one of the classes takes Table 3's warrior column (the class's
       `hit_dice` Constitution column, ADR 0040 step 1.8), the warrior value applies (PHB
       multi/dual, Multi-Class Benefits and Restrictions; the Morrison example). At least 1.
    2. **A level-up of one entry**: that class's hit die (with Table 3's raised low dice, ADR 0030
       step 7.1) divided by the number of the character's classes, rounded down, at least 1 (same
       section: a hit die never yields less than 1); plus that class's share of the Constitution
       adjustment, the adjustment divided by the number of classes _(Q7: the share rounded down,
       a warrior class's share using the warrior column)_.
    3. **Above the entry's name level** (ADR 0030 step 7.2): the class's fixed gain, divided by
       the number of classes like a die _(Q8)_, no Constitution share.
    4. Recorded as ADR 0030 step 7.3's one `level_up` transaction, carrying the entry's class, the
       die, the divisor, the share of the adjustment and the gain.
    Max HP entered for an existing character is kept as entered (step 4).
    Tests: `CreationHP_Step10_1_AverageOfTheDicePlusConstitution` (creationrollsvc: Morrison's
    dice 6, 5 and 2 give 4 before Constitution), `Roll_Step10_2_DieDividedByTheClasses` (levelupsvc:
    a d6 of 4 for a three-class character gives 1; a 1 gives 1, never 0; distinct dice through the
    dice port), `Roll_Step10_3_FixedGainAboveTheNameLevel` (as decided in Q8).

    _(As built, 2026-10-22, steps 10.2 to 10.4: `ruletablesvc.SharedGain(die, conAdjustment, classes)`
    gives the die share (at least 1), the Constitution share (floored towards minus infinity, the warrior
    column for a warrior entry) and the gain, **at least 1 in total** (`max(1, dieShare + conShare)`), so
    one class is exactly ADR 0030 step 7.1 and a negative share never takes hit points away (decided by
    the supervisor on implementation, 2026-10-22). The `level_up` payload adds `class`, `classes`,
    `dieShare` and `conShare` for a multi-class character only; a single-class one is unchanged. Step
    10.1 (creation) does not apply to a character entered from a sheet: its max HP is kept as entered
    (step 4), so creation rolls nothing; the roll of ADR 0032 step 7 at creation stays the creation
    rolls' own.)_

    _(Amended 2026-10-21 by [ADR 0043](0043-psionics.md) step 4.4, on implementation: a psionicist entry's PSP gain at its level-up is not divided by the number of classes)_

11. **Dual-class hit points** (amends ADR 0030 step 7). Taking a new class keeps the hit points
    (step 3). While the active entry's level is not above every frozen entry's `left_at_level`, a
    level-up of it gains **no** hit points: the level is still taken (its other benefits apply),
    and the `level_up` entry records a gain of 0 with the reason `dual_class_no_hit_points` (PHB
    multi/dual, Dual-Class: no Hit Dice or hit points while advancing until the new class passes
    the old). From the first level above it, ADR 0030 step 7.1 and 7.2 for the new class, the full
    Constitution adjustment included (the Tarus example: a d10 at fighter 4 after cleric 3).
    Tests: `Roll_Step11_NoHitPointsUntilTheCrossover` (levelupsvc: a fighter 9 to mage 9 gains 0;
    mage 10 rolls a d4 plus Constitution, which at level 10 the wizard's step 7.1 still gives),
    `Roll_Step11_TheZeroLevelUpIsUndoable` (composition).
    _(As built, 2026-10-22: each pending level of the active entry up to the highest frozen
    `left_at_level` (`classes.MaxLeftAt`) is `NoHitPoints` (no die, no fixed gain; the wire's
    pending level carries `noHitPoints`). Its roll takes the level with its other benefits (slots,
    free spells), gains 0 and records `reason: dual_class_no_hit_points` in the `level_up` payload
    and the answer; an entered die is refused with that code. Tests:
    `TestDerive_Step11_NoHitPointsUntilTheCrossover` (charactersvc: the elf's mage 7 with mage 10's
    XP owes 8 and 9 without hit points and 10 with the wizard's d4),
    `TestRoll_Step11_NoHitPointsUntilTheCrossover` (levelupsvc),
    `TestRoll_Step11_TheZeroLevelUpIsUndoable` (composition).)_

### Classes' abilities together

12. **The lookup through several entries** (amends [ADR 0040](0040-classes-are-data.md) step 5). A
    character has feature kind K when an entry's class lists K at or under that entry's realized
    level (a frozen entry: its `left_at_level`); the rule reads that entry's parameters and level
    (a backstab multiplier at the thief entry's level, a turning row at the cleric entry's). Two
    entries granting the same kind each keep their own grant, read with their own level; a figure
    that is one per character (THAC0, saves, proficiency slots, hit points) combines by steps 8 to
    10 and 15, never by a class id. Before the crossover, using a frozen entry's feature is allowed
    and carries step 13's warning.
    Test: `Lookup_Step12_EachEntryReadsItsOwnLevel` (domain: a fighter 7/thief 9 backstabs at the
    thief's 9; a dual-class's frozen feature is granted at its `left_at_level`).
    _(As built, 2026-10-22: `classes.Grants(entries, def, kind)` lists one grant per entry whose class has
    the kind at the entry's `Level()` (a frozen entry's left-at level); `Derived.Features` serves them and
    `Derived.FeatureLevel` / `FeatureClass` give the granting entry's level and class to the rules that
    read one (skill points and their budget, backstab, turning, spell failure, the spellbook). A figure
    one per character takes the first entry granting it. Also tested on the sheet:
    `Derive_Step12_FeaturesAtTheGrantingEntrysLevel` (charactersvc). The skill sets read every
    `rogue_skills` grant (`Derived.SkillGrants`) (unreachable through the bard since step 2.1's
    amendment of 2026-10-22, which bars a thief/bard; kept, generic for any two classes with skill sets): a thief/bard has the thief's and the bard's sets,
    each with its points and its budget at its own entry's level, and a spend names the set, checked
    against the class listing it at that entry's level: `Derive_Step12_TwoSkillGrantsKeepTheirOwnSets`,
    `SetPoints_Step12_EachGrantSpendsItsOwnSet`.)_
    _(As built, 2026-10-22, weapon specialization, supervisor decision on the book: only a
    single-class fighter specializes. PHB p.52 (Proficiencies, Weapon Specialization): "a fighter
    (only)", and next to it "multi-class characters cannot use weapon specialization; it is
    available only to single-class fighters". So `Derived.Specializes` and the specialization pick
    need one active entry: a fighter/thief, the fighter first, does not specialize and its
    specialization pick is refused `not_allowed` (`skills.Classes.specialist`). The book is silent
    on a dual-class character; its one active entry is read as single-class: `Derived.Specializes`
    is that entry's own grant, never a frozen fighter's (the elf's mage does not specialize), and
    a new specialization pick is the active class's. A specialization pick kept from an old class
    that specializes survives a save and counts past the crossover only
    (`skills.WeaponProficiencyOf` reads the stored entries, a frozen one at its left-at level).
    Tests: `Derive_Step12_OnlyASingleClassFighterSpecializes`,
    `WeaponPick_Step15_OnlyASingleClassFighterSpecializes`,
    `TestDerive_Step12_ADualClassSpecializesByItsActiveClass` (charactersvc),
    `TestWeaponProficiency_Step13_AKeptSpecializationCountsPastTheCrossover` (skills).)_

13. **The dual-class crossover and the old class's abilities.** The **crossover** is reached when
    the active entry's realized level is above every frozen entry's `left_at_level` (PHB
    multi/dual, Dual-Class: higher than his maximum level in any previous class). Before it, the
    book lets the character use the old class's abilities but costs him the XP of that encounter
    and half the XP of the adventure; it also counts the old attack and saving throw numbers and
    the weapons and armour the new class prohibits (same section). The app: an action that uses a
    frozen entry's feature, spell progression or equipment allowance before the crossover is
    allowed and carries the warning `dual_class_old_ability` (with the class), shown on the action
    and in its journal entry; the XP award is entered as the DM gives it, the app computes no
    halving _(Q9)_. After the crossover nothing is warned, but each action still follows
    the limits of the class it uses (same section: a dual-class fighter/mage casts no spell in
    armour; step 16).
    Tests: `DualClass_Step13_CrossoverAboveEveryLeftAtLevel` (domain: mage 9 after fighter 9 is
    before, mage 10 after), `Cast_Step13_AnOldClassSpellWarnsBeforeTheCrossover` (castsvc).
    _(As built, 2026-10-22: `classes.Crossed` (the active entry's realized level above
    `classes.MaxLeftAt`, true without a frozen entry) and `classes.Dormant` (a frozen entry before
    it); the derived entry and each feature grant carry `Dormant` (domain only, not on the wire);
    the code is `classes.WarnOldAbility`. Tests: `TestDualClass_Step13_CrossoverAboveEveryLeftAtLevel`
    (classes), `TestDerive_Step13_TheOldClassIsDormantUntilTheCrossover` (charactersvc).
    **Spells** (with step 14): a frozen spellcasting entry keeps its slots at its left-at level and
    is a caster entry (`Derived.CasterEntry` / `Casters`, the active ones first, so an untagged slot
    is an active entry's when one casts, else the frozen one's): it prepares in its own slots and
    casts at its own level. A cast from a dormant entry answers and logs `warnings:
    ["dual_class_old_ability"]` with `class`, the casting entry's class on a character of several
    class entries; the cast's warnings are now in the logged payload (additive fields), so
    `spell_in_armour` is logged too. The preparation itself is not warned: the cast is the use.
    Tests: `TestCast_Step13_AnOldClassSpellWarnsBeforeTheCrossover` (castsvc: a mage left at 9, now a
    cleric 5, casts at 9 from the mage's slot, warned on the answer and in the journal payload; past
    the crossover not), `TestPrepare_Step13_TheOldClassPreparesInItsOwnSlots` (prepsvc).
    **Equipment** (with step 16): `itemsvc.Warnings` also reads the frozen entries' equipment
    (`itemsvc.OldClasses`): a piece the active classes do not allow but an old class does is not
    warned past the crossover and warns `dual_class_old_ability` before it (instead of
    `armour_not_allowed` / `weapon_not_allowed`; the AC card's armour warnings follow). An old class
    only adds allowance (`classes.AllowedWithOld`): a frozen `blunt_only` class restricts no weapon
    of the active ones, and each action follows the limits of the class using it, so an active
    `blunt_only` class decides alone: a cleric who left the fighter at 9 is warned
    `weapon_not_allowed` for a sword, past the crossover too (supervisor decision 2026-10-22,
    reversing the first build's reading). The item warning carries the code only (no class: the
    character's old classes are on its sheet). The weapon picks read the same rule: an old class
    adds the types it may pick, before and after the crossover (a pick is not a use: the wielding
    warns). Tests: `TestClassWarnings_Step13_AnOldClassAllowanceWarnsBeforeTheCrossover` (itemsvc),
    `TestWeaponPick_Step16_1_TheOldClassAllowsThePicks` (skills).
    **Feature actions** (the user's default for Q9, 2026-10-22: the old ability's use is a warning
    only). A figure the character has without choosing to use it is the active classes' own
    before the crossover: the attack form's rate reads `Derived.ActiveGrant`, which skips a dormant
    grant (the elf at mage 7 has no fighter 3/2, at mage 10 it has), and a kept specialization gives
    no bonus (above, step 12). An action that uses the old class anyway is allowed and answers and
    logs `warnings: ["dual_class_old_ability"]` (additive fields on each payload): an attack with a
    backstab only a dormant entry grants or with a weapon only an old class allows (itemsvc's
    warning on the piece, read only for a character with a dormant entry), a turn whose
    `turn_undead` grant is dormant (read at the old class's level), and a thief skill check of a
    set whose `rogue_skills` grant is dormant (`skills.SkillSet.Dormant`, domain only). Past the
    crossover nothing warns and the old figures count (best of both). Tests:
    `TestGetAttackForm_Step13_NoOldAttackRateBeforeTheCrossover`,
    `TestResolveAttack_Step13_AnOldClassBackstabWarnsBeforeTheCrossover`,
    `TestResolveAttack_Step13_AnOldClassWeaponWarnsBeforeTheCrossover` (combatsvc),
    `TestTurn_Step13_AnOldClassTurnWarnsBeforeTheCrossover` (turnsvc),
    `TestThiefSkill_Step13_AnOldClassSkillWarnsBeforeTheCrossover` (skillsvc),
    `TestDerive_Step13_TheOldClassAddsNoFigureBeforeTheCrossover`,
    `TestDerive_Step13_TheOldClassSkillSetIsDormantUntilTheCrossover` (charactersvc), and end to
    end `TestCast_Step13_AnOldClassCastIsWarnedAndLogged` (composition). The web has no wording
    for these warnings yet.)_

14. **Spells: one progression per spellcasting entry** (amends ADR 0030 step 8). Each entry whose
    class has a `spell_progression` has its own slots (its table at its realized level, plus the
    Wisdom bonus spells when its class has `wisdom_bonus_spells`), its own preparation and its own
    caster level, the entry's realized level (a frozen entry's `left_at_level`) (PHB multi/dual,
    the Tarus example: spells as a 3rd-level cleric, fighting as a 4th-level fighter). A
    cleric/mage prepares cleric spells in the cleric's slots and wizard spells in the mage's; a
    preparation and a cast name the entry whose slot they use. A frozen entry's spells before the
    crossover warn (step 13).
    Tests: `Derive_Step14_SlotsPerSpellcastingEntry` (charactersvc: a cleric 5/mage 6 has Table 24
    at 5 with the Wisdom bonus and Table 21 at 6), `Cast_Step14_CasterLevelIsTheEntrysLevel`
    (castsvc).
    _(As built, 2026-10-22: `Derived.Classes[].SpellSlots` per entry; the character's `SpellSlots` and
    `Derived.CasterLevel()` are the first active spellcasting entry's, which castsvc, prepsvc and
    spheresvc read. A preparation and a cast name the entry: a stored slot carries `class` when the
    character has two active spellcasting entries (no migration: the slot list is a JSON blob; an
    untagged slot is the first entry's). `prepsvc.InfoFor` / `PrepareFor` (`GET
    /api/spells/prepared?class=`, the PUT body's optional `class`) read that entry's spells
    (`Spellbook.Of`: the sphere spells for a class with a sphere access within its own slots, else the
    stored spellbook), its limits and level; the preparation replaces only that entry's slots and
    its journal payload names the class. A cast reads its slot's entry for the caster level and
    step 16.2's armour check; a slot a cast grants keeps the entry. Tests:
    `Prepare_Step14_EachEntryPreparesInItsOwnSlots` (prepsvc),
    `Spheres_Step14_EachEntryPreparesFromItsOwnSource` (spheresvc), `Cast_Step14_TheSlotNamesItsEntry`
    (castsvc). The by-group spell effects (ADR 0030 step 2) read every active entry's group, and a
    multi-class character takes the best die (PHB Strength: multi-class characters use the best die):
    the highest maximum at the caster level, then minimum, then cap, with that group's cap.
    Test: `Effects_Step14_ByGroupGainTakesTheBestDie` (castsvc). Spell failure (ADR 0030 step 14) is per
    entry: each entry whose class grants `spell_failure` carries its chance
    (`EntryDerived.SpellFailure`; the sheet's one figure is the first granting entry's), and a cast
    rolls the d100 against the casting entry's chance only, so a cleric/mage's wizard spells roll
    none. Tests: `Derive_Step14_SpellFailurePerEntry` (charactersvc),
    `Cast_Step14_SpellFailureIsTheCastingEntrys` (castsvc). The spell lookup's "Mine" filter
    (`spheresvc.SpellKeys`) holds the spells of every spellcasting entry's source, a dual-class's
    frozen one too, each once (a cleric/mage: the cleric's sphere spells within its slots and the
    stored spellbook). Tests: `Spheres_Step14_MineHoldsEveryCastingEntrysSpells`,
    `TestSpheres_Step14_MineHoldsTheOldClassSpells` (spheresvc). The web does not yet send a class:
    a cleric/mage prepares and casts only through the first entry from the UI.)_

15. **Proficiency slots** (amends [ADR 0037](0037-proficiencies.md) step 3). The book (PHB
    multi/dual, Multi-Class Benefits and Restrictions): the **largest** initial number of slots of
    the classes, then new slots at the **fastest** rate. Read per pool, each column of Table 34 on
    its own, so the initial count and the rate may come from two groups (a fighter/cleric: weapon
    4 initial every 3 levels from the warrior row; non-weapon 4 initial from the priest row, every
    3) _(Q10: the book's, not the triage's "best line")_. The levels gained are counted on the level
    of the entry whose rate is used (the highest such level when several share it)
    _(Q11)_. The weapon picks are one pool for the character (decided in triage).
    The non-proficiency penalty (ADR 0037 step 8) is the smallest among the active entries'
    groups _(Q12)_. A non-weapon proficiency costs Table 37's slots when one of its
    groups is among any active entry's Table 38 groups (ADR 0037 step 4) _(Q13)_.
    A dual-class character keeps every pick of the old class (PHB multi/dual, Dual-Class: the old
    class's proficiencies are kept); its slots are the frozen entries' available at their
    `left_at_level` plus the new class's gained slots (`floor(L / #Levels)`, no second initial
    count) _(Q14)_.
    Tests: `ProficiencySlots_Step15_LargestInitialFastestRate` (ruletablesvc: the fighter/cleric
    above at levels 1, 3, 4 and 12), `ProficiencySlots_Step15_DualClassKeepsAndGains`,
    `NonproficientPenalty_Step15_SmallestOfTheGroups` (charactersvc).
    _(As built, 2026-10-22: `skills.Classes` (the active entries at their realized levels) and
    `skills.FiguresOf` combine Table 34 per column, `skills.GrantsOf` the slots a level of one entry
    grants; the penalty and the in-group cost of Q12 and Q13 are on the pools. Tests in the skills package
    with the code: `ProficiencySlots_Step15_LargestInitialFastestRate`,
    `NonproficientPenalty_Step15_SmallestOfTheGroups`, `NonweaponCost_Step15_AnyActiveClassesGroups`,
    `LevelGrants_Step15_TheCombinedPoolsAtTheEntrysLevel`. The attack's penalty
    (`skills.WeaponProficiencyOf`, read from the stored row's active entries) is the same smallest
    one: `WeaponProficiency_Step15_SmallestPenaltyOfTheActiveClasses` (a mage/fighter takes -2, a
    mage/thief -3). The dual-class slots are not built.)_
    _(As built, 2026-10-22, the dual-class slots (Q14): `skills.ClassLevel.Frozen` marks a frozen
    entry, passed at its left-at level (`Character.ProficiencyClasses`); the rate, penalty, in-group
    cost, weapon picks and specialization read the active entries only. `FiguresOf` then sets the
    first class's initial counts and adds each frozen entry's gained slots at its left-at level, so
    the active entry adds `floor(L / #Levels)` and no second initial count; with two frozen entries,
    only the first class's initial count is kept. A level-up's grants (`GrantsOf`) are the new
    class's gained slots. Tests: `TestProficiencySlots_Step15_DualClassKeepsAndGains` (skills: the
    elf, fighter left at 9, has 7 weapon and 6 non-weapon slots at mage 1, 8 and 9 at mage 10, the
    mage's -5), `TestProficiencyClasses_Step15_TheOldClassKeptAtItsLeftAtLevel` (charactersvc).)_

16. **Equipment limits together** (amends [ADR 0037](0037-proficiencies.md) step 7 and the warnings
    of [ADR 0033](0033-equipment.md) step 20; still warnings, never refusals, ADR 0033 step 20).
    From each class definition's `equipment` (ADR 0040 step 1.6), never a class id:
    1. **Weapons**: allowed when any active entry's class allows it (a warrior uses all his
       abilities, PHB multi/dual, Multi-Class Benefits and Restrictions, Warrior), except that an
       entry whose class is `blunt_only` restricts every weapon to its own list: a multi-class
       priest keeps his weapon limits whatever his other classes (same section, Priest: a
       fighter/cleric uses blunt weapons only, with the warrior's combat value). The weapon
       proficiency types (ADR 0037 step 7) follow the same rule (decided in triage: "a cleric
       multi-class keeps the cleric's weapon limits (warnings)"), **unless the ruleset lifts it**:
       with [ADR 0041](0041-ruleset.md) step 2.5's `multiclass_any_weapon` on, the `blunt_only`
       entry no longer judges alone, and a weapon or a weapon proficiency type is allowed when any
       active entry's class allows it (amended 2026-10-22, user: "our DM allows our priest fighter
       to use a long sword"). Off by default. A single-class character, a dual-class character (its
       one active entry decides, and an old class still adds nothing to an active `blunt_only` one,
       step 13) and the specialization (a multi-class character's stays refused) are unchanged.
    2. **Armour**: wearing it is allowed when any active entry's class allows it; what it blocks is
       each class's own: an entry whose class allows no body armour (`armour: none`) and has a
       `spell_progression` warns `spell_in_armour` on every cast of its spells while body armour is
       worn, except an elf in elven chain (same section, Wizard: elves in elven chain cast in
       armour; elven chain is a catalogue item, the elf a race trait) _(Q15: a warning, as
       the rest of the class limits)_.
    3. **Thief skills**: an entry with `rogue_skills` whose character wears body armour outside
       the kind's `armour` list can use only Open Locks and Detect Noise; every other skill of the
       set warns `skill_blocked_by_armour` and is not rolled (same section, Thief: only those two,
       the gauntlets off for the first and the helmet for the second) _(Q16: they take the
       no-armour percentage)_. A single-class thief or bard is unchanged (its
       armour outside the list is already ADR 0033 step 20's warning).
    Tests: `Weapons_Step16_1_ABluntOnlyEntryRestrictsTheUnion` (itemsvc: a fighter/cleric warned for
    a long sword, not for a mace; a fighter/thief not warned for a long sword),
    `WeaponPick_Step16_1_ABluntOnlyEntryRestrictsThePicks` (charactersvc),
    `Cast_Step16_2_WizardSpellInArmourWarns` (castsvc: a fighter/mage in chain warned; an elf
    fighter/mage in elven chain not), `Skills_Step16_3_ArmourBlocksAllButTwo` (skillsvc).
    _(As built, 2026-10-22, 16.1 and 16.2, on the class definitions' `equipment` (ADR 0040 step 1.6):
    `itemsvc.ClassWarnings` takes the `classes.Equipment` of every active entry (the AC card's
    armour warnings too; an entry whose class the catalogue does not define has no limit, so nothing
    is warned); a piece is warned only when no entry allows it, and a `blunt_only` entry judges every
    weapon alone. Test: `Weapons_Step16_1_ABluntOnlyEntryRestrictsTheUnion`. The weapon picks follow
    the same rule (`classes.AllowedTogether`, which both read): a type is refused only when no active
    entry's class may pick it, or when a `blunt_only` entry's may not; the specialization stays the
    first entry's. Test (skills, with the pick check): `WeaponPick_Step16_1_ABluntOnlyEntryRestrictsThePicks`. The types the picker offers are
    the same rule's answer, not a class's list: `derived.proficiencySlots.weaponTypes`
    (`[{type, specializationSlots}]`, additive; `skills.figuresOf` over `Classes.weaponTypeAllowed`,
    the weapon lift included) on the sheet, the editor's preview and the creation draft, so a
    cleric-first cleric/fighter is offered the long sword under the lift. Test:
    `WeaponTypes_Step16_1_TheOfferedTypesFollowTheAcceptedPicks`. Each derived entry
    carries `NoArmour` (its `armour` list is only `none`; test `Derive_Step16_2_NoArmourComesFromTheEquipment`).
    A cast whose spells are such an entry's (the slot's entry, else the first active spellcasting
    entry, step 14) reads the worn pieces and answers `warnings: ["spell_in_armour"]` (never
    refusing; logged with the cast since the dual-class build, step 13) while body armour other than `none` is worn, except an elf (race `elf`) in the
    `elven_chain` column (still a race id in the code: the race trait is to come). Test:
    `Cast_Step16_2_WizardSpellInArmourWarns`. Not built: ADR 0043 step 18's `power_in_armour`.)_
    _(As built, 2026-10-22, 16.3: "the kind's `armour` list" is read as the skill class's equipment
    `armour` list (ADR 0040 step 1.6; the rogue_skills armour entries name a Table 29 row for every
    column, so they list no armour out). A character of several class entries (a dual-class's frozen
    entry counts) wearing body armour that list does not allow (`skills.PointsSheet.ArmourBlocks`)
    reads Open Locks and Detect Noise (`ruletablesvc.ArmourFreeSkills`) at the no-armour column and
    marks every other skill of the set `Blocked` (the skill's `blocked` on the sheet and on GET
    /api/skills, where it is not `usable`); a roll of a blocked skill is refused before the die
    with the field code `skill_blocked_by_armour`, as the 0% skill's `skill_unusable`. Tests:
    `Skills_Step16_3_ArmourBlocksAllButTwo` (charactersvc, the derivation),
    `Skills_Step16_3_ABlockedSkillIsNotRolled` (skillsvc). The web has no wording for the code
    yet.)_

    _(Amended 2026-10-21 by [ADR 0043](0043-psionics.md) step 18, on implementation: an entry with the `psionics` kind warns `power_in_armour` on every power check while body armour outside its class's `armour` list is worn, as step 16.2 does for spells)_

### Scope

17. **The party is the acceptance fixture.** The implementing items prove the steps on the table's
    four characters as entered from their sheets: the elf fighter to mage (dual-class, past the
    crossover), the half-elf fighter/cleric (multi-class), the human thief/psionicist
    (multi-class, its psionicist half per ADR 0043) and the half-elf bard 15 (single class, every
    existing figure unchanged).
    Test: `Party_Step17_TheFourCharactersDeriveAsTheirSheets` (composition: THAC0, saves, slots,
    proficiency slots and warnings for each; the bard's equal to today's).
    _(As built, 2026-10-22, the two multi-class characters on the fixture's real tables:
    `Party_Step17_TheMultiClassCharacters` (charactersvc: the half-elf fighter 13/cleric 14 and the human
    thief 15/psionicist 15, the psionicist entry its level, THAC0 and saves only); the bard's golden
    files unchanged. The dual-class elf and the warnings come with their items.)_
    _(As built, 2026-10-22, the dual-class elf: the party's real levels are recorded nowhere, so the
    fixture is the ADR's own example, the fighter left at 9, then a mage 10 (past the crossover:
    the fighter's THAC0 and saves where better, both classes' features, none dormant, the mage's
    slots at 10) or a mage 7 (the mage's THAC0 and saves alone, the fighter's features dormant).
    Test: `TestParty_Step17_TheDualClassElf` (charactersvc).)_

## Decided with the user (2026-10-22)

The user answered "1-16 defaults ok" (2026-10-22): each point below is the rule as decided, which
is the step as written. The other readings noted were considered and not taken.

1. **Q1, multi-class and dual-class at once.** The book makes them exclusive (multi-class is
   demihuman, dual-class human); with both lifts, could a fighter/thief take a new class? Decided:
   refused.
2. **Q2, combinations under the lift.** The lift lets any race be multi-class; does it also allow
   any combination of classes (a half-elf fighter/bard, three classes for a gnome, whom the book
   limits to two), or only the book's combinations for some race? Decided: any two or three
   catalogue classes.
3. **Q3, the split's remainder.** The book divides the XP equally and says nothing of the odd
   point. Decided: each award's share rounded down per award, the remainder lost (as the book
   rounds hit points down).
4. **Q4, the bonus for several classes.** The book says "all his prime requisites" without the
   multi-class case. Decided: every prime requisite of every active class at 16 or more (one flag
   for the award); a dual-class's frozen class does not count. Not taken: a bonus per
   class, on that class's share.
5. **Q5, a dual-class past the crossover: THAC0 and saves.** The book says the restriction on the
   old numbers ends, not which numbers apply. Decided: the best of the new class and the old ones
   at their left-at levels. Not taken: the new class's only.
6. **Q6, best saves per category.** The outline's "best saves" and the book's "best saving throw"
   read as category by category (rows may mix). Decided: per category. Not taken: the one
   class whose row is best overall.
7. **Q7, the Constitution share at a level-up.** The book splits the adjustment between the
   classes but gives no rounding, and says the warrior's +3/+4 only for the first level. Decided:
   the share rounded down, and the warrior column for the share of a class that takes it.
8. **Q8, above the name level.** The book divides a hit die by the number of classes but is silent
   on the fixed gain after name level (+3, +2, +1). Decided: divided the same way, at least 1.
9. **Q9, the dual-class XP penalty.** The book's penalty is per encounter and adventure, which the
   app does not track. Decided: a warning on the action and its journal entry; the DM's award is
   entered as given. Not taken: the app halves the next award.
10. **Q10, "best Table 34 line" against the book.** The triage said the best line; the book says the
    largest initial count and the fastest rate, which can come from two rows (and per pool).
    Decided: the book's, per column.
11. **Q11, which level counts for the slot rate.** The book gives the fastest rate, not whose level
    it applies to. Decided: the level of the entry whose rate is used.
12. **Q12, the non-proficiency penalty.** The book does not say which class's. Decided: the
    smallest (a fighter/mage takes -2).
13. **Q13, the non-weapon crossover cost.** Not stated for several classes. Decided: a proficiency
    is in-group when it is in any active class's groups.
14. **Q14, a dual-class's slots.** The book keeps the old proficiencies and is silent on the new
    class's slots. Decided: the old class's slots at its left-at level plus the new class's gains,
    no second initial count.
15. **Q15, wizard spells in armour.** The triage says "not cast in armour"; every other class
    limit is a warning. Decided: a warning on the cast.
16. **Q16, Open Locks and Detect Noise in forbidden armour.** The book allows them (gauntlets or
    helmet off) without saying the percentage. Decided: the no-armour column.

## Consequences

- A character is a list of class entries; every derived figure names the entries it reads.
  Single-class characters are the one-entry case and change in nothing (the bard 15 is the
  regression proof, step 17).
- New warnings: `dual_class_old_ability`, `spell_in_armour`, `skill_blocked_by_armour`, the
  per-entry `level_limit`; a new `level_up` reason, `dual_class_no_hit_points`; new refusals of
  step 2. Each gets its render row and `apiError` / warning key (en, fr).
- The schema change (class entries replacing `class`, `level`, `realized_level`, `xp_total` as the
  one source) and the API change (the level-up and the cast name an entry, the derived block per
  entry) are the implementing items', reviewed as risky. ADR 0040 step 11's order holds: the five
  classes become data first.
- Amended in place, 2026-10-21, on implementation: ADR 0015 steps 1 to 3 (steps 5 and 6),
  ADR 0030 steps 1, 3, 5, 6, 7 and 8 (steps 1, 4, 7 to 11 and 14), ADR 0037 steps 3, 7 and 8
  (steps 15 and 16), ADR 0040 step 5 (step 12).
- Out of scope: the psionicist (ADR 0043), kits, level drain (PHB multi/dual's level-drain
  paragraphs), the DMG's optional rule of exceeding level limits by high prime requisites, the
  DMG's one level per award (ADR 0030 step 7.6), the classes not built (paladin, ranger, druid,
  specialist wizards; the book bars specialists from multi-class but the gnome illusionist).
