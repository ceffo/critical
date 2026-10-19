# ADR 0012: Attack against an unknown AC

**Status**: accepted, 2026-10-15 (critical-kiv.7)

_Amended by [ADR 0033](0033-equipment.md) (equipment, accepted 2026-10-18), on implementation: the target's armour type is optional like its AC: unknown by default, given on the attack form or corrected after the roll (ADR 0033 step 18); then removed by [ADR 0035](0035-equipment-screens.md) item 1 (the target's armour is not asked)._

## Context

At the table the GM rarely says the target's AC before the roll. Until now an attack needed one
(`target_ac` required), so the player typed a guess and corrected it afterwards
([ADR 0002](0002-attack-resolution.md) step 11). Whether the attack crits also depends on the AC
([ADR 0001](0001-critical-hits.md) step 2), so the player can't tell the GM "I crit" without it.
Triage (2026-10-15): the server says which AC the roll hits and from which AC it crits, as
numbers; the GM then gives the AC (the existing correction), or only says hit or miss, in which
case there is no crit. Natural 1 and 20 stay plain arithmetic (ADR 0002 step 3); the handoff's
nat-20/nat-1 rule is not adopted.

## Decision

Tests live in `api/internal/domain/service/combatsvc/unknown_ac_steps_test.go`
(names start with the prefix `TestUnknownAC_StepN_`, N being the step number) unless named otherwise. The dice mocks return a distinct value for each
call and expect an exact number of calls (d20 once, d100 twice, the damage dice once per attack).

1. **The AC is optional.** `POST /api/roll/attack` may omit `target_ac` (or send null): the
   attack is against an unknown AC. Its result and log entry have `targetAc: null` and
   `acKnown: false` (`rolledAcUnknown: true` stays once the AC is given). Until it is settled
   (step 4 or 5) the attack has no hit (`hit: false`), no crit and no margin, and its damage is
   the one a hit at ×1 would do, for the card's "if it hits". The attack form's target AC is
   unknown by default and can be made unknown again (`PATCH /api/attack/form` with
   `"targetAc": null`); its preview then has no roll needed (`rollNeeded: null`).
   Tests: `TestUnknownAC_Step1_AttackWithoutAC`, `TestUnknownAC_Step1_ResultNotSettled`;
   `TestUpdateAttackForm_UnknownTargetAC`; `router` `TestResolveAttack_TargetACOptional`,
   `TestAttackForm_PatchUnknownTargetAC`;
   SQLite migration 019 kept the target AC of the existing forms (a past migration; the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3).
2. **Hits AC.** Every attack carries `hitsAc = effective THAC0 − d20`, the lowest target AC it
   hits: ADR 0002 step 2 (`d20 ≥ effective THAC0 − AC`) solved for the AC. The AC it names is hit,
   one lower is missed. It is the number `targetACNeeded` already held, kept for older entries.
   Test: `TestUnknownAC_Step2_HitsACIsEffectiveThac0MinusD20`.
3. **Crits from AC.** Every attack carries `critsFromAc = effective THAC0 − d20 + crit-check
   d100`, the lowest target AC it crits: ADR 0001 step 2 (`d20 − crit-check d100 ≥ effective
   THAC0 − AC`) solved for the AC. The multiplier d100 plays no part.
   Test: `TestUnknownAC_Step3_CritsFromACAddsTheCheckD100`.
4. **GM verdict.** While the AC is unknown, `POST /api/roll/attack/{id}/verdict` `{hit}` records
   the GM's `verdict: "hit" | "miss"` on the **latest roll** (the ADR 0002 step 11 rule): the
   attack hits or misses as told, with no crit (×1) and no margin; a hit does the damage rolled
   with the attack (ADR 0003). A later verdict replaces it. It is an audited amend: no die rolled,
   no ammunition spent. Refused (400) on an attack whose AC is known.
   Tests: `TestUnknownAC_Step4_VerdictSettlesWithoutCrit`,
   `TestUnknownAC_Step4_VerdictRefusedWhenACKnown`; `router` `TestAttackVerdict_WireShape`,
   `TestAttackVerdict_BadRequests`; `actionlogsvc` `TestAmendLatest` ("a later attack exists": a verdict
   goes to the latest attack only).
5. **Giving the AC.** The GM's AC goes through the target AC correction
   (`POST /api/roll/attack/{id}/adjust`): ADR 0001 step 6 with the stored d20, both d100s, the
   crit row of the roll and the damage dice; nothing is rolled. It replaces any verdict, the
   attack's AC is known from then on (a verdict is refused) and can still be corrected. There is
   no AC "as rolled" to show (`rolledTargetAc` stays absent).
   Tests: `TestUnknownAC_Step5_GivenACReplacesTheVerdict`,
   `TestUnknownAC_Step5_GivenACOverridesAMissVerdict`.
6. **Same dice.** An attack against an unknown AC rolls exactly what one against a known AC rolls,
   in the same order (ADR 0002 step 1): the d20, the crit-check d100, the multiplier d100, then
   the damage dice. A verdict or the AC given later rolls nothing; the ammunition is spent once,
   with the attack. Tests: `TestUnknownAC_Step6_SameDiceAsKnownAC`, `TestResolveAttack_RecordsAttackWithAmmoSnapshot` (the
   ammunition is spent with the attack).
7. **Crit hint bound.** The line to tell the GM mentions the crit only when some real AC can
   crit. The server sends `critPossible: true` when `critsFromAc` is at most 10, the highest
   armor class the rules allow (`combatsvc.MaxRealAC`), and false when the roll crits no real
   target (a "crit from AC 57" is noise). The frontend only reads the flag.
   Test: `TestUnknownAC_Step7_CritPossibleOnlyWithinRealACs`; `router`
   `TestClearAttackAC_WireShape`.
8. **Back to unknown.** After the AC was given (or the attack was rolled with one), the **latest**
   attack can go back to an unknown AC (`POST /api/roll/attack/{id}/unknown-ac`, no body), like
   the AC correction: an audited amend and its own undoable step (the step's `to` is null),
   no die rolled, no ammunition spent, refused like an AC correction (not an attack, voided, not
   the latest roll) and refused (400) when the AC is already unknown. The attack is unsettled
   again: no hit, no crit, no verdict, damage "if it hits"; the AC as rolled stays for the "was"
   line. The ledger stays append-only and undo pops the step.
   Tests: `TestUnknownAC_Step8_BackToUnknownAfterTheACWasGiven`,
   `TestUnknownAC_Step8_KnownAtRollGoesUnknown`; `actionlogsvc` `TestAmendLatest`, `TestEntry_PoppedStepAmendIsOutOfEffect`
   (only the latest attack; the step is popped by undo).

## Consequences

- The card of the latest attack shows "Hits AC X", the line to tell the GM, Touché / Raté
  (step 4), the real AC stepper with Corriger (step 5), and the damage dimmed "if it hits" until
  settled. The sentence is built in the frontend from `hitsAc` and `critsFromAc`; the server
  sends numbers only.
- A verdict and an AC given are each their own undoable step above the attack
  ([ADR 0010](0010-action-stack-and-undo.md) step 6): undo pops them newest first, then the attack
  (which gives the ammunition back).
- Attacks logged before this ADR have no `acKnown`: they read as known, with their `targetAc`.
- Migration 019 adds `attack_forms.target_ac_known`; forms saved before keep their AC (known).
- Not covered: an unknown AC for a spell's to-hit roll (ADR 0002 consequences).
