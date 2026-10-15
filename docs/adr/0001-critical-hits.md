# ADR 0001: Critical hits (house rule)

**Status**: accepted, 2026-10-13 (critical-qki.28; supersedes the one-d100 implementation from
critical-qki.7 / critical-qki.12); step 5 amended 2026-10-15 (critical-qki.35) to the buff
multipliers on the raw dice, as ruled 2026-10-14 and shipped in critical-qki.39

## Context

The table plays a house crit rule: a d100 rolled with the attack decides whether the hit is a
critical, and a d100 looked up in the DM's crit table decides how much it multiplies the damage.
Until 2026-10-13 the code rolled **one** d100 and used it for both, so a low crit check always
picked the low rows of the table. The user confirmed that these are two separate, independent
rolls. The target AC can also be corrected after the roll (critical-qki.28), so every die the
outcome depends on has to be stored.

## Decision

Each step has one named test in `api/internal/domain/service/combatsvc/crit_steps_test.go`
(`TestCrit_StepN_…`). The dice mocks return a distinct value for each call and expect an exact
number of calls (d20 once, d100 twice per attack).

1. **Rolls.** Every attack rolls the d20 and the crit-check d100 together.
2. **Crit check.** The attack is a crit when `d20 − crit-check d100 ≥ roll needed`, where the
   roll needed is `effective THAC0 − target AC`, the same number the hit uses. A crit is always a
   hit. A natural 1 or 20 is plain arithmetic.
3. **Multiplier roll.** Every attack also rolls a second, independent d100, the multiplier d100.
   It is rolled on every attack, miss or hit, crit or not, after the crit check and before the
   damage dice. The d20 and both d100s are stored in the attack's log entry.
4. **Multiplier.** On a crit the multiplier is the one of the house crit-table row
   (`crit_table_rows`, edited in the app) that holds the multiplier d100, in the table as it is at
   the roll. Until the table is edited it is the default 1–100 → ×2, flagged `CritPlaceholder`.
   Table multipliers may be decimals (×1.5). An attack that is not a crit has ×1.
5. **Damage.** The crit multiplier multiplies the dice part and the weapon bonus together, after
   the buff multipliers, which apply to the raw dice only (user, 2026-10-14: "x2 multiplier on raw
   dice result"; [ADR 0003](0003-damage.md) steps 4 and 6):
   `((dice × spells × buffs) + weapon) × crit + strength + buff damage`. It is computed at full
   precision and floored **once**, on the final value after the strength and buff damage terms
   (`RollResult.UnroundedDamage` → `TotalDamage`), never below 0. The breakdown shows
   `… = 10.5 → 10` when the result is fractional. Test: `TestCrit_Step5_MultipliersStackFlooredOnce`
   (`((5 × 1 × 3) + 2) × 1.5 = 25.5 → 25`; with the buff next to the crit it would be 31).
6. **Correction.** A target AC correction runs steps 2, 4 and 5 again with the stored d20,
   crit-check d100, multiplier d100 and its crit-table row, and the stored damage dice. Nothing is
   rolled again.

## Consequences

- The card, the damage term and the log line show both d100s ("crit check 7 · multiplier 63 →
  ×3").
- Attack entries logged before this decision are not rewritten. They still decode and display,
  but a target AC correction refuses them (400 "void it and roll again") because they have no
  multiplier d100.
- Editing the crit table never changes a logged attack, and doesn't change a correction of one
  either, because the row is stored at the roll.
