# ADR 0004: Buffs

**Status**: accepted, 2026-10-15 (critical-qki.36; collects the buff rules shipped in
critical-qki.8, critical-qki.20, critical-qki.29 and critical-qki.33, until then in
`docs/backend.md` "Buff rules"); steps 10 and 11 and the Consequences amended and step 18 added
2026-10-16 by [ADR 0016](0016-inventory.md) steps 9 to 16 (an item's own powers); step 19 (add a
spell as an effect from the lookup) added 2026-10-16. Steps 7 to 10 amended for warriors by [ADR 0030](0030-classes-and-races.md) (18/xx), on its implementation.

_Proposed amendment by [ADR 0033](0033-equipment.md) (equipment, proposed 2026-10-18), in effect once it is accepted: step 11 (an `ac` effect is a change of the AC number, negative betters it; new effect kinds `base_ac` and `initiative`, ADR 0033 step 11) and step 18 (a second condition, `without_magic_armour`, ADR 0033 step 12)._

## Context

A buff is an active effect on the character: a spell received or cast (Strength, Stoneskin, Mirror
Image), or something the player enters by hand. An item's own powers (the Power Gem's) are not buffs:
they join the same aggregate ([ADR 0016](0016-inventory.md) steps 9 to 16). Its lifetime, its effect on
Strength and the way several buffs combine feed the attack ([ADR 0002](0002-attack-resolution.md)),
the damage ([ADR 0003](0003-damage.md)) and the saves. Strength magic needs particular care:
the PHB lets Strength stack with itself but caps it at 18, and Improved Strength goes higher.
User rulings, 2026-10-13: "strength level 2 says WItH OTHER. it's find with ITSELF"; "improved
replaces. level 2 can stack with itself. this is why kelsen often equips 2 str lvl 2 to be sure
to get to 18"; "simple: all str spells stack up to 18. unless it's improved strength that can go
to 23."

## Decision

Tests live in `api/internal/domain/service/buffsvc/` unless named otherwise. The round and time
side (what Next Round, End combat and passing time do) is [ADR 0005](0005-rounds-and-time.md).

1. **Counters.** A buff has two optional counters: `Duration` (rounds left, "timed" while > 0)
   and `Charges` (hits it can still absorb, "charged" while > 0). It may have both (Mirror Image),
   or neither: it then lasts until dismissed. A negative counter is refused.
   Tests: `TestActivate` ("until dismissed", "rejects negative rounds/charges"),
   `TestAdvanceRound` (charge-only and untimed buffs are not counted down).
2. **Duration.** Each Next Round takes 1 off the `Duration` of every timed buff, and a buff that
   reaches 0 expires (a 1-round buff is gone after the next Next Round, a 2-round buff has 1 left).
   Tests: `TestAdvanceRound` ("counts timed buffs down, expires at 0": Duration 2 keeps 1,
   Duration 1 expires).
3. **Charges.** Absorbing a hit spends one charge and nothing else (`Duration` is untouched); the
   buff ends when its charges reach 0. A buff without charges cannot absorb (refused).
   Tests: `TestTrigger` ("a Stoneskin layer absorbs a hit", "the last layer ends the buff",
   "Mirror Image loses an image, keeps its rounds", "a buff without charges cannot absorb").
4. **Both counters.** Whichever runs out first ends the buff: Mirror Image with images left
   expires with its rounds, and its last image ends it though rounds are left. Next Round never
   spends charges, an absorb never spends rounds.
   Tests: `TestAdvanceRound` ("Mirror Image with images left still expires with its rounds"),
   `TestTrigger` ("the last image ends Mirror Image though rounds are left", "Mirror Image loses
   an image, keeps its rounds"), `TestPassTime` ("a timed buff with charges (Mirror Image) expires
   with its rounds").
5. **Editing a counter.** The player may correct the rounds and the charges of an active buff
   (and its note). Setting a counter the buff uses to 0 ends the buff; setting a counter on a
   buff that did not use it makes it use it. Negative values and an edit that changes nothing are
   refused.
   Tests: `TestEdit` ("rounds set to 0 ends a timed buff", "charges set to 0 ends a charged buff",
   "rounds on an untimed buff make it timed", the refusals).
6. **Same name.** Casting or activating a buff whose name an active buff already has (compared
   without case) renews it: the old one is replaced, spell effects are not cumulative with
   themselves. The exception is a Strength bonus that is not Improved (step 7), which is added
   as a second buff. Improved Strength is renewed.
   Tests: `TestAdd` ("replaces same name", "a second Strength is a second buff", "a hand-made
   uncapped Strength stacks too", "Improved Strength is renewed").
7. **Strength bonuses stack, up to 18.** A buff with a `str_bonus` and no `str_max` above 18
   is a Strength bonus (cast, received or entered by hand). With no Improved Strength active, the
   effective STR is the sheet's STR plus the **sum** of every Strength bonus, capped at 18 (PHB:
   Strength "cannot confer a Strength of 19 or more"). A buff's own `str_max` of 18 or less is
   not used. The sheet's STR above 18 is kept (step 9).
   Tests: `TestStrength_Step7_StrengthStacksUpTo18` (several casts, the 18 cap, a hand-made uncapped bonus
   stacking with a received one); the effective STR is computed from the active list, so a dismissed,
   expired or voided Strength drops out of it (`TestDismiss`, `TestAdvanceRound`,
   `TestRestorers_WriteTheSnapshotBack`).
8. **Improved Strength replaces them.** A `str_bonus` whose `str_max` is above 18 is Improved
   Strength (seeded 1d6+4, cap 23, for a bard). While one is active, every Strength bonus
   contributes nothing (the buff list shows it as not applying) and the best single Improved wins:
   the sheet's STR plus its bonus, capped at its `str_max`, whether that is more or less than
   what the Strength bonuses would have given.
   Tests: `TestStrength_Step8_ImprovedReplacesStrength`, `TestListStatus` ("Improved active: the
   Strength bonuses show as not applying"), `TestStrength_Step10_PowerGemFrom18Up` (a low Improved
   turns the gem off).
9. **Never gated, never lowered.** A Strength effect counts whatever its `min_str` says, and no
   rule brings the effective STR below the sheet's, even above 18. There is no "set STR to X"
   effect (user, 2026-10-13: "remove that buff type"): a `str_set` key in a stored buff is ignored
   when read, and in a request body it is ignored like any unknown key.
   Tests: `TestStrength_Step9_StrengthEffectsAreNotGated`, `TestStrength_Step7_StrengthStacksUpTo18`
   ("never lowers a score above 18"), `TestGetEffectiveStr`; `TestActiveBuffs_IgnoresRemovedStrSet` (the ignored
   stored key, `api/internal/adapters/buffdal/`).
10. **Gate.** A buff's other effects apply while the effective STR is at least its `min_str`
    (no `min_str`: always): with `min_str` 18, at effective STR 18 **or more** (19 to 23 with
    Improved Strength included), off at 17. User, 2026-10-13: "not exactly 18 >= 18". A gated buff
    stays on the list and shows as not applying ("needs STR N"). The Power Gem is no longer a gated
    buff: its thunder hammer is an item power whose `min_str` is a trigger (ADR 0016 steps 10 and
    16); the gate stays for hand-made buffs.
    Tests: `TestStrength_Step10_PowerGemFrom18Up`, `TestAggregate` ("MinStr gates the buff when
    STR is too low", "MinStr met exactly applies", "to-hit and damage are gated…", the Power Gem
    cases), `TestListStatus`, `TestStrength_Step8_ImprovedReplacesStrength`.
11. **Combining effects.** Of the buffs that apply: `spells_buff` and `multiplier` factors
    multiply (1 without any); `to_hit`, `damage`, `save_bonus` and `ac` add; a `weapon_buff`
    ("the weapon becomes +N") does not add up, the highest N wins. Each contribution is listed
    with its buff's name for the breakdowns. The carried items' powers that are on join the same
    totals by the same rules, each contribution named by its item (ADR 0016 steps 11 and 14). How
    the attack and the damage use them is ADR 0002 and ADR 0003.
    Tests: `TestAggregate` ("no buffs gives neutral modifiers", "multipliers stack
    multiplicatively, flat bonuses additively, weapon +N: best wins").
12. **Saving throws.** A buff's `save_bonus` is one modifier per buff, labelled with the buff's
    name (so two buffs are two lines, a curse is a negative line), subject to the gate of step 10.
    The save itself is resolved elsewhere.
    Tests: `TestAggregate` (`SaveBonusFrom`: one entry per buff), `combatsvc`
    `TestResolveSavingThrow_OneModifierPerBuff`.
13. **Every change is logged and voidable.** Activating, editing, absorbing and dismissing a buff
    are each one action-log entry with a snapshot of the buff list before and after; casting or
    receiving a spell records its own entry with the same snapshot (castsvc). Voiding the entry
    restores the list. The entry is written in the same transaction as the change. Next Round,
    End combat, Set round and passing time are in ADR 0005.
    Tests: `TestActivate` ("logs the activation with before/after snapshots, then saves"),
    `TestEdit`, `TestTrigger`, `TestDismiss`, `TestRestorers_CoverEverySnapshotKey`,
    `TestRestorers_WriteTheSnapshotBack`.
14. **Saved templates.** A saved buff template (name, rounds, charges, effects, note) is a
    shortcut for filling the buff form; it is not a buff. Saving (a same name replaces) and
    deleting a template are logged and voidable, and they never read or change the active buffs;
    activating from a template is a normal activation (step 13). The snapshot carries each
    template's creation time (`createdAt`) and a restore (undo or redo) writes it back unchanged;
    a snapshot from before the field has none, and the restored row is stamped then.
    Tests: `TestSaveBuff`, `TestDeleteSaved` ("removes and logs, active buffs not read": the
    mock fails on any read of the active buffs), `TestSavedRestorer_WritesTheSnapshotBack`,
    `TestSavedRestorer_KeepsCreatedAtAndReadsOldSnapshots`,
    `TestSavedBuffs_ReplaceStampsARowRestoredWithoutCreatedAt`, `TestRedo_ReplaysTheNewestPoppedTransaction`
    (`actionlogsvc`: the "after" snapshots are written back through the restorers).
15. **Start round.** A buff carries the round it was put on the list in (`StartRound`): the round
    counter when it is activated, cast or received, stamped by the server (a value sent by the
    client is ignored). A renewed buff (step 6) starts again. Nothing else changes it: editing,
    absorbing, Next Round, passing time, End combat and Set round leave it as it is, so after End
    combat or passing time it can be above the counter. Voiding an entry restores it with the list.
    A buff stored before the field existed, or an entry recorded before it, reads as round 0. It
    has no rules meaning: the turn header draws a buff's elapsed rounds from it; what is left is
    `Duration` (step 2).
    Tests: `TestBuffs_Step15_StartRound` (activate and add stamp the round and ignore the caller's,
    renewal starts again, edit / absorb / Next Round / passing time keep it, the void restores it,
    an old entry restores 0); `TestActiveBuffs_StartRoundRoundTripsAndOldRowsReadZero`
    (`api/internal/adapters/buffdal/`).
16. **Starting charges.** A buff carries the charge count it started with (`ChargesTotal`, served
    as `chargesTotal`): the server stamps it from the buff's charges when it is activated, cast,
    received or added from a saved template (a value sent by the client is ignored), so the
    timeline shows "N / M". Absorbing and the other steps leave it as it is. Editing the charges
    keeps it, except that raising the charges above a known total raises the total with them (used
    never goes negative). Voiding restores it with the list. A buff stored before the field
    existed, or an entry recorded before it, reads as 0: the total is unknown and the timeline
    shows just "N charges". It has no rules meaning. No SQL migration: the active buffs are a JSON
    list (step 15 did the same).
    Tests: `TestBuffs_Step16_ChargesTotal` (activate and add stamp it and ignore the caller's,
    absorb keeps it, edit keeps it and raises it with the charges, the void restores it, an old
    entry restores 0, no charges means 0); `TestActiveBuffs_ChargesTotalRoundTripsAndOldRowsReadZero`
    (`api/internal/adapters/buffdal/`).
17. **Spell id.** A buff from a spell carries the spell's catalogue id (`SpellID`, served as
    `spellId`, stored and snapshotted as `spell_id`; [ADR 0021](0021-spell-catalogue.md) step 2:
    state refers to a spell by id). Only the spell cast (cast or received, castsvc) stamps it, from
    the spell's effect entry; a buff activated by hand or from a saved template has none (a value
    sent by the client is ignored). A renewal (step 6) takes the new buff's. Editing, absorbing, Next
    Round and passing time keep it; voiding restores it with the list. It has no rules meaning: the
    client names the buff from the catalogue in the player's language (the French name in the
    French UI; text in a language is a localized-text row, [ADR 0023](0023-localized-text.md)), and shows the stored `Name` when there is no id or the catalogue does not know it.
    The buff entries of the log carry it too: `spellId` next to `buff` (activate never has one,
    edit, absorb, dismiss), and `expiredSpellIds` parallel to `expired` (Next Round, passing time;
    `""` for a buff not from a spell, omitted when no expired buff has one). An entry recorded
    before keeps its stored name; payloads are never rewritten.
    **Backfill** (migration 028): a stored buff without an id, in `active_buffs` and in the buff
    snapshots of `action_snapshots`, takes the `key` of the one catalogue spell whose English
    `name` equals its name exactly (case included). A name no keyed spell has (a hand-made buff, a
    Power Gem form), one that several spells share (a Wizard and a Priest spell of the same name),
    a different case, a buff that already has an id and a list holding anything but buff objects
    are left alone. A database whose spells are not keyed yet when 028 runs (its catalogue never
    imported) matches nothing: its buffs keep showing their stored names, as do the buffs of a
    database migrated with no catalogue.
    The catalogue importer counts these ids as references (ADR 0021 step 5.4.3).
    Tests: `TestBuffs_Step17_SpellID` (add keeps it and a renewal takes the new one, activate drops
    a caller's, edit / absorb / Next Round keep it, the payloads, the void restores it, an old
    entry restores none); `TestCast_PerSeedEntry`, `TestReceive` (castsvc: the buff a cast or a received spell puts on carries the spell's id);
    `TestActiveBuffs_SpellIDRoundTripsAndOldRowsReadEmpty` (`buffdal`);
    `TestBuffs_SpellIDOnTheWire` (`router`); SQLite migration 028 backfilled the spell ids of
    existing buffs (a past migration; the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3); the client's
    `buffName` and log tests (`web/src/i18n/spellName.test.ts`, `web/src/lib/logSummary.test.ts`).
18. **Weapon-kind condition.** A buff's effects, or an item power's (ADR 0016 step 12), may carry
    a `weapon_kind` condition, `melee` or `missile` (the weapon kinds of ADR 0016 step 8). With one,
    the effects count only for an attack, or its preview, made with a weapon of that kind; a hurled
    weapon is `missile`; a save or a spell's to-hit has no weapon, so they never count there.
    Without one they count for every roll, as before. The aggregate is asked for the weapon kind
    of the roll it serves (none for a save or a spell's to-hit). It is generic: no buff or item is
    special-cased. It does not change whether a buff applies on the list (step 10) nor whether a
    power is on (ADR 0016 step 10), and it never conditions a Strength effect (`str_bonus`,
    `str_max`) or `min_str`: those make the one effective STR. A buff takes it in the API and in
    saved templates; the buff form has no field for it yet. Tests (`buffsvc`):
    `TestBuffs_Step18_WeaponKindFiltersTheEffects` (melee-only effects count for a melee attack,
    not for a missile one, a save or a spell to-hit; without the condition: everywhere),
    `TestBuffs_Step18_RejectsAnUnknownWeaponKind`.

19. **Add as effect.** From the spell lookup's cast box ([ADR 0022](0022-activity-focus-layout.md)
    step 2.5), a spell that adds a buff can be put on the Bard as an effect at **his** level, with no
    slot spent: `POST /api/spells/effect {"spellId"}`; the client sends no level. A spell adds a buff
    when its effect entry is a `buff`, not low confidence, and sets no ability (an ability set is
    applied by hand); the same rule decides whether a cast or a received spell puts a buff on, so the
    offer and the action cannot drift. The lookup's spells say so (`effect: true`, absent otherwise);
    the rule is generic, hostile buffs (Slow, Malison) and self-only spells (Shield, Mirror Image)
    included. The buff is rolled as a cast rolls it (rounds, charges, ability gain at the Bard's
    level); nothing else is: no damage, no to-hit, no item spent, no slots granted. It always lands
    on the Bard. Any other spell (damage, utility, low confidence, ability set, no effect entry) is
    refused with `spell_not_an_effect` before anything is rolled or written; an unknown spell is
    `not_found`. Like a receive, it is one cast entry (step 13), its payload's `action` being
    `"effect"`, with the buff list's snapshots, so voiding it removes the buff; renewal (step 6),
    start round (15), starting charges (16) and spell id (17) apply as for a cast.
    Tests: `castsvc` `TestBuffs_Step19_AddAsEffect` (the buffs at level 15, the exact dice, self-only
    and hostile accepted, one entry with only the buff snapshot), `TestBuffs_Step19_AddAsEffectLogPayload`,
    `TestBuffs_Step19_AddAsEffectRefusals` (nothing rolled, written or logged, not offered),
    `TestBuffs_Step19_FlagMatchesTheAction` (every seed entry: offered exactly when a buff goes on);
    `spelleffectsvc` `TestEntry_AddsBuff`; `TestSearch_EffectFlag` (`spellhdl`), `TestAddEffect_Handler`
    (`casthdl`).

## Consequences

- A second Strength cast adds its bonus rather than renewing the first, so a player who casts it
  twice at the same bonus gets both; the cap at 18 is what stops it (step 7).
- Strength bonuses that Improved Strength replaces are not removed: if the Improved Strength
  expires or is dismissed, they count again.
- Exceptional Strength (18/xx) is not modelled: PHB, warriors only; Kelsen is a Bard.
- The Power Gem is an item, not a buff: its daily Dispel Magic and its thunder hammer (`min_str`
  18 trigger, melee only, +5 and ×2) are [ADR 0016](0016-inventory.md) steps 9 to 16. Its powers
  join the aggregate of step 11; they are never on the buff list.
- Not covered: how long a received spell lasts (the caster's level and the spell table give the
  rounds at cast time), resistances, dispelling.
