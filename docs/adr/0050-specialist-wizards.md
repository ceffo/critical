# ADR 0050: Specialist wizards: the eight schools, school codes, opposition and the entry switch

**Status**: proposed 2026-11-13 (CRI-343, under the epic CRI-336). The user's triage of 2026-11-13 is
quoted in Context and binds; the points it left open are decided below against the existing ADRs and
the code, and the readings the books leave open are in "Questions for the user", each with a
recommended default that the record takes as written unless the user says otherwise. Steps 1 to 4,
the catalogue, are built (CRI-344, format 23 of [ADR 0021](0021-spell-catalogue.md) step 31); steps 5
to 11 and 14, the engine, are built (CRI-345): the eight specialists are creatable. The saves (steps
12 and 13) are built (CRI-346). The entry switch (step 15) is built (CRI-383). It amends [ADR 0040](0040-classes-are-data.md) step 3 (a new
feature kind), [ADR 0021](0021-spell-catalogue.md) (school codes on a spell, a format bump),
[ADR 0008](0008-spell-slots-and-long-rest.md) steps 2 and 3 (the school slot, opposed spells),
[ADR 0006](0006-saving-throws.md) step 10.2 (the school modifiers of a save that names its spell),
[ADR 0030](0030-classes-and-races.md) step 20 (20.1, 20.2, 20.7, 20.8 and 20.9: learning, the
starting book, the free spell) and [ADR 0042](0042-multi-class-and-dual-class.md) step 2.1 (who may
be a multi-class specialist).

## Context

The catalogue has one wizard class, the mage. The table's friend plays a mage 14 that is a
transmuter on his sheet ("spécialisation altération (+1 sort / niveau)"); the app cannot say so, and
ADR 0030 step 20.9 left "the specialist wizards' school spells" out of scope. CRI-185 (the deferred
classes) held the specialists with the paladin, the ranger and the druid; the triage takes the
specialists out of it.

### Decided in triage (user, 2026-11-13)

- **All eight schools**, the full book mechanic now (CRI-185 keeps the paladin, ranger and druid).
- **Table 22's race limits warn**, as the level limits do ([ADR 0041](0041-ruleset.md) step 5); the
  campaign lifted racial limits.
- A specialist is **chosen at creation**, and an **existing mage entry can be switched** to a
  specialist of the same group by its owner, XP and level unchanged, so entered characters are fixed
  (the friend's mage 14 becomes a transmuter).
- The friend's "+1 sort / niveau" is the transmuter; the book's mechanic is broader (below).

### What the books say (restated, cited; no book text)

Files are in `critical-rulebooks`. The PHB's mage and specialist section is
`phb/ch03-player-character-classes/06-mage.md` (PHB p.31-32), Table 22 is
`tables/phb-022-wizard-specialist-requirements.md` (PHB p.31).

- **Schools** (PHB p.31): nine schools, eight greater ones and the minor school of lesser divination,
  which is every divination of spell level 4 or less and is open to every wizard; greater divination
  is a divination of level 5 or more. Lesser divination is no specialty (PHB p.32, Table 22's notes).
- **Who may specialize** (PHB p.31): a single-class wizard; a multi-class character may not, except
  a gnome illusionist; a dual-class human may.
- **Table 22** (PHB p.31) gives per specialist the school, the races, the second minimum ability and
  the opposition schools. Its notes (PHB p.32): every specialist also needs the mage's Intelligence,
  the second ability is an **additional prime requisite** (the wizard's Intelligence is "one of
  several prime requisites", PHB p.30, `05-wizard.md`), and an opposition school's spells can be
  neither learned nor used through magical items that duplicate them.
- **The benefits** (PHB p.32), in the book's order:
  1. one extra spell per spell level, which must be a spell of his school;
  2. +1 to his saves against spells of his school cast by other wizards; anyone else saves at -1
     against his spells of his school; both apply together and cancel;
  3. +15% to the chance to learn a spell of his school, -15% for any other school (Table 4's chance);
  4. at each new spell level, one spell of his school with no learn roll (the DMG's free spell, DMG
     p.41, `dmg/ch07-magic/01-initial-wizard-spells.md` "Going Up in Levels": for a specialist that
     spell is of his school when one is available);
  5. spell research one level easier in his school (DMG): not modelled (the app has no spell
     research).
- **Starting book** (DMG p.40, same file): the DM may let a specialist know one spell of his school
  beside Read Magic and Detect Magic.
- **A spell's school** (PHB p.87, `phb/ch07-magic/05-magical-research.md`, the spell description
  notes): for a wizard spell it decides what a specialist may learn; for a priest spell it is for
  reference only.
- **DMG Table 7** (DMG p.15, `tables/dmg-007-racial-class-and-level-limits.md`) has an Illusionist row
  (human unlimited, gnome 15, dashes elsewhere) and the Mage row (elf 15, half-elf 12), no row for the
  seven other specialists.
- The Complete Wizard's Handbook in the rulebooks repo holds spells only, no specialist rules.

| Specialist (id) | School (code) | Table 22 races | Second ability | Opposition (codes) |
|---|---|---|---|---|
| Abjurer (`abjurer`) | `abjuration` | human | Wis 15 | `alteration`, `illusion` |
| Conjurer (`conjurer`) | `conjuration_summoning` | human, half-elf | Con 15 | `greater_divination`, `invocation_evocation` |
| Diviner (`diviner`) | `greater_divination` (and `lesser_divination`, Q3) | human, half-elf, elf | Wis 16 | `conjuration_summoning` |
| Enchanter (`enchanter`) | `enchantment_charm` | human, half-elf, elf | Cha 16 | `invocation_evocation`, `necromancy` |
| Illusionist (`illusionist`) | `illusion` | human, gnome | Dex 16 | `necromancy`, `invocation_evocation`, `abjuration` |
| Invoker (`invoker`) | `invocation_evocation` | human | Con 16 | `enchantment_charm`, `conjuration_summoning` |
| Necromancer (`necromancer`) | `necromancy` | human | Wis 16 | `illusion`, `enchantment_charm` |
| Transmuter (`transmuter`) | `alteration` | human, half-elf | Dex 15 | `abjuration`, `necromancy` |

### What the code does today (read, 2026-11-13)

- A class is catalogue data with typed features ([ADR 0040](0040-classes-are-data.md)); the mage is
  `group: wizard`, `prime_requisites: [int]`, Int 9, `spell_progression: wizard_spells` and one
  feature, `spellbook {max_spell_level: 9, free_spell_per_new_spell_level: true}`. A class listing an
  unimplemented kind is not creatable (`classes.Class.Implemented`, as for the psionicist's kinds
  before they were built).
- A spell's `school` is raw book text (ADR 0021 step 2's table). The real catalogue (format 22) has
  511 wizard spells and 31 distinct comma-separated school tokens: the PHB's names in several
  spellings (`Conjuration/Summoning`, `Conjuration`, `Summoning`; `Illusion/Phantasm`, `Illusion`,
  `Phantasm`; `Invocation/Evocation`, `Invocation`, `Evocation`; `Enchantment/Charm`,
  `Enchantment`), two slash pairs that are two schools (`Necromancy/Alteration`,
  `Divination/Illusion`), the Tome of Magic's schools (`Dimension`, `Geometry`, `Force`,
  `Alchemy`, `Artifice`, the `Elemental` ones, `Mentalism`, `Shadow`, `Song`) and Cantrip's
  `All Schools`. Two wizard spells have no PHB school: Cantrip (`All Schools`) and Etherealness
  (`Dimension`). 115 wizard spells name two schools or more.
- The racial level limits (`race_level_limits`, DMG Table 7) need a row per restricted race and per
  class (`ruletablesvc.NewLevelLimits`); a dash refuses a single-class character of that race and
  class at creation (`RaceAllows`, ADR 0041 step 5), a level over the limit is the `level_limit`
  warning.
- A multi-class character's class with `multiclass: false` (format 17, the bard) is refused at
  `classes.<i>.class` with `multiclass_class_not_allowed` (ADR 0042 step 2.1).
- The spellbook's limits are one pure check, `spellbook.Check` (ADR 0030 step 20, as built); the
  learn roll is learnsvc's; the free spell is `spellbook.FreeSpellsAt` and `Owed` with learnsvc
  `AddFreeSpell`; the slot allotment is derived in `charactersvc` and reaches `prepsvc` through the
  character view ([ADR 0046](0046-derived-figures.md) step 4.4); the save's terms are
  `saves.Terms` (ADR 0046 step 3.3) and the save already names its spell (ADR 0006 step 10.2).
- The dual-class switch, `POST /api/character/dual-class {class}` (dualclasssvc), is the pattern for
  a change of an entry's class: checks as 409 codes, one `class_entry_switch` entry whose `class_entries`
  snapshot the undo writes back.

## Decision

Book facts are cited by page and file of `critical-rulebooks`; no book text is copied here. **Test
names** are written without their `Test` prefix while the test does not exist (`check-adr-tests`);
the implementing item adds the prefix here in the same PR. Constants that are the same for every
specialist (the +1 slot, the ±1 save, the ±15%) stay in the kind's code (ADR 0040 step 3.3); what
differs between specialists is a parameter. No class, school-specialist or race id appears in the
code (ADR 0040 step 10).

### The data

1. **A spell carries school codes** (amends [ADR 0021](0021-spell-catalogue.md) step 2's spell
   table, the next catalogue format, step 4). Beside the raw `school` text (kept, shown as the book
   words it), a spell carries `schools`: a list of codes from a closed list of nine, the PHB's
   schools: `abjuration`, `alteration`, `conjuration_summoning`, `enchantment_charm`,
   `lesser_divination`, `greater_divination`, `illusion`, `invocation_evocation`, `necromancy`.
   1. The builder maps each comma-separated token of the raw text with a committed table: each
      PHB spelling to its school (`Conjuration` and `Summoning` to `conjuration_summoning`,
      `Phantasm` to `illusion`, `Invocation` and `Evocation` to `invocation_evocation`,
      `Enchantment` to `enchantment_charm`), a slash pair that names two schools to both
      (`Necromancy/Alteration`, `Divination/Illusion`), and `Divination` to `lesser_divination`
      at spell level 4 or less and `greater_divination` at 5 or more (PHB p.31).
   2. A token that is no PHB school (the Tome of Magic's schools) maps to no code; Cantrip's
      `All Schools` maps to no code (Q2). The list may then be empty: such a spell is of no
      specialist's school and in no opposition. A token the table does not know fails the build
      (no school is silently dropped).
   3. Priest spells carry `schools` too (the same mapping); no rule reads them (PHB p.87: a priest
      spell's school is for reference).
   4. The API serves the codes with the spell (lookup, spellbook, preparation); the web words them. Built:
      the import stores them (`spells.schools`, a JSON list, migration 044) and the spell payload carries
      `schools` (`[]` for none), `TestImportCatalogue_Step31_SchoolsAreStoredWithTheSpell`,
      `TestSpells_ReadTheStoredSchoolCodes`, `TestSpellToDTO_ServesTheSchoolCodes`; the web wording is the
      UI item (step 17.5).
   Tests (rulebooks repo): the builder's case for each mapping rule and the unknown-token failure;
   (API) `TestParse_Step1_ASpellReadsItsSchools` (catalogueadpt).
2. **The feature kind `school_specialization`** (amends [ADR 0040](0040-classes-are-data.md) step
   3, a new row of the closed list):

   | Kind | Params | Computed by | Classes |
   |---|---|---|---|
   | `school_specialization` | `schools` (school codes, at least one), `opposition_schools` (school codes), `races` (race codes, Table 22), `multiclass_races` (race codes, optional) | this ADR, steps 5 to 15 | the eight specialists |

   1. Start-up checks (ADR 0040 step 8), each naming the class: every code a school of step 1 or a
      defined race; `schools` and `opposition_schools` disjoint; `lesser_divination` in no
      `opposition_schools` (PHB p.31: open to every wizard); the class also lists `spellbook` (a
      specialist is a wizard who keeps a book).
   2. A class lists the kind at most once (one specialty: PHB p.31, "a single school").
   Tests: `TestFeatures_Step2_SchoolSpecializationParses` (domain/classes: the params, each refusal of
   2.1 and 2.2), `TestFeatures_Step2_TheEightSpecialistsGrantTheKind` (seedcheck on the real artifact:
   the table of Context, row for row), `TestParse_Step2_RefusesAnUnknownSchoolOrKind` (catalogueadpt: the
   schema's refusals) and `TestDefinitions_Step2_ARefusedSpecializationStopsTheImport` (ruletablesadpt: the
   registry's, at start).
3. **The eight class definitions** (catalogue data, the builder's). Each is the mage's definition
   (group `wizard`, equipment, Table 20's experience, hit dice, `spell_progression: wizard_spells`,
   non-weapon groups, alignments, the `spellbook` feature with `max_spell_level: 9` and
   `free_spell_per_new_spell_level: true`) plus:
   1. `prime_requisites`: `int` and Table 22's second ability (PHB p.32, Q1); `ability_minimums`:
      Int 9 and Table 22's (abjurer Wis 15 … transmuter Dex 15, the table of Context);
   2. the feature `school_specialization` with the table of Context's `schools`,
      `opposition_schools` and `races`;
   3. `multiclass: false` (format 17) on the seven specialists who are never part of a
      multi-class character, and on the illusionist the feature's `multiclass_races: [gnome]`
      instead (step 14);
   4. `name`: a language map (en: the table's names; fr: Abjurateur, Conjurateur, Devin,
      Enchanteur, Illusionniste, Invocateur, Nécromancien, Transmutateur: the French PHB's Tableau 22,
      `source/phb-fr/phb-fr_djvu.txt` of the rulebooks clone, which says Invocateur where this record
      first guessed Évocateur).
   5. **Their DMG Table 7 rows** (`race_level_limits`, a row per restricted race): the illusionist
      takes DMG Table 7's Illusionist row (gnome 15, dwarf, elf, half-elf and halfling a dash); the
      seven others take the Mage row (elf 15, half-elf 12, dwarf, gnome and halfling a dash). The
      human has no row (unlimited), as today.
   Test: `TestDefinitions_Step3_TheEightSpecialists` (ruletablesadpt: the fixture's eight parse with
   their params, minimums, prime requisites and rows; the fixture holds all eight because the schema
   pins the 70 level-limit rows and `seeds/proficiencies.json` must name every class).
4. **The catalogue format bump** (a new step of [ADR 0021](0021-spell-catalogue.md), the next
   format: 23 if no other bump lands first). The spell's `schools` (step 1, required, possibly
   empty), the feature kind (step 2) and the eight classes with their rows (step 3). **Rollout, as
   ADR 0021 step 18.6**: the API reads the new format only (the old one is refused at start, naming
   `just catalogue`); the builder publishes it first, then the API change immediately. The API reads
   the kind but does not compute it in that change: `Class.Implemented` is false for the eight, so
   they are not creatable and no character changes, until the engine item (step 17) sets it.
   Tests: `TestParse_Step4_ReadsTheNewFormatAndRefusesTheOld` (catalogueadpt),
   `TestSchema_Step4_SchoolsAndTheSpecializationKind` (cataloguecheck).

### A spell for a specialist

5. **Of his school, opposed, other.** For a character entry whose class has the kind, a **wizard**
   spell (catalogue class `Wizard`) is:
   1. **opposed** when one of its `schools` is in `opposition_schools`;
   2. otherwise **of his school** when one of its `schools` is in `schools`;
   3. otherwise **other** (a spell with no code included).
   Opposed wins over his school (Q4): a transmuter may not learn an `Alteration, Necromancy` spell.
   A priest spell is none of these. One pure function in the domain (`domain/spellbook` or the
   kind's own package), read by every rule below; for a character with several entries, each
   specialist entry judges with its own params (a gnome fighter/illusionist has one).
   Test: `TestSchool_Step5_OpposedWinsThenHisThenOther` (the three cases, a two-school spell each way,
   a spell with no code, a priest spell). Built: `spellbook.School` (domain/spellbook), which reads
   the entry's `spellbook.Specialization` from the derived spellbook block.

### The spellbook (amends [ADR 0030](0030-classes-and-races.md) step 20)

6. **Opposed spells are refused** (amends 20.1, 20.6, 20.7 and 20.8). Learn, Ajouter, the free spell
   and the creation's starting book refuse an opposed spell with the code `spell_school_opposed`
   (400 on the endpoints, a field refusal on `spellbook.<id>` in the draft and the creation), before
   any die. In `spellbook.Check` it comes after `already_in_spellbook` and before the level checks.
   The code gets a row in `internal/render` and an `apiError.spell_school_opposed` key (en, fr).
   Tests: `TestCheck_Step6_OpposedRefusedInOrder` (spellbook), `TestLearn_Step6_OpposedRefusedBeforeAnyDie`
   (learnsvc, the dice mock expects no call: Learn, Ajouter and the free spell),
   `TestDraft_Step6_OpposedStartingSpellRefused` (charactersvc; with the creation, step 17.2, since a
   specialist reaches the draft only once creatable).
   Built (the starting book): the draft's and the creation's check passes the specialist school of
   the spellbook's entry to `spellbook.Check`, so an opposed starting spell is refused at
   `spellbook.<id>` in the check's order (a 3rd-level abjuration at level 1 reads opposed, not above
   the class).
7. **The chance to learn: ±15** (amends 20.2). For a specialist the chance the d100 is compared with
   is Table 4's chance **+15** for a spell of his school and **-15** for any other (PHB p.32). It is
   not capped (a chance over 100 always succeeds; Int 9's 35 becomes 20 for another school). The
   learn answer's breakdown carries the Table 4 chance, the adjustment (`+15`, `-15`, or 0 for a
   mage) and the chance used; the derived `spellbook` block carries the adjustments, so the web
   words the chance before the roll without computing it. A mage is unchanged. Built: the answer's
   `chance` (Table 4), `adjustment` and `chanceUsed`, also in the `spell_learn` journal entry
   (absent from an entry written before them: its chance used is `chance`); the derived block's
   `schoolAdjustment` (+15) and `otherAdjustment` (-15), both 0 for a mage or a bard.
   Tests: `TestLearn_Step7_SchoolSpellPlusFifteen`, `TestLearn_Step7_OtherSpellMinusFifteen`,
   `TestLearn_Step7_AMageIsUnchanged` (learnsvc: distinct dice values, one call each, the roll that
   succeeds only with the bonus and the one that fails only with the penalty),
   `TestGetCharacter_Step7_TheSpellbookBlockCarriesTheAdjustments` (charactersvc).
8. **The free spell is of his school** (amends 20.8). A specialist entry owes the same one free spell
   per new spell level as the mage (DMG p.41; PHB p.32 describes the same grant, not a second one,
   Q5). Claiming it (`AddFreeSpell`) is refused with `free_spell_not_of_school` (400) for a spell
   that is not of his school, **unless** the catalogue holds no wizard spell of his school at that
   spell level that is not already in his book (the DMG's "if there is a spell available"); then
   any spell he may add is accepted (an opposed one stays refused, step 6). The pending level's
   grant says `ofSchool: true` for a specialist, so the web's "free spell" reminder words it. A
   free spell claimed before a switch (step 15) is kept.
   Built: learnsvc's catalogue port lists the wizard spells of a spell level for the look; the
   grant's `ofSchool` is absent for a mage.
   Tests: `TestAddFreeSpell_Step8_ASpecialistsFreeSpellIsOfHisSchool`,
   `TestAddFreeSpell_Step8_AnySpellWhenNoneOfHisSchoolIsLeft` (learnsvc),
   `TestGetCharacter_Step8_TheGrantSaysOfSchool` (charactersvc).
9. **The starting book** (amends 20.7). Opposed spells are refused (step 6); nothing else changes:
   no learn roll, the same limits, Read Magic and Detect Magic pre-ticked as for the mage (the seed
   is keyed by class: the eight specialists get the mage's entry, data). The DMG's "one spell of his
   school" (DMG p.40) is the DM's to give, entered like any starting spell (Q6).
   Built: `seeds/starting_spellbook.json` (and the fixture's) gives each of the eight the mage's
   list; the seed's start-up check also refuses a pre-ticked spell the class opposes (the creation
   would refuse it).
   Tests: `TestOptions_Step9_ASpecialistIsPreTickedLikeAMage` (charactersvc, on the fixture's seed),
   `TestStartingSeed_Step9_TheSpecialistsArePreTickedLikeTheMage` (seedcheck, the real seed),
   `TestParseStartingSeed_Step9_AnOpposedPreTickStopsTheStart` (spellbookadpt).

### Slots (amends [ADR 0008](0008-spell-slots-and-long-rest.md) steps 2 and 3)

10. **The school slot.** For each spell level whose allotment (ADR 0030 step 8, Table 21 at the
    entry's realized level) is 1 or more, a specialist entry has **one more slot** of that level that
    holds only a spell of his school (PHB p.32). A preparation of that level is accepted when its
    spells number at most the allotment + 1 **and** its spells that are not of his school number at
    most the allotment; otherwise it is refused with nothing written. More spells than the allotment
    + 1 keep the existing `prep_over_limit` (409, a mage's code, the limit counted with the school
    slot); a count that fits but holds too few spells of his school is
    `school_slot_needs_school_spell` (400; a row in `internal/render`, `apiError` en and fr). No
    slot is marked: the stored slot list and the cast are unchanged (ADR 0008 steps 1 and 6), so a cast uses any slot of the level. The
    extra slots a spell grants (ADR 0008 step 9) are not counted here and hold any spell. The
    derived slots per level (the view's, ADR 0046 step 4.4) carry the school slot separately
    (`school: 1`), so the web shows "2 + 1" and never adds it itself; so do the preparation
    editor's limits (`GET /api/spells/preparation`, each `limits` row's `school` beside the
    allotment `max`). For a multi-class gnome fighter/illusionist the illusionist entry's progression gets it (ADR 0042 step 14).
    Built: `spellbook.SchoolSlots` (one per spell level with an allotment, `SchoolSlotsPerLevel`),
    derived per entry from the class's `school_specialization` (`EntryDerived.SchoolSlots`; the
    character's `SchoolSlots` follow the same entry as `SpellSlots`) and served as each
    `spellSlots` row's `school` (absent for anyone else), and as prepsvc `LevelLimit.School`, each
    preparation `limits` row's `school` (absent likewise). A level with no allotment has no school
    slot, so it still refuses any spell with `prep_over_limit`. prepsvc reads the spell's school codes
    from the spellbook entry and the specialization from the character's spellbook block.
    Tests: `TestPrepare_Step10_TheSchoolSlotTakesOnlyASchoolSpell` (prepsvc: allotment + 1 with one
    school spell accepted; allotment + 1 with none refused; allotment + 2 refused; a level with no
    allotment has no school slot), `TestDerive_Step10_TheSchoolSlotPerOpenLevel` (charactersvc),
    `TestSchoolSlots_Step10_OnePerSpellLevelWithAnAllotment` (spellbook),
    `TestCharacter_Step10_TheSlotsCarryTheSchoolSlot` (the route's `school`),
    `TestInfo_Step10_TheLimitsCarryTheSchoolSlot` (prepsvc: transmuter and mage limits),
    `TestGetInfo_Step10_ServesTheSchoolSlot` (spellprephdl: `school`, omitted at 0).
11. **Opposed spells are not prepared.** A preparation holding an opposed spell is refused,
    `spell_school_opposed` (step 6's code), nothing written; so is picking one for a blank extra
    slot (ADR 0008 step 9). An opposed spell can only be in the book after a switch (step 15).
    Built: one sentinel, `spellbook.ErrSchoolOpposed`, for learnsvc, prepsvc and castsvc's
    blank extra slot pick (a spell picked there need not be in the book, so castsvc reads its
    schools from the spell catalogue).
    Tests: `TestPrepare_Step11_AnOpposedSpellIsRefused` (prepsvc, the slot list),
    `TestCast_Step11_AnOpposedSpellIsRefusedInAnExtraSlot` (castsvc: the extra slot is picked
    at cast time, so the pick is castsvc's), `TestErrorMapping_Step10_11_SchoolSlotAndOpposed`
    (render: both codes, 400).

### Saves (amends [ADR 0006](0006-saving-throws.md) step 10.2)

12. **+1 against his school.** A save that names a wizard spell (step 10.2's `spellId`) of the
    saver's school gets a named term `specialist` +1 in `saves.Terms` (ADR 0046 step 3.3), beside the
    race and Wisdom terms. A save is always against someone else's spell, which is the book's "cast
    by other wizards" (PHB p.32). A Wisdom immunity (step 10.2) still rolls nothing. A priest spell
    or no spell named gives nothing. With several specialist entries (not possible today, step 14)
    the term counts once.
    Built: `saves.Terms` reads step 5's standing (`spellbook.School`) of the named spell for the
    saver's specialization (`saves.Inputs.Specialization`, from the view's spellbook), so a spell of
    his school that is also of an opposed school gives nothing (Q4); combatsvc reads the spell
    through a `Spells` port (spellbookadpt's catalogue), and an id the catalogue does not hold gives
    nothing, as before. The term has no detail; the web words it (`result.mods.specialist`). The
    save card names a spell without the mind flag through a wand toggle on its label line.
    Test: `TestResolveSavingThrow_Step12_PlusOneAgainstHisSchool` (combatsvc: distinct dice, the term,
    target and verdict; nothing for a priest spell or another school),
    `TestResolveSavingThrow_Step12_AnImmunityStillRollsNothing`.
13. **-1 against a specialist caster.** The save request takes an optional boolean
    `casterSpecialist`: the spell named was cast by a specialist of its school (the player says so;
    the caster is outside the app, or another character the app does not link, until games exist,
    ADR 0041 step 9). With a wizard spell named it adds a term `caster_specialist` -1; without one
    it is refused (400 `caster_specialist_needs_spell`). Both terms on one save cancel, as the book
    says. For a specialist's **own** cast of a spell of his school that allows a save, the cast
    answer carries `targetSaveModifier: -1`, information the web shows for the DM (Q7).
    Built: the request's `casterSpecialist` (`saves.CheckFlags`), refused when the spell named is
    none, a priest spell or not in the catalogue, before anything is read or rolled. The cast's
    modifier is `save.targetSaveModifier` in the cast answer and its logged payload (absent for 0;
    `saves.CasterModifier`: step 5's standing, his school), worded on the cast card and the journal.
    The save card's "Specialist caster" box sits on its spell line, enabled once a spell is named.
    Tests: `TestResolveSavingThrow_Step13_MinusOneAgainstASpecialistCaster` (both terms cancel),
    `TestResolveSavingThrow_Step13_TheFlagNeedsAWizardSpell`, `TestRollSave_Step13_TheCasterSpecialistFlag`
    (combathdl), `TestErrorMapping_Step13_CasterSpecialistNeedsSpell` (render),
    `TestCast_Step13_ASchoolSpellTellsTheTargetsPenalty` (castsvc).

### Who may be a specialist

14. **Creation, races and multi-class** (amends [ADR 0042](0042-multi-class-and-dual-class.md) step
    2.1). A specialist is a class of the creation's picker, from the catalogue (the eight are
    creatable once the engine item sets `Implemented`).
    1. **Table 22's races warn.** A character whose class entry has the kind and whose race is not in
       its `races` gets the warning `specialist_race` (`{class, race}`) on every read, beside
       `level_limit` (triage: never a refusal). It is not a ruleset switch.
    2. **DMG Table 7's dash still refuses** a single-class character, as for any class (ADR 0041
       step 5, step 3.5's rows): a gnome transmuter is refused as a gnome mage is. The racial lift
       waives it for multi-class and dual-class characters, as today (Q8).
    3. **Multi-class.** The seven classes with `multiclass: false` are refused in a multi-class
       character by the existing rule (`multiclass_class_not_allowed`). A class whose
       `school_specialization` has `multiclass_races` is refused in a multi-class character whose race
       is not in it, with the same code at `classes.<i>.class`; the ruleset's racial lift does not
       waive it (the triage: refused except gnome illusionists). The creation options carry each
       class's `multiclassRaces` beside `noMultiClass`, so the form offers the illusionist in a
       gnome's multi-class rows only.
    4. **Dual-class** is unchanged (ADR 0042 step 2.2): a human, or any race with the lift; the 15
       and 17 apply to each prime requisite, the specialist's two included (step 3.1).
    Built: the warning is `{code: specialist_race, field: race, class, race}` (the warning gains
    `race`), for each entry whose class has the kind, frozen ones included, on every read and on the
    draft's sheet. The multi-class refusal reads the kind's `multiclass_races` beside
    `multiclass: false`; a draft whose race is not chosen yet is not judged on it (the class step
    comes before the race step). The options' `multiclassRaces` is a map, class to races (only the
    classes that have the limit); the creation's class picker leaves such a class out of a
    multi-class row once the race chosen is not in it. The eight classes' names are worded by the
    web (`details.classes.<id>`, en and fr).
    Tests: `TestCreate_Step14_TableTwentyTwoRaceWarns` (an elf abjurer created, warned),
    `TestCreate_Step14_TheDashStillRefuses` (a gnome transmuter refused, a gnome illusionist created),
    `TestCreate_Step14_MultiClassSpecialistOnlyAGnomeIllusionist` (gnome fighter/illusionist accepted;
    half-elf fighter/transmuter and elf fighter/illusionist refused, with the lift on and off),
    `TestCreate_Step14_DualClassToASpecialist` (a human fighter to transmuter with Int 17 and Dex 17,
    refused with Dex 16), `TestGetCharacter_Step14_TheRaceWarningOnEveryRead`,
    `TestDerive_Step14_TheGnomeFighterIllusionistsSchoolSlots` (step 10 on the one multi-class
    specialist) (charactersvc), `TestWarningsDTO_Step14_TheSpecialistRaceWarningCarriesClassAndRace`
    (wire).

### The entry switch

15. **Switching an entry to another class of its group** (the triage's fix for entered characters).
    `POST /api/character/class-entry/class {entry, class}` (the entry's index in the character's
    class entries, the target class id), on the current character of its owner (ADR 0029 step 7).
    1. **Allowed** when the entry exists, the target is another class of the **same group**, and
       both classes list `spellbook` (so it is a wizard to a wizard, never a fighter to a mage),
       and at least one of the two lists `school_specialization` (a mage to a specialist, a
       specialist back to a mage, or one specialist to another: Q9). Refusals, 409 each:
       `class_switch_not_allowed` (another group, no spellbook, the same class, no specialist
       side), `class_switch_minimums_not_met` (the target's ability minimums on the sheet's
       scores), `class_switch_alignment_not_allowed`, and step 14.3's
       `multiclass_class_not_allowed` for a multi-class character. A race outside Table 22 warns
       (step 14.1); DMG Table 7's dash does not refuse a switch (the character exists; Q8).
       A frozen entry (the first class of a dual-class character) may be switched like an active
       one: only its class changes, and the new class's opposed spells leave the slot list (15.4);
       an undo gives class and slots back (15.5). `TestSwitch_Step15_AFrozenEntryMayBeSwitched`.
    2. **What changes**: the entry's class only. XP, realized level, `left_at_level`, hit points,
       proficiency picks and skill points are unchanged (the group, so Table 20, Table 21, THAC0 and
       saves, is the same).
    3. **The spellbook**: every entry is kept; an opposed spell is served with `opposed: true` and
       cannot be prepared (step 11). The learn failures and the free spells claimed are kept; what
       is owed is unchanged (the same levels open the same spell levels).
    4. **The preparation**: the switch takes the opposed spells out of the current slot list in the
       same transaction (the other slots keep their used flags, ADR 0008 step 4); the school slot is
       judged at the next preparation.
    5. **One transaction, one journal entry** `class_entry_switch` (payload `{entry, from, to}`)
       with the `class_entries` and slot-list snapshots; undo writes both back, redo the "after"
       ([ADR 0018](0018-transactions-and-undo.md)).
    Built: `classentrysvc` (one transaction: the checks in the order above, `ReplaceClassEntries`,
    the slot list without the prepared opposed spells, one `class_entry_switch` entry with the
    `class_entries` snapshot written by `dualclasssvc.EntriesSnapshot` and the `spell_slots` one by
    `prepsvc.SlotsSnapshot`, so the existing codecs undo and redo it); an entry that does not exist
    or a class the catalogue does not know is a 400 field error (`entry`, `class`); a class another
    entry holds already is `class_switch_not_allowed`. An extra slot a spell granted is kept (it
    holds a spell only once cast). The preparation's spellbook entries carry `opposed: true`
    (`GET /api/spells/preparation`). The creation options gain, additively, each class's `group` and
    `specialistRaces` (Table 22's races by class), so the web offers a class's own group and words the
    race warning before the switch. Web: a one-word "Switch" action in the details popover and a
    dialog (picker, then a confirmation that names what is kept, the opposed spells and the race
    warning).
    Tests: `Switch_Step15_AMageEntryBecomesATransmuter` (XP, level, HP unchanged; one entry),
    `Switch_Step15_Refusals` (one row per code), `Switch_Step15_OpposedSpellsKeptAndUnprepared`
    (the book keeps them flagged, the slot list loses them), `Switch_Step15_UndoRestoresClassAndSlots`
    (composition).

### Scope and order

16. **Not modelled**: spell research one level lower (DMG; the app has no research), magic items
    that duplicate an opposed school's spells (an item names no spell school yet; ADR 0049's
    abilities may later), the Complete Wizard's Handbook's kits and its other specialists
    (wild mages, elementalists), and the Tome of Magic's schools as specialties (their tokens map to
    no code, step 1.2).
17. **The follow-up items, in order** (the scoper files them once this record is accepted):
    1. **Catalogue**: the rulebooks builder's school mapping, the kind, the eight classes and their
       rows, the format bump; then the API reads the format (steps 1 to 4). The eight are not
       creatable yet.
    2. **Engine**: the kind's code, steps 5 to 11 and 14, the new codes' render rows and i18n keys,
       `Implemented` set; the specialists become creatable. It ships as three changes, each leaving
       the app as it was until the last: the spellbook (steps 5, 6 and 7 and 8 on Learn, Ajouter and
       the free spell; built), the slots (steps 10 and 11; built), then the creation (step 6's starting
       book, steps 9 and 14, and `Implemented`; built): a specialist reaches the draft and the creation
       options only once creatable.
    3. **The entry switch** (step 15), with its Settings or class-card action in the web (the user's
       fix for the friend's character comes here).
    4. **Saves** (steps 12 and 13), with the save card's flag; built.
    5. **UI**: the school codes worded on the spell lookup, book and preparation (opposed marked),
       the learn chance's adjustment, the school slot "+1", the free spell "of his school", the
       warnings worded; the creation's class picker already lists the catalogue's classes.

## Questions for the user

Each has a recommended default; the record takes the default as written unless the user says
otherwise.

- **Q1. The second ability as a prime requisite.** PHB p.32 calls Table 22's ability an additional
  prime requisite, so the 10% XP bonus needs 16 in both Intelligence and it, and the dual-class 15/17
  apply to both. **Default: yes** (`prime_requisites: [int, <ability>]`). Alternative: Intelligence
  alone, the second ability a minimum only.
- **Q2. Cantrip, "All Schools".** Every specialist's school and every opposition at once would bar it
  for all. **Default: no code** (open to every wizard, -15 for a specialist, no school slot).
  Alternative: all nine codes with "his school wins" for it alone (a special case: rejected unless
  the user wants it).
- **Q3. The diviner and lesser divination.** Table 22 gives Greater Divination; lesser divination is
  "no specialty" but open to all. Read literally, a diviner has no spell of his school at levels 1 to
  4 (no school slot, no free spell of his school there). **Default: the diviner's `schools` lists
  `greater_divination` and `lesser_divination`** (data, no code), so his benefits start at level 1;
  the conjurer's opposition stays `greater_divination` only.
- **Q4. A spell of two schools.** **Default: opposed wins** (any opposed school bars it; else any of
  his schools makes it his). Alternative: his school wins.
- **Q5. One free spell or two at a new spell level.** The DMG's free spell (all wizards) and the PHB's
  specialist bullet read as one grant that a specialist takes in his school. **Default: one**, of his
  school when one is left (step 8). Alternative: two (the mage's any spell and one of his school).
- **Q6. The starting book's school spell** (DMG p.40). **Default: not pre-ticked or required**; the
  player enters it like any spell the DM gave.
- **Q7. The -1 for others.** The caster is outside the app. **Default: the save's
  `casterSpecialist` flag** for a save rolled in the app, and the cast's `targetSaveModifier` shown
  for a target outside it. Alternative: the cast only (no flag on the save).
- **Q8. DMG Table 7's dash for a specialist.** The triage makes Table 22's race list a warning; the
  dash of DMG Table 7 (a gnome or a dwarf mage) is a different table that refuses a single-class
  creation today. **Default: the dash still refuses at creation** (a gnome may be an illusionist,
  not a transmuter), and **does not refuse the switch** of an existing entry. Alternative: the dash
  only warns for the specialists too (a ruleset change for every class, ADR 0041 step 5).
- **Q9. Which switches.** The triage names mage to specialist. **Default: any wizard-group class with
  a spellbook to another, one side a specialist** (a mage to a specialist, back, or specialist to
  specialist), as sheet corrections. Alternative: mage to specialist only.

## Consequences

- A new class of known kinds stays a catalogue change: the eight specialists add one kind, no class
  id in the code; a ninth specialist (the Tome of Magic's) would be data plus a school code.
- The spell catalogue gains structured schools; the raw text stays for display, so nothing a player
  reads changes until the UI item.
- The slot list keeps its shape: the school slot is a preparation rule, not a stored mark, so casts,
  rests, undo and the snapshots of ADR 0008 are untouched.
- A switch is a correction, not a game action: it is not the dual-class switch and leaves the
  dual-class rules alone. An opposed spell left in a switched book is kept (the book has no
  unlearning, ADR 0030 step 20.3) but is never prepared.
- The race warning is new and is not a ruleset setting; if the user later wants Table 22 enforced,
  it becomes one (ADR 0041 step 2).
- Not covered (step 16): research, opposed-school items, kits and the other supplements' specialists.
