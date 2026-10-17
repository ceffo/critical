# ADR 0028: Ability checks

**Status**: proposed, 2026-10-17

## Context

At the table the DM sometimes asks for a plain ability check, with no proficiency behind it:
"throw a DEX check at -2". Triage (2026-10-17, user) made it an action on the stats themselves:
clicking a stat on the character card rolls the check, and the DM's value ("at -2") is set **on
the result card only**, after the roll, so the character card stays uncluttered. There is no
adjustment before the roll in the UI.

The book has no section of its own on the plain ability check. The PHB (p.102) and the DMG (p.66)
use one "in lieu of a saving throw" when a character avoids danger with an ability (a Dexterity
check to dodge a falling block) without restating how it is rolled. The roll they mean is the
d20-under-an-ability-score roll of the nonweapon proficiency check (PHB, Chapter 5): one d20,
success at or under the score, a positive DM modifier added to the score and a negative one
subtracted from it, and a roll of 20 always fails. The DMG p.101 index entry is about encounter
checks and does not apply. This ADR therefore takes the proficiency check's mechanic
([ADR 0014](0014-skill-checks.md) steps 1 and 2) with no proficiency modifier, and its adjustment
after the roll (ADR 0014 steps 6 to 8, itself the logic of [ADR 0006](0006-saving-throws.md) steps
5 to 7): latest roll only, optional reason, an amend correction, undoable, the verdict as rolled
kept, and **the target moves, the roll never changes**.

Where the project differs from the book:

- The book's DM modifier is decided before the roll. Here it is set after the roll (the player
  learns it once the dice are down), which gives the same verdict for the same numbers.
- The book's `+`/`-` modifier changes the score; so does the app's adjustment (+ easier,
  - harder, the sign of every adjustment in the app). Nothing differs, it is noted because a save's
  adjustment ([ADR 0006](0006-saving-throws.md) step 5) moves its target the other way (`target -
  adjustment`), since a save succeeds at or *above* its target.
- The target is the ability score **in effect** (with the effects the app applies), where a
  proficiency check uses the sheet's score ([ADR 0014](0014-skill-checks.md) Consequences). Triage
  asked for the score in effect: the stat the player clicks is the one the card shows.

## Decision

Tests live in `api/internal/domain/service/skillsvc/ability_test.go` unless named otherwise (the
ability check shares the proficiency check's d20 mechanic and adjustment, so it lives beside it).
The tests this ADR plans are written without their `Test` prefix until they exist, because
`just check-adr-tests` refuses a cited `Test…` name that is not in the code: step 1's
`AbilityCheck_Step1_OneD20PerCheck` gains the prefix in the implementation, and
the change that adds a test writes its full name here. Names with the prefix (`TestAmendLatest`,
`TestSpecialSentinels`) exist today and gain the quoted cases.

1. **The roll.** `POST /api/roll/ability/{ability}` (`str`, `dex`, `con`, `int`, `wis`, `cha`; no
   body) rolls one d20 through the dice port. Any other ability is refused (400, `invalid_input`)
   before a die is rolled or anything is logged.
   Tests: `AbilityCheck_Step1_OneD20PerCheck` (one case per ability; exactly one d20, no other
   die), `AbilityCheck_Step1_UnknownAbilityRefusedBeforeTheRoll` (no die, no log entry);
   `router` `AbilityCheck_RollWireShape`.
2. **The target is the score in effect.** The target is the character's score in that ability with
   the effects the app applies to it, computed on the server through one port that returns the
   sheet's score and the score in effect for an ability. Today the only such effect is Strength
   magic ([ADR 0004](0004-buffs.md) steps 7 to 9, `buffsvc` effective STR); every other ability's
   score in effect is the sheet's. An effect on another ability later feeds the same port: the check
   has no case per ability or per buff.
   Tests: `AbilityCheck_Step2_TargetIsTheScoreInEffect` (STR raised by a Strength buff: the
   target is the effective STR, the sheet's STR kept beside it; DEX with no effect: the sheet's
   score; the sheet and the effective score distinct values).
3. **Verdict.** The check succeeds when the roll is at or under the adjusted target (step 5);
   without an adjustment the adjusted target is the target. Plain arithmetic, no clamp: an adjusted
   target at or under 0 always fails, a natural 1 succeeds whenever the adjusted target is 1 or
   more.
   Test: `AbilityCheck_Step3_AtOrUnderTheTarget` (at the target, one over, one under, natural 1
   against 1 and against 0).
4. **A natural 20 always fails**, whatever the adjusted target (an effective STR of 20 or more, or
   an easier adjustment), as for a proficiency check ([ADR 0014](0014-skill-checks.md) step 2).
   Test: `AbilityCheck_Step4_NaturalTwentyAlwaysFails` (adjusted target 22: 20 fails, 19
   succeeds).
5. **Logged and poppable.** Each check is one `ability_check` entry recorded in the check's
   transaction ([ADR 0018](0018-transactions-and-undo.md)), with no snapshots (it changes no state)
   and no summary. Its payload is `{ability, die, roll, abilityScore, target, adjustment,
   adjustedTarget, success, rolledSuccess}`, plus `adjustmentReason` when set: `abilityScore` is
   the sheet's score, `target` the score in effect (step 2), `adjustedTarget` is `target +
   adjustment`, `success` the verdict in effect and `rolledSuccess` the verdict as rolled (with
   adjustment 0). The roll endpoint returns that breakdown and the entry id; the UI words the
   journal line and the result card in en and fr, the backend sends no text. `ability_check` is one
   of the roll kinds (`actionlog.RollKinds`): it replaces the result card, and it ends the
   correction window of the roll before it ([ADR 0006](0006-saving-throws.md) step 6). Undo pops
   it like any top entry ([ADR 0010](0010-action-stack-and-undo.md)); popping restores nothing.
   Tests: `AbilityCheck_Step5_OneLoggedEntryPerCheck` (kind, no snapshots, no summary, the
   payload's fields), `actionlog` `IsRoll` ("ability check"); `actionlogsvc` `TestAmendLatest`
   ("save, a later ability check exists"); `router` `AbilityCheck_RollWireShape`.
6. **Adjustment after the roll, on the latest check.** `POST /api/roll/ability/{entryId}/adjust`
   `{adjustment, reason?}` sets the adjustment of the check. It **moves the target, + easier,
   - harder**: the adjusted target is `target + adjustment` (DEX 15 "at -2" is judged against 13).
   It replaces any earlier adjustment; 0 removes it and drops the reason. The verdict is recomputed
   from the stored roll and the new adjusted target, a natural 20 still failing (step 4): **the
   roll never changes**, `rolledSuccess` is kept. Only the latest roll can be adjusted
   (`actionlog.RollKinds`, voided entries included, as in [ADR 0006](0006-saving-throws.md) step
   6), and it must be an ability check: another kind is refused (400, `entry_not_amendable`), a
   voided check or one that is not the latest roll (409, `entry_already_voided` /
   `entry_not_latest`). No new refusal code is needed; a new sentinel would get its
   `internal/render` row and `apiError.<code>` keys.
   Tests: `AbilityCheck_Step6_AdjustmentMovesTheTarget` (fail to success, success to fail, at
   the adjusted target, a new adjustment replaces the previous one, 0 removes it and drops the
   reason, natural 20 with an easier adjustment; one d20 at the roll, no die at the adjustment),
   `AbilityCheck_Step6_LatestCheckOnly` (not the latest, voided, not an ability check);
   `actionlogsvc` `TestAmendLatest` ("not an ability check", "ability check, a later save exists",
   "latest ability check amended"); `router` `AbilityCheck_AdjustWireShape`,
   `TestSpecialSentinels` ("ability adjust not latest", "ability adjust voided", "ability adjust
   not an ability check").
7. **Audited and undoable.** Each adjustment is an `amend` correction on the entry carrying the
   recomputed payload (with `previousSuccess` and `previousAdjustment`); the entry is never edited.
   It is its own step on the action stack ([ADR 0010](0010-action-stack-and-undo.md) step 6, field
   `abilityAdjustment`, from and to the adjustment): undo pops the newest adjustment and the check
   reads the one before it, down to the check as rolled; redo replays it
   ([ADR 0019](0019-redo.md)).
   Tests: `AbilityCheck_Step7_AdjustmentIsAnAmendCorrection` (the correction's field, from and
   to; the amend's `previousSuccess` and `previousAdjustment`; two adjustments then one pop leave
   the first in effect).
8. **Ranges.** The adjustment is -20 to +20; the reason is optional, trimmed, at most 80
   characters. Anything else is refused (400, `invalid_input`) before the log is amended.
   Tests: `AbilityCheck_Step8_AdjustmentRanges` (-20, +20, -21, +21; reason of 80 and 81
   characters, a reason of spaces only reads as none); `router` `TestSpecialSentinels` ("ability
   adjust out of range").

## Consequences

- The frontend sends intent only (the ability, then the adjustment) and renders the breakdown; it
  never recomputes a verdict. The character card's stat tile is the roll button (click, Enter or
  Space); the sheet's score is on hover. The adjustment input lives on the result card.
- The verdict a check shows is always the arithmetic of its stored roll and adjusted target; there
  is no manual "mark as success". A DM ruling is an adjustment with a reason.
- The score in effect is read at the roll and stored in the payload: a buff added or dropped after
  the roll does not change a logged check, and adjusting it later uses the stored target.
- A check used "in lieu of a saving throw" (PHB p.102) is an ability check like any other: the app
  does not link it to a save category, and it has no save bonus from buffs.
- Proficiency checks keep the sheet's score ([ADR 0014](0014-skill-checks.md) Consequences); making
  them use the score in effect too would be an amendment of ADR 0014, not covered here.
- Not covered: an adjustment before the roll (the API takes no body; adding one later would follow
  ADR 0014 step 5), the last check per ability on the character card, checks by anyone but the
  Bard, exceptional Strength (18/xx) as a target, and the effect of a failed check (the DM's call).
