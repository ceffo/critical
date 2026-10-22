# ADR 0042: Multi-class and dual-class characters: class entries and how they combine

**Status**: accepted 2026-10-21 for what the user's triage decided (quoted in Context); the steps
or parts marked _(question Qn)_ are readings where the book is silent or differs from the triage's
outline: each carries a stated default that is **not decided** until the user answers the
"Questions for the user" section. Not implemented. Amends [ADR 0030](0030-classes-and-races.md)
steps 1, 3, 5, 6, 7 and 8, [ADR 0015](0015-experience.md) steps 1 to 3,
[ADR 0037](0037-proficiencies.md) steps 3, 7 and 8 and [ADR 0040](0040-classes-are-data.md)
step 5, each in place (dated 2026-10-21) and each on implementation. Builds on the class
definitions and the lookup of ADR 0040 and the racial lift and level-limit mode of
[ADR 0041](0041-ruleset.md). The psionicist's rules as a class of a multi-class character are
ADR 0043's (not written yet).

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
   Multi-Class Combinations), a character both multi-class and dual-class _(question Q1)_.
   Today's columns become the one-entry case: `class`, `COALESCE(realized_level, level)` and
   `xp_total` give one active entry, with no change of behaviour (the implementing item's data
   migration; the storage shape is its).
   Tests: `ClassEntries_Step1_ShapesAndRefusals` (domain: single, multi, dual accepted; each
   refusal above), `Migrate_Step1_ACharacterBecomesOneActiveEntry` (storage, in Go without a
   database: today's class, level and XP give one entry, and every existing step test stays green).

2. **Who may be multi-class or dual-class, and with which classes.**
   1. **Multi-class**: the book allows only demihumans, in the combinations its race lists (PHB
      multi/dual, Multi-Class Combinations; `phb/ch02-player-character-races/`, each race). With
      the ruleset's `multiclass_any_race` true (the default, ADR 0041 step 2.2) any race may be
      multi-class, in any combination of two or three classes the catalogue defines
      _(question Q2)_; with it false, the race's book combinations only (catalogue data, rows keyed
      by race, beside the level limits of ADR 0040 step 2). Each class's ability minimums and
      alignments apply (ADR 0040 step 1.4 and 1.5), and Table 7's dash is waived by the lift
      (ADR 0041 step 5).
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

4. **Entering an existing character.** A character entered from a sheet gives, per entry, its
   class, its level (the realized level) and its XP (`starting_xp`), and for each frozen entry the
   level it was left at; the max HP is entered as the sheet has it, never recomputed (decided in
   triage; ADR 0030 step 7.5). The creation form of ADR 0030 step 1 takes one to three classes;
   for a new multi-class character created in the app, the starting XP is split by step 5 and each
   entry's level comes from its share.
   Tests: `Create_Step4_EntersLevelAndXPPerEntry` (charactersvc: the party's three non-bard
   characters entered as on their sheets, levels and XP kept, max HP kept).

### Experience and levels

5. **The XP award split** (amends [ADR 0015](0015-experience.md) step 1). The ledger stays one list
   per character (an award is the DM's to the character). Each award's amount (base plus its bonus,
   ADR 0015 step 1) goes:
   1. for a multi-class character, **in equal shares to every active entry**, even when an entry
      has reached its race's level limit (decided in triage; PHB multi/dual, first paragraph: the
      experience is divided equally between the classes). The share of an award is
      `floor(amount / n)` for `n` active entries, the remainder lost _(question Q3)_;
   2. for a dual-class character, wholly to the active entry; a frozen entry's XP never changes
      (PHB multi/dual, Dual-Class: no more XP in the previous class).
   An entry's XP is its `starting_xp` plus its shares, derived on read, never stored. The ledger's
   entries, edits and undo are unchanged (ADR 0015 step 4).
   Tests: `EntryXP_Step5_1_EvenSharesEvenAtTheLimit` (xpsvc: a fighter/cleric's 1,001 XP award
   gives 500 each; a share still goes to an entry over its race's limit),
   `EntryXP_Step5_2_DualClassAllToTheActiveEntry` (the frozen entry's XP is unchanged).

6. **The XP bonus** (amends ADR 0015 step 2 as amended by ADR 0040 step 6). The book gives the 10%
   bonus to a character with 16 or more in all his prime requisites
   (`phb/ch03-player-character-classes/02-class-descriptions.md`, Prime Requisites). Eligible when
   every prime requisite of **every active entry's class** is 16 or more _(question Q4)_; a frozen
   entry's class does not count (it earns nothing). The flag stored on each award still counts
   (ADR 0015 step 2).
   Test: `BonusEligible_Step6_EveryActiveClassesPrimeRequisites` (xpsvc: a fighter/mage of Str 16,
   Int 15 not eligible; of Str 16, Int 16 eligible; a dual-class mage of Int 16 whose frozen
   fighter's Str is 15 eligible).

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

### Combat values

8. **THAC0** (amends ADR 0030 step 5). A multi-class character uses the best (lowest) Table 53
   value among its active entries, each at its realized level (decided in triage; PHB multi/dual,
   Multi-Class Benefits and Restrictions: the most favourable combat value). A dual-class
   character uses the active entry's value while its realized level is not above every frozen
   entry's `left_at_level` (the **crossover** of step 13; PHB multi/dual, Dual-Class: the new
   class's combat and saving throw tables, the old numbers penalised). After the crossover, the
   best of the active entry and each frozen entry at its `left_at_level` _(question Q5)_.
   Tests: `THAC0_Step8_BestOfTheActiveEntries` (charactersvc: a fighter 6/cleric 7 uses the
   fighter's row), `THAC0_Step8_DualClassNewClassUntilTheCrossover` (a fighter 9 to mage 9 uses the
   mage's row; at mage 10, the default of Q5).

9. **Saving throws** (amends ADR 0030 step 6). For each of the five categories, the best (lowest)
   target among the active entries at their realized levels (decided in triage; PHB multi/dual,
   Multi-Class Benefits and Restrictions; `dmg/ch09-combat/14-breath-weapons.md`, saving throws:
   the most advantageous roll), category by category, so two classes' rows may mix
   _(question Q6)_. A dual-class character as step 8: the active entry's rows until the crossover,
   then the best per category _(question Q5)_. The race's bonus is unchanged (ADR 0030 step 22).
   Test: `Saves_Step9_BestPerCategory` (charactersvc: a fighter/mage at levels where the fighter is
   better on breath and the mage on spells takes each best).

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
       adjustment, the adjustment divided by the number of classes _(question Q7: the rounding of
       the share, and whether a warrior class's share uses the warrior column)_.
    3. **Above the entry's name level** (ADR 0030 step 7.2): the class's fixed gain, divided by
       the number of classes like a die _(question Q8)_, no Constitution share.
    4. Recorded as ADR 0030 step 7.3's one `level_up` transaction, carrying the entry's class, the
       die, the divisor, the share of the adjustment and the gain.
    Max HP entered for an existing character is kept as entered (step 4).
    Tests: `CreationHP_Step10_1_AverageOfTheDicePlusConstitution` (creationrollsvc: Morrison's
    dice 6, 5 and 2 give 4 before Constitution), `Roll_Step10_2_DieDividedByTheClasses` (levelupsvc:
    a d6 of 4 for a three-class character gives 1; a 1 gives 1, never 0; distinct dice through the
    dice port), `Roll_Step10_3_FixedGainAboveTheNameLevel` (the default of Q8).

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

13. **The dual-class crossover and the old class's abilities.** The **crossover** is reached when
    the active entry's realized level is above every frozen entry's `left_at_level` (PHB
    multi/dual, Dual-Class: higher than his maximum level in any previous class). Before it, the
    book lets the character use the old class's abilities but costs him the XP of that encounter
    and half the XP of the adventure; it also counts the old attack and saving throw numbers and
    the weapons and armour the new class prohibits (same section). The app: an action that uses a
    frozen entry's feature, spell progression or equipment allowance before the crossover is
    allowed and carries the warning `dual_class_old_ability` (with the class), shown on the action
    and in its journal entry; the XP award is entered as the DM gives it, the app computes no
    halving _(question Q9)_. After the crossover nothing is warned, but each action still follows
    the limits of the class it uses (same section: a dual-class fighter/mage casts no spell in
    armour; step 16).
    Tests: `DualClass_Step13_CrossoverAboveEveryLeftAtLevel` (domain: mage 9 after fighter 9 is
    before, mage 10 after), `Cast_Step13_AnOldClassSpellWarnsBeforeTheCrossover` (castsvc).

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

15. **Proficiency slots** (amends [ADR 0037](0037-proficiencies.md) step 3). The book (PHB
    multi/dual, Multi-Class Benefits and Restrictions): the **largest** initial number of slots of
    the classes, then new slots at the **fastest** rate. Read per pool, each column of Table 34 on
    its own, so the initial count and the rate may come from two groups (a fighter/cleric: weapon
    4 initial every 3 levels from the warrior row; non-weapon 4 initial from the priest row, every
    3) _(question Q10: the triage said "best line")_. The levels gained are counted on the level
    of the entry whose rate is used (the highest such level when several share it)
    _(question Q11)_. The weapon picks are one pool for the character (decided in triage).
    The non-proficiency penalty (ADR 0037 step 8) is the smallest among the active entries'
    groups _(question Q12)_. A non-weapon proficiency costs Table 37's slots when one of its
    groups is among any active entry's Table 38 groups (ADR 0037 step 4) _(question Q13)_.
    A dual-class character keeps every pick of the old class (PHB multi/dual, Dual-Class: the old
    class's proficiencies are kept); its slots are the frozen entries' available at their
    `left_at_level` plus the new class's gained slots (`floor(L / #Levels)`, no second initial
    count) _(question Q14)_.
    Tests: `ProficiencySlots_Step15_LargestInitialFastestRate` (ruletablesvc: the fighter/cleric
    above at levels 1, 3, 4 and 12), `ProficiencySlots_Step15_DualClassKeepsAndGains`,
    `NonproficientPenalty_Step15_SmallestOfTheGroups` (charactersvc).

16. **Equipment limits together** (amends [ADR 0037](0037-proficiencies.md) step 7 and the warnings
    of [ADR 0033](0033-equipment.md) step 20; still warnings, never refusals, ADR 0033 step 20).
    From each class definition's `equipment` (ADR 0040 step 1.6), never a class id:
    1. **Weapons**: allowed when any active entry's class allows it (a warrior uses all his
       abilities, PHB multi/dual, Multi-Class Benefits and Restrictions, Warrior), except that an
       entry whose class is `blunt_only` restricts every weapon to its own list: a multi-class
       priest keeps his weapon limits whatever his other classes (same section, Priest: a
       fighter/cleric uses blunt weapons only, with the warrior's combat value). The weapon
       proficiency types (ADR 0037 step 7) follow the same rule (decided in triage: "a cleric
       multi-class keeps the cleric's weapon limits (warnings)").
    2. **Armour**: wearing it is allowed when any active entry's class allows it; what it blocks is
       each class's own: an entry whose class allows no body armour (`armour: none`) and has a
       `spell_progression` warns `spell_in_armour` on every cast of its spells while body armour is
       worn, except an elf in elven chain (same section, Wizard: elves in elven chain cast in
       armour; elven chain is a catalogue item, the elf a race trait) _(question Q15: a warning, as
       the rest of the class limits, or a refusal)_.
    3. **Thief skills**: an entry with `rogue_skills` whose character wears body armour outside
       the kind's `armour` list can use only Open Locks and Detect Noise; every other skill of the
       set warns `skill_blocked_by_armour` and is not rolled (same section, Thief: only those two,
       the gauntlets off for the first and the helmet for the second) _(question Q16: what
       percentage those two take in such armour)_. A single-class thief or bard is unchanged (its
       armour outside the list is already ADR 0033 step 20's warning).
    Tests: `Weapons_Step16_1_ABluntOnlyEntryRestrictsTheUnion` (itemsvc: a fighter/cleric warned for
    a long sword, not for a mace; a fighter/thief not warned for a long sword),
    `WeaponPick_Step16_1_ABluntOnlyEntryRestrictsThePicks` (charactersvc),
    `Cast_Step16_2_WizardSpellInArmourWarns` (castsvc: a fighter/mage in chain warned; an elf
    fighter/mage in elven chain not), `Skills_Step16_3_ArmourBlocksAllButTwo` (skillsvc).

### Scope

17. **The party is the acceptance fixture.** The implementing items prove the steps on the table's
    four characters as entered from their sheets: the elf fighter to mage (dual-class, past the
    crossover), the half-elf fighter/cleric (multi-class), the human thief/psionicist
    (multi-class, its psionicist half per ADR 0043) and the half-elf bard 15 (single class, every
    existing figure unchanged).
    Test: `Party_Step17_TheFourCharactersDeriveAsTheirSheets` (composition: THAC0, saves, slots,
    proficiency slots and warnings for each; the bard's equal to today's).

## Questions for the user

Each question holds its default (the step as written) until answered; the implementing item for a
step waits for its answer or builds the default, as the supervisor rules.

1. **Q1, multi-class and dual-class at once.** The book makes them exclusive (multi-class is
   demihuman, dual-class human); with both lifts, could a fighter/thief take a new class? Default:
   refused.
2. **Q2, combinations under the lift.** The lift lets any race be multi-class; does it also allow
   any combination of classes (a half-elf fighter/bard, three classes for a gnome, whom the book
   limits to two), or only the book's combinations for some race? Default: any two or three
   catalogue classes.
3. **Q3, the split's remainder.** The book divides the XP equally and says nothing of the odd
   point. Default: each award's share rounded down per award, the remainder lost (as the book
   rounds hit points down).
4. **Q4, the bonus for several classes.** The book says "all his prime requisites" without the
   multi-class case. Default: every prime requisite of every active class at 16 or more (one flag
   for the award); a dual-class's frozen class does not count. The other reading: a bonus per
   class, on that class's share.
5. **Q5, a dual-class past the crossover: THAC0 and saves.** The book says the restriction on the
   old numbers ends, not which numbers apply. Default: the best of the new class and the old ones
   at their left-at levels. The other reading: the new class's only.
6. **Q6, best saves per category.** The outline's "best saves" and the book's "best saving throw"
   read as category by category (rows may mix). Default: per category. The other reading: the one
   class whose row is best overall.
7. **Q7, the Constitution share at a level-up.** The book splits the adjustment between the
   classes but gives no rounding, and says the warrior's +3/+4 only for the first level. Default:
   the share rounded down, and the warrior column for the share of a class that takes it.
8. **Q8, above the name level.** The book divides a hit die by the number of classes but is silent
   on the fixed gain after name level (+3, +2, +1). Default: divided the same way, at least 1.
9. **Q9, the dual-class XP penalty.** The book's penalty is per encounter and adventure, which the
   app does not track. Default: a warning on the action and its journal entry; the DM's award is
   entered as given. The other reading: the app halves the next award.
10. **Q10, "best Table 34 line" against the book.** The triage said the best line; the book says the
    largest initial count and the fastest rate, which can come from two rows (and per pool).
    Default: the book's, per column.
11. **Q11, which level counts for the slot rate.** The book gives the fastest rate, not whose level
    it applies to. Default: the level of the entry whose rate is used.
12. **Q12, the non-proficiency penalty.** The book does not say which class's. Default: the
    smallest (a fighter/mage takes -2).
13. **Q13, the non-weapon crossover cost.** Not stated for several classes. Default: a proficiency
    is in-group when it is in any active class's groups.
14. **Q14, a dual-class's slots.** The book keeps the old proficiencies and is silent on the new
    class's slots. Default: the old class's slots at its left-at level plus the new class's gains,
    no second initial count.
15. **Q15, wizard spells in armour.** The triage says "not cast in armour"; every other class
    limit is a warning. Default: a warning on the cast.
16. **Q16, Open Locks and Detect Noise in forbidden armour.** The book allows them (gauntlets or
    helmet off) without saying the percentage. Default: the no-armour column.

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
