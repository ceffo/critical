# ADR 0003: Damage

**Status**: accepted, 2026-10-15 (critical-qki.35; collects the rules shipped in critical-qki.7,
critical-qki.12 and critical-qki.39, until then in `docs/backend.md` "Damage Formula"); steps 4, 5
and 11 amended 2026-10-16 by [ADR 0016](0016-inventory.md) steps 9 to 16 (an item's own powers)

_Amended by [ADR 0033](0033-equipment.md) (equipment, accepted 2026-10-18), on implementation: step 1 (a launcher's attack rolls its ammunition's dice; the launcher's own dice when the ammunition has none, ADR 0033 step 4). Steps 1 and 2 amended 2026-10-19 (the supervisor's ruling, picking from the catalogue, following the PHB): the damage as printed, its flat part K part of the weapon's standard damage; steps 3, 6 and 11 and [ADR 0030](0030-classes-and-races.md) step 19 follow (the backstab is its own factor)._

## Context

A hit ([ADR 0002](0002-attack-resolution.md)) deals damage from the weapon's dice, buffs that
multiply or add, the weapon's and the ammunition's magic, Strength and the house crit multiplier
([ADR 0001](0001-critical-hits.md)). Where each term goes in the formula changes the result
(the Power Gem's ×2 on the dice only, user ruling 2026-10-14), and crit multipliers may be
decimals, so the rounding has to be fixed too. The formula line in `docs/backend.md` read
`floor(… × Multipliers) + Str + Buff` while its text and the code floored once, at the end; this
ADR settles it as the floor of the whole.

## Decision

Tests live in `api/internal/domain/service/combatsvc/` unless named otherwise. The dice mocks
return a distinct value for each call and expect an exact number of calls.

1. **Dice.** The damage dice are the weapon's dice for the target size: its large-target dice
   against a Large target, its base dice against Small or Medium, and its base dice against a
   Large target when it has no large-target dice. They are rolled on every attack (ADR 0002
   step 1); a miss deals no damage and lists no damage term.
   Tests: `TestDamage_Step1_DiceByTargetSize`, `TestResolveAttack` (the misses: no damage, the
   dice stored). _Amended by ADR 0033 step 4_: a launcher that fires ammunition with dice rolls the
   ammunition's dice for the target size; a launcher with none of its own and ammunition without
   dice is refused before any die is rolled (field `ammoItemId`). Test:
   `TestDamage_ADR0033Step4_LauncherRollsAmmunitionDice`. A weapon with no dice and no
   ammunition (the mancatcher) is refused the same way on field `weaponId`.
   1.1. **The damage as printed** (2026-10-19, [ADR 0016](0016-inventory.md) step 8): `NdM`,
   `NdM+K`, `NdM-K` or a flat `K`. The N dice are rolled (none for a flat damage); their sum is
   BaseDamage, and K, the **DiceModifier**, is kept apart for step 2. The result and its log
   payload carry `diceModifier` (absent when 0), so a target AC correction recomputes it.
   Test: `TestDamage_Step1_DamageAsPrinted`.
2. **Formula.**

   ```
   Standard = BaseDamage × DiceMultiplier + DiceModifier
   Damage   = floor((Standard × SpellsBuff × Backstab + WeaponBuff) × Crit
                    + StrengthBonus + BuffDamage)
   ```

   with the terms of steps 3 to 8, computed at full precision and floored **once**, on the whole
   (step 9), never below 0 (step 10).
   **DiceModifier** (2026-10-19, the supervisor's ruling, following the book) is the flat part K
   of the damage as printed (step 1.1). It is part of the weapon's **standard damage** (the footman's
   mace's is 1d6+1), so whatever multiplies the standard damage multiplies it with the dice:
   SpellsBuff (Enlarge, PHB p.134: "damage rolls increase proportionately"; only Strength, class
   and magic bonuses are left as they are), the **Backstab** (PHB p.40: the weapon's standard damage
   is multiplied by Table 30, then Strength and magic are added; [ADR 0030](0030-classes-and-races.md)
   step 19) and the crit. Only the Power Gem's DiceMultiplier stays on the raw dice (step 4, user
   ruling "raw dice x2"): K is added after it. Strength, the weapon's magic and buff damage stay
   outside SpellsBuff and the backstab, as before. Backstab is 1 without a backstab; an attack
   logged before it had its own factor (`backstabMultiplier` absent, the backstab inside
   `diceMultiplier`, no K) recomputes as it was rolled. In the breakdown (step 11) K is a `weapon`
   term of kind `dice_modifier` whose detail is the damage as printed, listed after the dice
   multipliers; the backstab is a `standard_multiplier` term of kind `backstab`.
   A L5 thief (×3) hitting with a 1d6+1 mace on a 3 deals `(3 + 1) × 3 + Str`, not `3 × 3 + 1 + Str`.
   Tests: `TestResolveAttack_DamageTermsListEveryModifier` (every term at once:
   `((5 × 2 × 1.5) + 2) × 2 + 2 + 1 = 37`), `TestCrit_Step5_MultipliersStackFlooredOnce`; K's place,
   one test per multiplier: `TestDamage_Step2_FlatPartMultipliedBySpellsBuff`,
   `TestDamage_Step2_FlatPartAfterDiceMultiplier`, `TestDamage_Step2_FlatPartMultipliedByBackstab`,
   `TestDamage_Step2_FlatPartMultipliedByCrit`; `TestAdjustAttackAC_Step2_BackstabFactorIsLogged`
   (a correction recomputes the logged factor; an entry logged before keeps its total).
3. **SpellsBuff.** The product of the buffs' `spells_buff` factors (Enlarge), 1 without any. It
   multiplies the standard damage: the dice (after the DiceMultiplier) and K. Tests: `buffsvc` `TestAggregate` ("multipliers stack multiplicatively…"),
   `TestResolveAttack_DamageTermsListEveryModifier`.
4. **DiceMultiplier.** The product of the `multiplier` effects of the buffs that apply and of the
   item powers that are on (the Power Gem's thunder hammer ×2, melee only:
   [ADR 0016](0016-inventory.md) steps 12 and 16), 1 without any. The weapon keeps its own dice:
   the hammer doubles what they roll, it does not replace them. It multiplies **only the raw dice**: the weapon's "+N", Strength and buff damage are never
   multiplied by it (user, 2026-10-14: "x2 multiplier on raw dice result"). With the gem, 1d8 = 6
   gives `6 × 2 + 5 + STR`, not `(6 + 5) × 2 + STR`.
   Tests: `TestResolveAttack_PowerGem` ("x2 on the raw dice only, not on the +5"),
   `TestResolveAttack_DecimalCritMultiplier` ("x1.5 crit on a x2 dice buff doubles only the
   dice"), `buffsvc` `TestAggregate`, `TestItemPowers_Step12_HammerOnMeleeOnly` (×1 with a bow
   or darts), `TestItemPowers_Step10_TriggerAtEffectiveStr` (`buffsvc`: the hammer's power follows
   the effective STR).
5. **WeaponBuff.** The weapon's damage magic, or the "+N" of a buff or of an item power that is on
   when that wins (ADR 0002 step 6, the higher one, never both), plus the ammunition's magic
   (ADR 0002 step 7). Added after the dice factors, before the crit.
   Tests: `TestResolveAttack_OwnWeaponMagicBeatsLowerWeaponBuff`,
   `TestResolveAttack_WeaponBuffPlusMagicAmmo`.
6. **Crit.** The crit multiplier of ADR 0001 step 4 (1 without a crit). It multiplies the standard
   damage and the WeaponBuff together, after the DiceMultiplier, SpellsBuff and the backstab: a crit with the gem is
   `((dice × 2) + 5) × crit`. Strength and buff damage are not multiplied.
   Tests: `TestResolveAttack_PowerGem` ("crit with the gem: gem on the dice, crit on dice and
   weapon"), `TestResolveAttack_DamageTermsListEveryModifier`.
7. **StrengthBonus.** The PHB Table 1 damage adjustment for the effective STR (+2 at 18), for
   melee and hurled weapons only; none for a bow (same rule as ADR 0002 step 4). Added after the
   multipliers. Tests: `TestResolveAttack` ("long sword +2 vs AC 5 at effective STR 18 hits on 5":
   +2; "short bow +2 with a +3 arrow at DEX 17 hits on 4" at effective STR 18: none; "hurled darts
   take STR and DEX": +2).
8. **BuffDamage.** The sum of the buffs' `damage` effects, flat (may be negative), added after
   the multipliers. Tests: `TestResolveAttack` ("buff damage is a flat bonus after the
   multipliers"), `TestResolveAttack_DamageTermsListEveryModifier` (Rage +1 not doubled by the
   crit).
9. **One floor.** Decimal factors (×1.5 crit rows, ×1.5 buffs) are kept at full precision through
   the whole formula; the result is floored once, after the Strength and buff damage terms. A tiny
   epsilon absorbs float noise so that, for example, 100 × 1.15 gives 115, not 114.
   Tests: `TestResolveAttack_DecimalCritMultiplier`, `TestResolveAttack_DecimalFloorsAfterBonusDamage`,
   `TestResolveAttack_DecimalFloatNoiseKeepsThePoint`.
10. **Never below 0.** A negative result (negative buff damage) is 0.
    Tests: `TestResolveAttack_DecimalCritWithNegativeBonusClampsToZero`,
    `TestCrit_Step5_MultipliersStackFlooredOnce` ("never below 0").
11. **Breakdown.** Every contribution is listed in `DamageTerms`, in formula order: dice
    multipliers, the flat part K, spells, the backstab, weapon and ammunition magic, crit,
    Strength, buff damage. The breakdown shows the
    unrounded value and the floor (`… = 10.5 → 10`) when they differ. A term from an item power
    carries the item's id next to its stored name, and the client names it from the item
    (ADR 0016 step 14); an item power that is off lists no term.
    Tests: `TestResolveAttack_DamageTermsListEveryModifier`,
    `TestAdjustAttackAC_DamageTermsInFormulaOrder`, `TestResolveAttack_DecimalCritMultiplier`.
    `TestItemPowers_Step14_BreakdownCarriesTheItemID`, `buffsvc`
    `TestItemPowers_Step11_PowerOffContributesNothing`.

## Consequences

- Strength and buff damage are whole numbers, so flooring before or after adding them gives the
  same result; the formula is still written and computed as the floor of the whole.
- A target AC correction (ADR 0002 step 11) recomputes the damage from the stored dice and terms:
  the buffs as they were at the roll, not as they are at the correction.
- Not covered: damage resistance, damage reduction by the target, Stoneskin absorbing (a buff
  charge, not a damage term).
