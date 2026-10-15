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
   the same data and level row, with the spellbook at the Bard's level. It is the dice of one instance (dice groups in the data's order
   plus a constant), the number of instances and the number of rounds; none for a spell a cast
   rolls no damage for. A damage formula of the data must reduce to dice plus a constant at every
   level (a die that is multiplied, divided, capped or subtracted is a startup error).
   Tests: `TestCast_PerSeedEntry`, `TestReceive`, `TestExpand`, `TestExpandRejectsDiceNotAdded`,
   `TestEntry_DamageAt`, `TestEntry_DamageAt_None`, `TestDamageAt_IsWhatACastRolls`,
   `TestInfo_DamageAtTheCharacterLevel`.
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

## Consequences

- A spell with no catalogued effect still spends its slot: the cast is logged and its text shown.
- Time passing never refills anything ([ADR 0005](0005-rounds-and-time.md) step 7): only the
  long rest does.
- Not covered: memorisation time, preparing from scrolls or other books, and spells per day
  beyond the PHB table.
