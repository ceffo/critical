# ADR 0004: Buffs

**Status**: accepted, 2026-10-15 (critical-qki.36; collects the buff rules shipped in
critical-qki.8, critical-qki.20, critical-qki.29 and critical-qki.33, until then in
`docs/backend.md` "Buff rules")

## Context

A buff is an active effect on the character: a spell received or cast (Strength, Stoneskin, Mirror
Image), a Power Gem form, or something the player enters by hand. Its lifetime, its effect on
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
   Tests: `TestStrength_Step7_StrengthStacksUpTo18`; on the real stack
   `TestEffectiveStr_StrengthStacksThenImprovedReplacesIt`, `TestEffectiveStr_VoidSecondStrength`,
   `TestEffectiveStr_DismissOneStrength`, `TestEffectiveStr_StrengthsExpireIndependently`,
   `TestEffectiveStr_HandMadeUncappedStacksWithReceived` (`api/internal/composition/`).
8. **Improved Strength replaces them.** A `str_bonus` whose `str_max` is above 18 is Improved
   Strength (seeded 1d6+4, cap 23, for a bard). While one is active, every Strength bonus
   contributes nothing (the buff list shows it as not applying) and the best single Improved wins:
   the sheet's STR plus its bonus, capped at its `str_max`, whether that is more or less than
   what the Strength bonuses would have given.
   Tests: `TestStrength_Step8_ImprovedReplacesStrength`, `TestListStatus` ("Improved active: the
   Strength bonuses show as not applying"), `TestEffectiveStr_LowImprovedTurnsTheGemOff`,
   `TestEffectiveStr_ImprovedExpiresStrengthsCountAgain` (`composition`).
9. **Never gated, never lowered.** A Strength effect counts whatever its `min_str` says, and no
   rule brings the effective STR below the sheet's, even above 18. There is no "set STR to X"
   effect (user, 2026-10-13: "remove that buff type"): a `str_set` key in a stored buff is ignored
   when read, and in a request body it is ignored like any unknown key.
   Tests: `TestStrength_Step9_StrengthEffectsAreNotGated`, `TestStrength_Step7_StrengthStacksUpTo18`
   ("never lowers a score above 18"), `TestGetEffectiveStr`; `TestActiveBuffs_IgnoresRemovedStrSet` (the ignored
   stored key, `api/internal/adapters/buffdal/`).
10. **Gate.** A buff's other effects apply while the effective STR is at least its `min_str`
    (no `min_str`: always). The Power Gem forms set `min_str` 18: +5 and ×2 at effective STR 18 **or
    more** (19 to 23 with Improved Strength included), off at 17. User, 2026-10-13: "not exactly
    18 >= 18". A gated buff stays on the list and shows as not applying.
    Tests: `TestStrength_Step10_PowerGemFrom18Up`, `TestAggregate` ("MinStr gates the buff when
    STR is too low", "MinStr met exactly applies", "to-hit and damage are gated…", the Power Gem
    cases), `TestListStatus`, `TestEffectiveStr_StrengthStacksThenImprovedReplacesIt`.
11. **Combining effects.** Of the buffs that apply: `spells_buff` and `multiplier` factors
    multiply (1 without any); `to_hit`, `damage`, `save_bonus` and `ac` add; a `weapon_buff`
    ("the weapon becomes +N") does not add up, the highest N wins. Each contribution is listed
    with its buff's name for the breakdowns. How the attack and the damage use them is ADR 0002
    and ADR 0003.
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
    activating from a template is a normal activation (step 13).
    Tests: `TestSaveBuff`, `TestDeleteSaved` ("removes and logs, active buffs not read": the
    mock fails on any read of the active buffs), `TestSavedRestorer_WritesTheSnapshotBack`.

## Consequences

- A second Strength cast adds its bonus rather than renewing the first, so a player who casts it
  twice at the same bonus gets both; the cap at 18 is what stops it (step 7).
- Strength bonuses that Improved Strength replaces are not removed: if the Improved Strength
  expires or is dismissed, they count again.
- Exceptional Strength (18/xx) is not modelled: PHB, warriors only; Kelsen is a Bard.
- The Power Gem forms are buffs like the others (a `min_str` gate, `weapon_buff` and `multiplier`
  effects); the daily gem item that activates them is inventory, not covered here.
- Not covered: how long a received spell lasts (the caster's level and the spell table give the
  rounds at cast time), resistances, dispelling.
