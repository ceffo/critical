# ADR 0008: Spell slots and long rest

**Status**: accepted, 2026-10-15 (critical-qki.37; collects the rules shipped in critical-qki.10,
critical-qki.16, critical-qki.41 and critical-qki.42, until then in `docs/backend.md`)

## Context

The Bard prepares spells from his spellbook each morning, within the number the Player's Handbook
gives his level, and each prepared spell can be cast once before the next long rest. Casting also
applies the spell's effect (a buff, damage dice), and the player makes mistakes, so every change
to the slots must be one undoable log entry. A long rest ends the day: slots and per-day items
come back together or not at all.

## Decision

Tests live in `api/internal/domain/service/` (`prepsvc`, `castsvc`, `spellsvc`, `restsvc`) unless
named otherwise.

1. **Slots are the preparation.** The slot list holds one slot per prepared spell: spell level,
   spell, and a used flag. The same spell may be prepared several times. Slots are sorted by
   spell level and keep the order of the request within a level.
   Tests: `TestPrepare_AcceptsTheLimit_KeepsOrderAndDuplicates`.
2. **Limits.** The number of slots of a spell level may not exceed the PHB Table 32 (Bard spells
   per day) value for the character's level; level 15 is 3 / 3 / 3 / 3 / 2 for levels 1 to 5.
   Exactly the limit is accepted, one more is refused with nothing written, and the same spell
   prepared five times counts five times.
   Tests: `TestBardSpellsPerDay` (adapters/ruletablesadpt: the table against the PHB, levels 1 to
   20, "level 0 and 21 are errors"), `TestInfo`, `TestPrepare_Refused` ("4 level-1 spells at level
   15", "5 copies of one level-1 spell"), `TestPrepare_ThreeLevelOneSpellsAccepted`,
   `TestPrepare_RealStack_LimitsAreEnforcedAtLevel15` (composition).
3. **Only from the spellbook.** A spell that is not in the Bard's spellbook cannot be prepared
   (the whole request is refused, nothing written).
   Tests: `TestPrepare_Refused` ("spell outside the spellbook", "valid and invalid mixed"),
   `TestPrepare_RealStack_LimitsAreEnforcedAtLevel15` ("a refused call changes nothing").
4. **Preparing again keeps the casts.** A new preparation replaces the list, but a spell that was
   already prepared keeps its used flag, one for one (three Mirror Images with one cast used and
   one in the new list as well: still one used). Editing the preparation never refills a slot.
   Tests: `TestPrepare_KeepsUsedFlagOfSpellsStillPrepared`.
5. **A preparation is one logged entry.** The slot list is written and a `spell_prep` entry with
   the slot-list snapshot is recorded in one transaction (if the log fails, the slots are not
   changed), and voiding it brings the previous list back.
   Tests: `TestPrepare_AcceptsTheLimit_KeepsOrderAndDuplicates` (entry, snapshot before and
   after), `TestPrepare_RecordFailureFailsTheCall`, `TestRestore_WritesTheSnapshotBack`,
   `TestPrepare_RealStack_VoidRestoresThePreviousPreparation`.
6. **A cast consumes one slot.** Casting marks the chosen slot used and no other (not another slot
   of the same spell), writes the list once and refuses a used slot (409) or an index outside the
   list (400) with nothing written or logged.
   Tests: `TestCast_ConsumesExactlyTheChosenSlot`, `TestCast_SlotErrors`.
7. **A cast is one entry; void gives the slot back.** The cast, its slot and its buff (when it
   adds one, replacing a same-name buff) are one `cast` entry with the slot-list and buffs
   snapshots, so voiding it (newest first, [ADR 0007](0007-hp-ledger.md) step 8) frees the slot
   and removes the buff. A spell cast on the Bard by someone else is logged too but uses no slot.
   Tests: `TestCast_PerSeedEntry` (the entry and its snapshots), `TestReceive` (no slot call),
   `TestCastIsOneEntryAndVoidRestoresSlotAndBuff`, `TestCastVoidsGoNewestFirst` (composition).
8. **Effects come from the data.** A spell's effect is read from `seeds/spell_effects.json` at the
   caster's level: the Bard's for a cast, the caster's for a spell received (by-group dice use the
   Bard's group, rogue). A spell that is low-confidence, a utility or not in the catalogue is
   shown as text only and changes nothing. The damage a cast rolls is also served unrolled, from
   the same data and level row: with the spellbook at the Bard's level, and in the spell lookup at
   its caster level (1 to 40). It is the dice of one instance (dice groups in the data's order
   plus a constant), the number of instances and the number of rounds; none for a spell a cast
   rolls no damage for. A damage formula of the data must reduce to dice plus a constant at every
   level (a die that is multiplied, divided, capped or subtracted is a startup error).
   Tests: `TestCast_PerSeedEntry`, `TestReceive`, `TestExpand`, `TestExpandRejectsDiceNotAdded`,
   `TestEntry_DamageAt`, `TestEntry_DamageAt_None`, `TestDamageAt_IsWhatACastRolls`,
   `TestInfo_DamageAtTheCharacterLevel`, `TestSearch_ServesDamageAtTheCasterLevel`.
9. **A spell can grant extra slots.** A spell whose effect data has `grant_slots` (Rary's
   Mnemonic Enhancer: 3 levels) adds that many spell levels of extra slots, split by the caster at
   cast time into slots of level 1 or more (3 levels: three level-1 slots, the default, a level-1
   and a level-2, or one level-3); any other split is refused. An extra slot is blank until cast,
   and the spell cast from it is picked then, of the slot's level (none picked, or another level,
   is refused). A new preparation keeps the extra slots; voiding the cast removes them and frees
   the granting spell's slot; a long rest removes them (user ruling 2026-10-15; the spell's
   duration is not tracked). The mechanic is data, never a branch on a spell id.
   Tests: `TestGrantSlots_SlotSplits`, `TestCast_GrantSlots_AddsTheChosenSlots`,
   `TestCast_GrantSlots_RejectsAWrongSplit`, `TestCast_ExtraSlot_PicksTheSpell`,
   `TestPrepare_KeepsExtraSlots`, `TestResetSlots_DropsExtraSlots`,
   `TestGrantedSlots_VoidRemovesExtrasAndRestoresTheBaseSlot`,
   `TestGrantedSlots_PrepareKeepsExtrasLongRestRemovesThem` (composition).
10. **A spell can spend an item and roll a to-hit.** A spell whose effect data has
    `consume_item` spends one inventory item of that type when cast (never a daily item such as
    the Power Gem; the item is named when several are left, refused when none); one with
    `attack_roll` needs a target AC and rolls one d20 against it
    ([ADR 0002](0002-attack-resolution.md), Consequences). The target AC is checked before anything
    is spent. The result shows hit or miss, the item and how many are left. Chromatic Orb is both
    (a `gem`); the app models no gem effect and no damage (user: *"i just need the selected gem
    type to be consumed"*), so its data is a `utility` spell. The item count rides the cast
    entry, so voiding the cast gives the item back. A spell received from someone else spends and
    rolls nothing: these are the caster's. What a cast asks of the player (split, item, target AC)
    is sent with the spellbook, from the same data.
    Tests: `TestCast_ConsumeItemAttackRoll_HitShowsTheItemSpent`,
    `TestCast_ConsumeItemAttackRoll_MissSpendsTheItemNoEffect`, `TestCast_ConsumeItemAttackRoll_Refusals`,
    `TestCast_ConsumeItem_PicksTheChosenItem`, `TestReceive_CastMechanicsAreTheCasters`,
    `TestInfo_CastInputsFromTheCatalog`, `TestConsumedItem_VoidRestoresTheItem` (composition).
11. **Long rest.** One transaction marks every slot unused (the preparation stays) and refills the
    per-day items ([ADR 0016](0016-inventory.md) step 3), and nothing else: stock and charges are
    not refilled. If either part fails nothing changes.
    Tests: `TestResetSlots`, `TestLongRest_ResetsBothInOneLoggedTransaction`,
    `TestLongRest_ItemFailureFailsTheRest`, `TestLongRest_SlotFailureSkipsItems`,
    `TestLongRest_RefillsSlotsAndDailyItemsOnly`, `TestLongRest_FailureInItemsRollsBackSlots`,
    `TestLongRest_FailureInSlotsLeavesGemSpent`, `TestPrepare_RealStack_LongRestKeepsPreparationAndResetsUsed`
    (composition).
12. **A long rest is logged and voidable.** It is one `long_rest` entry that always snapshots the
    slot list (even when no slot was used) and every item whose count changed. Because of that
    snapshot, an older cast cannot be voided past the rest (refused, the rest is on top); voiding
    the rest itself spends the slots and items again.
    Tests: `TestLongRest_ResetsBothInOneLoggedTransaction` (snapshots, payload),
    `TestLongRestBlocksVoidingAnEarlierCast`, `TestLongRest_RefillsSlotsAndDailyItemsOnly` (void),
    `TestGemDispelOncePerDayUntilLongRest` (composition).
13. **Damage parts with their own save.** A spell whose save negates only part of its damage
    (Mordenkainen's Force Missiles: per missile 2d4 that no save stops, plus a burst of 1 per
    level that a save vs. spell negates) writes one instance as `parts` instead of `dice` /
    `by_level` (exactly one of the three): each part has its dice (`dice` or `by_level`, under the
    rules of step 8) and its save effect, its own `save` or else the entry's. A part's effect must
    be `none` or `negates` (a part exists to say exactly what a save does to it; `half` and
    `special` are refused at startup), as are both `dice` and `parts`, an empty list and a part
    whose dice do not reduce to dice plus a constant. The damage served (step 8) is the instance
    as the parts added up (dice groups in the data's order, constants summed), plus each part's
    dice and save effect. A cast rolls each part of each instance exactly once, in the data's
    order; the instance roll adds them up, and the result reports the total and the total on a
    save (the parts a save negates left out, for a target that saves against every instance). The
    cast entry records each part roll; entries logged before parts existed still render. Rolling
    the target's save stays the DM's side ([ADR 0006](0006-saving-throws.md) is the Bard's own
    saves). Spells without parts serve and roll exactly as in step 8. The mechanic is data
    (`seeds/spell_effects.json`), never a branch on a spell id.
    Tests: `TestEntry_DamageAt_Parts` (spelleffectsvc), `TestParse_DamageParts`,
    `TestParse_DamagePartsRefused` (adapters/spelleffectsadpt), `TestDamageAt_ForceMissilesParts`
    (levels 7, 15 and 25), `TestCast_DamageParts_RollsEachPartOnce`,
    `TestReceive_DamageParts_AtTheCastersLevel`, `TestDamageAt_IsWhatACastRolls`,
    `TestDamageToDTO_Parts` (rest/handler/spellhdl), `TestToDTO_DamageParts` (rest/handler/casthdl).
14. **Cast-time modes.** A spell whose damage depends on an option the caster picks when casting
    (Wall of Ice: an ice plane, a hemisphere or a falling ice sheet; Meteor Swarm: four large
    spheres or eight small ones; Ice Storm: hail or sleet) lists `modes` in its data instead of a
    `damage`: each mode has a key (snake_case, unique in the spell) and its own damage, or none. A
    mode's damage has the shape of step 8, may be made of parts (step 13) and uses the entry's
    save. Only a `damage` spell has modes, at least two, at least one of them with damage, and no
    `damage` of its own; any other list is a startup error. The cast request names the mode by key;
    none named is the first mode listed (the default, as for the slot split of step 9); a key the
    spell does not list, or any key for a spell without modes, is refused (400,
    `unknown_spell_mode`) before anything is written, spent, rolled or logged. The cast rolls
    exactly the chosen mode's damage, under steps 8 and 13; a mode without damage rolls nothing and
    the slot is still spent. A spell received from someone else picks its mode the same way and
    rolls it at the caster's level. Served unrolled (step 8): every mode with the damage a cast in
    it rolls, in the spellbook at the Bard's level and in the spell lookup at its caster level; the
    damage served for the spell itself is the default mode's (none when the default rolls none).
    The cast result and its entry record the mode's key (absent for a spell without modes and on
    entries logged before modes existed); the UI words the key. Spells without modes cast exactly
    as before. The mechanic is data (`seeds/spell_effects.json`), never a branch on a spell id.
    Tests: `TestEntry_ModeFor`, `TestEntry_ModesAt` (spelleffectsvc), `TestParse_Modes`,
    `TestParse_ModesRefused` (adapters/spelleffectsadpt), `TestCast_Mode_RollsTheChosenModesDamage`,
    `TestCast_Mode_DefaultIsTheFirst`, `TestCast_Mode_WithoutDamageSpendsTheSlot`,
    `TestCast_Mode_UnknownIsRefused`, `TestReceive_Mode_AtTheCastersLevel`,
    `TestReceive_Mode_RollsAtTheCastersLevel`,
    `TestDamageAt_IsWhatACastRolls`, `TestInfo_ServesTheModes` (prepsvc),
    `TestSearch_ServesModesAtTheCasterLevel` (rest/handler/spellhdl), `TestGetInfo_ServesTheModes`
    (rest/handler/spellprephdl), `TestCastAndReceive_CarryTheMode` (rest/handler/casthdl),
    `TestSpecialSentinels` (rest/router).
15. **Dice steps.** A spell whose dice change from one round, or one instance, to the next
    (Vitriolic Sphere: two dice fewer each round; Chain Lightning: 1d6 fewer each strike;
    Incendiary Cloud: nothing for two rounds, then 1d2, 1d4 and 1d2 per level) writes its damage
    as `steps` instead of `dice` / `by_level` / `parts` (exactly one of the four), and without
    `instances` or `rounds`: the steps are the rounds (`per: round`) or the instances
    (`per: instance`). The steps are either one formula for every step (`dice`, with a `count`,
    the most steps there can be) or one formula per step (`list`). A step formula may use `step`,
    the step's number from 1 (no other formula may); each must reduce to dice plus a constant (step
    8) at every caster level and step, so a decreasing dice count is capped at 0 in the data
    (`max(...,0)`). At a caster level the steps are numbered from 1; a step may have no damage (a
    round of smoke before the cloud flames) and is kept, but the steps after the last one with
    damage are dropped (the dice ran out). The data is refused at startup when it mixes steps with
    another dice form, `instances` or `rounds`, has both or neither of `dice` and `list`, a `dice`
    without `count` or a `list` with one, a `count` that is not a number from 1 to 100 at some
    level, an empty or longer list, or a level at which no step has damage. Served unrolled
    (step 8): the unit and every step's number and dice in order, with no dice of one instance and
    the steps counted as the rounds or the instances. A cast, or a spell received, rolls every
    served step up front (user ruling 2026-10-15: each is shown on its own, as Melf's Acid Arrow
    rolls every round), in order, exactly once, a step without damage rolling no die; Chain
    Lightning rolls the most strikes the caster's level allows (user ruling 2026-10-15), not a
    target count. The result and the cast entry record the unit and each roll's step; entries
    logged before steps existed still render. Stopping the sequence on a save, and the saves, stay
    the DM's side: the app shows each step, the DM drops the ones a save ended. A cast-time mode's
    damage (step 14) may be made of steps. Spells without steps serve and roll exactly as in steps
    8, 13 and 14. The mechanic is data (`seeds/spell_effects.json`), never a branch on a spell id.
    Tests: `TestParseStep` (pkg/formula), `TestEntry_DamageAt_Steps`, `TestEntry_ModesAt_Steps`
    (spelleffectsvc), `TestParse_DamageSteps`, `TestParse_DamageStepsRefused`
    (adapters/spelleffectsadpt), `TestDamageAt_StepsMatchThePHB` (levels 12, 15 and 20),
    `TestCast_DamageSteps_RollsEachStepOnce`, `TestReceive_DamageSteps_AtTheCastersLevel`,
    `TestDamageAt_IsWhatACastRolls`, `TestDamageToDTO_Steps` (rest/handler/spellhdl),
    `TestToDTO_DamageSteps` (rest/handler/casthdl).

16. **Exposure damage.** A spell whose damage is taken each round by whoever is in the area it
    leaves, or passes through it (Cloudkill: 1d10 poison per round above 6 Hit Dice; Wall of Fire:
    2d4 within 10 ft, 1d4 within 20 ft, 2d6 + 1 per level passing through; Death Fog: 1, 2, 4,
    then 8 points per round of exposure), has an `exposure` in its data: the damage of one round of
    exposure, written as one `damage` or as `modes` to pick from at each exposure (Wall of Fire's
    distances), exactly one of the two. The `damage` has the shape of step 8 (parts of step 13
    allowed) or steps per round (step 15), a step being the round of exposure, and never
    `instances` or `rounds` (one roll is one round). The `modes` have the shape of step 14: at
    least two, keys snake_case and unique in the exposure, each with its damage under the same
    rules. Only a `damage` spell has an exposure; it may have no cast damage of its own (a cast
    then rolls nothing and spends the slot). Any other block is a startup error. User ruling
    2026-10-15: exposure is stateless. The app tracks no area, no duration and no count of
    exposures, and neither a cast nor Next Round asks for an exposure: the player rolls each one.
    Served unrolled (step 8) next to the spell's damage: in the spellbook at the Bard's level, in
    the spell lookup at its caster level, and on the cast result at the cast's caster level; with
    modes, every mode and its damage. An exposure roll names the spell, the caster level (1 to
    40), the mode (none: the first; one the exposure does not list, or any for an exposure without
    modes, is refused, `unknown_spell_mode`), the round of exposure (1 to 100, none: 1) and
    whether the Bard is the one exposed. It rolls exactly the dice served for that round, once:
    with steps, the step of that round, or the last step for a later round (Death Fog's 8 points
    from the fourth round on); without steps, the same dice every round. A spell with no exposure,
    a low-confidence one, or one not in the catalogue is refused (400, `no_exposure`), as is a
    caster level or round out of range, with nothing rolled or logged. The roll is one `exposure`
    entry (a roll kind: it replaces the result card), recording the spell, the caster level, the
    mode, the round, whether the Bard was exposed and the roll as a cast records its damage; it and
    the cast entry also record the exposure served at the caster level, so the next exposure rolls
    from a journal entry as well as from a fresh result. User
    ruling 2026-10-15: when the Bard is the one exposed (his own cloud or a spell received), the
    same entry takes the total off his HP through the HP ledger ([ADR 0007](0007-hp-ledger.md),
    the floor of [ADR 0011](0011-hp-below-zero.md) step 1) and the result says what the ledger
    applied (less than the damage when the floor cuts it), so popping it gives the HP back;
    otherwise the roll is the DM's and changes no HP. Spells without an exposure serve and cast
    exactly as before. The mechanic is data (`seeds/spell_effects.json`), never a branch on a
    spell id.
    Tests: `TestEntry_ExposureAt`, `TestEntry_ExposureRound` (spelleffectsvc),
    `TestParse_Exposure`, `TestParse_ExposureRefused` (adapters/spelleffectsadpt),
    `TestExposureAt_MatchesThePHB` (levels 9 and 15), `TestExpose_RollsTheServedDiceOnce`,
    `TestExposureAt_IsWhatAnExposureRolls`, `TestExpose_OnTheBardTakesHP`, `TestExpose_Refused`,
    `TestCast_Exposure_ServedOnTheResult` (castsvc), `TestInfo_ServesTheExposure` (prepsvc),
    `TestSearch_ServesExposureAtTheCasterLevel` (rest/handler/spellhdl),
    `TestExpose_Handler` (rest/handler/casthdl), `TestExposureOnTheBard_VoidGivesTheHPBack`,
    `TestExposureOnTheBard_FloorCutsTheAppliedHP`, `TestExposureRecord_MatchesTheLookupsWireShape`,
    `TestSpecialSentinels` (rest/router).

## Consequences

- A spell with no catalogued effect still spends its slot: the cast is logged and its text shown.
- Time passing never refills anything ([ADR 0005](0005-rounds-and-time.md) step 7): only the
  long rest does.
- Not covered: memorisation time, preparing from scrolls or other books, and spells per day
  beyond the PHB table.
- Damage parts (step 13) cover a save that negates part of the damage. A part halved by a save is
  not modelled (it would need a rounding rule) and is refused; a save whose effect is not damage
  (Mummy Rot's disease) leaves the damage as one part with no save. Force Missiles is not in the
  PHB of the rulebooks repo: its numbers are the ones of its data entry.
- Cast-time modes (step 14) choose the damage only. What a mode does besides (Ice Storm's sleet
  blinds and slows; the ice plane's damage to whoever breaks through it later) is not rolled or
  tracked: the spell's note says it.
- Dice steps (step 15) roll every round or strike there can be. A round the target's save ends,
  and a strike past the last target, are rolled anyway and dropped by the DM. Vitriolic Sphere is
  not in the PHB of the rulebooks repo: its numbers are the ones of its data entry, and its splash
  on secondary targets is not modelled. Incendiary Cloud's save rule (one success ends the saves,
  a failure saves again on the fourth and fifth rounds) is the DM's.
- Exposure damage (step 16) is rolled one round at a time on request: who is in the area, for how
  many rounds, and Cloudkill's slaying by Hit Dice are the DM's (the note says them). Incendiary
  Cloud has no exposure: its dice follow the cloud's round, not the creature's, so they stay the
  steps a cast rolls up front (step 15). Undead and creatures subject to fire taking more from a
  Wall of Fire are the DM's too.
