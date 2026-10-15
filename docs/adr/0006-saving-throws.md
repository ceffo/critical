# ADR 0006: Saving throws

**Status**: accepted, 2026-10-15 (critical-qki.37; collects the rules shipped in critical-qki.9 and
critical-qki.14, until then in `docs/backend.md`)

## Context

A saving throw is a d20 against a target from the character sheet, and the DM often adds a bonus
the app cannot know in advance ("+4 against fire, Gaia protects you"). The player learns it after
the roll, so the result card must be correctable without rolling again, and the correction must
stay auditable. The Bard's sheet holds one target per category; the buffs may add to the roll.

## Decision

Tests live in `api/internal/domain/service/combatsvc/` unless named otherwise.

1. **The roll.** One d20, plus the save bonus of each active buff (a buff's `save_bonus`, which
   may be negative), against the target of the category on the character sheet: Paralyze/Poison,
   Rod/Staff/Wand, Petrify/Polymorph, Breath Weapon, Spells. Each buff is its own modifier, named
   after the buff. Any other category is refused (400) before anything is rolled.
   Tests: `TestResolveSavingThrow` (one case per category), `TestResolveSavingThrow_OneModifierPerBuff`,
   `TestResolveSavingThrow_Errors` ("unknown category").
2. **Verdict.** The save passes when `d20 + buff bonuses + adjustment >= target`: equal passes.
   The margin is the total minus the target (negative on a failure).
   Tests: `TestResolveSavingThrow` ("para/poison pass" at exactly the target, "para/poison fail" one
   below, "buff bonus turns a fail into a pass", "negative buff bonus"), `TestAdjustSave_Recompute`.
3. **Natural 1 and 20.** Plain arithmetic, as for attacks ([ADR 0002](0002-attack-resolution.md)
   step 3): no automatic pass, no automatic failure.
   Tests: `TestResolveSavingThrow_NaturalOneAndTwentyArePlain`, `TestAdjustSave_Recompute`
   ("natural 1 is plain arithmetic").
4. **Logged, changes no state.** Every save is one `save` entry in the action log with no
   snapshots; its payload holds the category, the d20, the buff modifiers, the target, the
   adjustment and its reason, the total, the verdict, the margin and `rolledPassed` (the verdict
   as rolled, kept through any adjustment).
   Tests: `TestResolveSavingThrow_LogPayload`, `TestResolveSavingThrow` (asserts the kind and no snapshots on every case).
5. **Adjustment.** After the roll the player may set a signed adjustment from -20 to +20 with an
   optional reason (at most 80 characters, trimmed). It replaces any earlier adjustment; 0 removes
   it and drops the reason. The verdict and margin are recomputed from the stored d20, buff
   modifiers and target: **the d20 and the buff modifiers never change**, and the original
   verdict stays in `rolledPassed`. An out-of-range value or a too long reason is refused (400).
   Tests: `TestAdjustSave_Recompute` ("fail to pass", "pass to fail, buff kept", "back to 0
   restores the rolled verdict, reason dropped", "a new adjustment replaces the previous one"),
   `TestAdjustSave_Rejections` ("above +20", "below -20", "reason too long").
6. **Latest roll only.** Only the newest entry among the roll kinds (`actionlog.RollKinds`:
   attack, save, cast, item use, skill check ([ADR 0014](0014-skill-checks.md)): the ones that
   replace the result card) can be adjusted, and that
   entry must be a save. Voided entries count when looking for the newest roll. Another kind is
   refused (400), a voided save or one that is not the latest roll is refused (409).
   Tests: `TestAmendLatest` (actionlogsvc: "not a save", "voided", "a later save exists", "a later
   attack exists", "a later cast exists", "a later item use exists", "a later skill check exists"), `TestAdjustSave_Rejections`
   ("not the latest", "voided", "not a save"), router `TestSpecialSentinels` ("adjust voided", "adjust not latest",
   "adjust not a save").
7. **Audited.** The adjustment is an `amend` correction on the entry carrying the new
   payload (with `previousPassed` and `previousAdjustment`); the entry is never edited, the log
   reads the latest payload (`effectivePayload`).
   Tests: `TestAdjustSave_Recompute` (one amend per adjustment, `rolledPassed` kept),
   `TestAmendLatest` ("latest roll amended, older attack ignored", "amended again"),
   `TestEntryEffectivePayload`.

## Consequences

- The verdict a save shows is always the arithmetic of its stored numbers; there is no manual
  "mark as passed". A DM ruling that changes the outcome is an adjustment with a reason.
- A save changes no state, so popping it from the log ([ADR 0007](0007-hp-ledger.md) step 9)
  restores nothing; it only removes the line and frees the adjustment of the roll before it.
- The save bonus of a buff is a modifier of the roll, not of the target: a buff added after the
  roll does not change a logged save.
- Not covered: the target values themselves (they come from the character sheet seed), saves by
  creatures other than the Bard, and the effect of a failed save (the DM's call).
