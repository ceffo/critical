# ADR 0002: Attack resolution

**Status**: accepted, 2026-10-15 (critical-qki.35; collects the rules shipped in critical-qki.7,
critical-qki.13, critical-qki.28, critical-qki.33 and critical-qki.34, until then in
`docs/backend.md` "Attack Modifiers"); step 6 and the Consequences amended 2026-10-16 by
[ADR 0016](0016-inventory.md) steps 9 to 16 (an item's own powers). Step 2 amended by [ADR 0030](0030-classes-and-races.md) step 5 (THAC0 derived), implemented 2026-10-18.

_Amended by [ADR 0033](0033-equipment.md) (equipment, accepted 2026-10-18), on implementation: steps 2 and 12 (a new to-hit modifier `armor_type`, weapon type against the target's armour type, ADR 0033 step 19) and step 11 (the target's armour corrected after the roll like its AC, ADR 0033 step 18); then removed by [ADR 0035](0035-equipment-screens.md) item 1 (no target armour, no `armor_type` modifier)._

_Amended by [ADR 0037](0037-proficiencies.md) (proficiencies, proposed 2026-10-20), implemented 2026-10-20: steps 2 and 12 gain the to-hit modifier `nonproficient` (PHB Table 34's penalty for a weapon type without a pick, listed after the buffs and before the situational modifier, ADR 0037 step 8; detail the weapon type), and the Consequences' "proficiency penalties" leave the not covered. Tests: ADR 0037 step 8's._

_Amended by [ADR 0030](0030-classes-and-races.md) step 22.3 (the racial to-hit bonus, accepted by the user 2026-11-13), implemented 2026-11-13: steps 2 and 12 gain the to-hit modifier `race` (the race's `weapon_to_hit` trait, [ADR 0040](0040-classes-are-data.md) step 4, when the weapon's type is one it lists; detail the race code; not magical, to hit only), listed after Dexterity and before the weapon's magic; the Consequences' order gains it. Tests: ADR 0030 step 22.3's._

_Step 4 amended 2026-11-13 (the strength bow; the user's triage of 2026-11-13, the book's rule rather than a composite-bow house rule), implemented 2026-11-13: a bow takes a Strength penalty always and a Strength bonus only as a strength bow, up to its Strength rating ([ADR 0033](0033-equipment.md) step 3); a crossbow never takes Strength. Step 4.1 (a warning, no roll, when an archer without exceptional Strength wields a bow built for it) is **proposed**, awaiting the user's OK. [ADR 0003](0003-damage.md) step 7 follows. **Behaviour change**: before, a bow took no Strength at all, so an archer with a Strength penalty now takes it with every bow._

## Context

The app resolves the Bard's weapon attacks: it rolls the dice, adds every to-hit modifier and says
hit or miss, then works out the damage ([ADR 0003](0003-damage.md)) and the house crit
([ADR 0001](0001-critical-hits.md)). The table plays AD&D 2nd edition with THAC0 and a few user
rulings. The rules were written as prose in `docs/backend.md`; this ADR turns them into numbered
steps, each proven by a named test, so a regression in one step fails one test.

## Decision

Tests live in `api/internal/domain/service/combatsvc/` unless named otherwise. The dice mocks
return a distinct value for each call and expect an exact number of calls.

1. **Dice.** Every attack rolls, in this order: the d20, the crit-check d100, the multiplier d100
   (ADR 0001 steps 1 and 3), then the damage dice for the target size (ADR 0003 step 1). All of
   them are rolled on every attack, hit or miss, through the dice port, and stored in the attack's
   log entry, so a later correction (step 11) never rolls again.
   Test: `TestAttack_Step1_DiceInOrderOnEveryAttack`.
2. **Hit.** The effective THAC0 is the character's THAC0 minus the sum of every to-hit modifier
   (steps 4 to 9). The attack hits when `d20 + modifiers ≥ THAC0 − target AC`, that is
   `d20 ≥ effective THAC0 − target AC` (the roll needed). The margin is `d20 − roll needed`.
   Tests: `TestResolveAttack` ("long sword +2 vs AC 5 at STR 12 hits on 6" / "… misses on 5").
3. **Natural 1 and 20.** Plain arithmetic: no automatic miss, no automatic hit (user ruling, also
   for the crit check, ADR 0001 step 2). Tests: `TestResolveAttack` ("natural 1 is plain THAC0
   math: hits when 1 is enough", "natural 20 is plain THAC0 math: misses when 21 is needed").
4. **Strength.** The PHB Table 1 hit adjustment for the **effective** STR (the sheet's STR with
   the Strength buffs, [ADR 0004](0004-buffs.md)), for melee and hurled weapons.
   _(Amended 2026-11-13, the strength bow, replacing "a bow (a weapon that fires ammunition) gets
   none".)_ A **bow** (weapon type `long_bow` or `short_bow`, composite bows included; PHB Table 44
   prices and sizes the composite bow but gives it no Strength of its own):
   - a Strength **penalty** of the effective STR always applies, each adjustment on its own (STR 7:
     -1 to hit, no damage penalty). PHB p.90-91 "Strength Modifiers": a character with a Strength
     penalty always suffers it with a bow;
   - a Strength **bonus** applies only to a **strength bow**, a bow with a Strength rating (ADR
     0033 step 3): the adjustment of the **lower** of the effective STR and the rating, 18/xx
     compared by its percentile (STR 18/50 with a bow rated 17: the 17 row). PHB p.99 "Ability
     Modifiers in Missile Combat" (and DMG p.62): with a bow, the Strength attack and damage
     modifiers apply only with a bow prepared for the character; PHB p.73 "Bows": a heavier pull
     grants the bonus of a higher Strength. An ordinary (unrated) bow gives no bonus;
   - the Strength modifier's detail is the score whose row applied (the rating's when it is the
     lower), and it is listed only when the bow is rated or an adjustment is a penalty.

   A crossbow takes no Strength, bonus or penalty (PHB p.99: never a Strength bonus with a crossbow
   or another mechanical device; PHB p.73 "Crossbow" and p.90-91: no Strength modifier at all). Every other
   launcher (a sling, a launcher with no weapon type) keeps taking none: unchanged. One Strength
   path serves the roll and the preview (`combatsvc` `strengthOf`).
   _(Amended by [ADR 0046](0046-derived-figures.md) Q6, 2026-11-03: the table, its validation and the lookup are
   the `strength` package's (`strength.Table.Of(score, exceptional)`); combat is handed the table as a
   value (`Deps.Strength`) and keeps no Strength port. The effective score still comes from the buffs.)_
   Tests: `TestResolveAttack` ("long sword +2 vs AC 5 at effective STR 18 hits on 5", "short bow
   +2 with a +3 arrow at DEX 17 hits on 4" at effective STR 18, "hurled darts take STR and DEX"),
   `TestResolveAttack_Step4_StrengthWithBowsAndCrossbows` (rated 18/50 at 18/50: +1/+3, Table 1's
   18/01-50 row; rated 17 at 18/50: the 17 row; an unrated bow at STR 5: -2/-1; a light crossbow:
   none), `TestPreviewAttack_Step4_StrengthWithBowsMatchesTheAttack`, `composition`
   `TestStrengthBow_Step3_TheRatingOnTheWireAndInTheAttack`.

   4.1. _(Proposed 2026-11-13, awaiting the user's OK; the default applied meanwhile.)_ **Below
   the rating.** A weaker archer still shoots a strength bow: he takes the lower adjustment, his
   own Strength's (step 4). The book asks more only of a bow built for exceptional Strength: PHB
   p.73 "Bows", such a custom bow (for 18/01 or more) takes a successful bend bars/lift gates roll
   to string or use for a character without exceptional Strength.
   Proposal, as the book scopes it: a wielded bow rated 18/01 or more, in the hands of a character
   whose Strength in effect (the attack's, with the Strength buffs, read from `buffsvc`) is below
   18/01, carries the class warning `bow_too_strong` ([ADR 0033](0033-equipment.md) step 20's
   list). No roll is made and nothing is refused. A character with exceptional Strength is never
   warned, whatever the rating (18/50 with a bow rated 18/90: no warning, the 18/50 row applies),
   and a bow rated 18 or less never warns. The alternative, nothing at all, is a one-line removal.
   Tests: `itemsvc` `TestBowWarnings_Step4_1_ABowBuiltForExceptionalStrengthWithoutIt`,
   `TestWarnings_Step4_1_TheBowWarningReadsTheStrengthInEffect`, `composition`
   `TestStrengthBow_Step3_TheRatingOnTheWireAndInTheAttack`.

5. **Dexterity.** The PHB Table 2 missile attack adjustment for every missile weapon, bow or
   hurled; none in melee. _(Read from the character view's `DexMissile`, not from `combatsvc`'s own
   rules port: [ADR 0046](0046-derived-figures.md) step 2.1, implemented 2026-10-23.)_ Tests: `TestResolveAttack` (same three cases as step 4).
6. **Weapon magic.** The weapon's own magic, or a "the weapon becomes +N" from a buff or from an
   item power that is on (the Power Gem's thunder hammer, +5) when that is at least as high: the
   higher one, never both. To hit and to damage alike. Several "+N" do not add up either, buffs
   and powers together: the best one counts. A "+N" with a `weapon_kind` condition counts only
   with a weapon of that kind (the hammer: melee only, [ADR 0004](0004-buffs.md) step 18). The
   modifier names its source: the buff's name, or the item's id and stored name for a power
   (ADR 0016 step 14).
   Tests: `TestResolveAttack_OwnWeaponMagicBeatsLowerWeaponBuff`, `TestResolveAttack_PowerGem`
   ("active at effective STR 18": the gem's +5 replaces the sword's +2), and in `buffsvc`
   `TestAggregate` ("… weapon +N: best wins"), `TestItemPowers_Step12_HammerOnMeleeOnly`,
   `TestItemPowers_Step14_BreakdownCarriesTheItemID` (`combatsvc`),
   `TestItemPowers_Step11_PowerJoinsTheAggregate` (`buffsvc`).
7. **Ammunition magic.** A magic arrow's bonus is added to hit and to damage, on top of step 6
   (DMG "Bow +1": with magic arrows, total the bonuses of bow and arrow).
   Tests: `TestResolveAttack` ("short bow +2 with a +3 arrow at DEX 17 hits on 4"),
   `TestResolveAttack_WeaponBuffPlusMagicAmmo`.
8. **Buff to-hit.** Each buff's `to_hit` effect is its own modifier, labelled with the buff.
   Tests: `TestAttack_Step8_OneToHitModifierPerBuff`, `TestResolveAttack` ("buff to-hit (Bless +1)
   turns a 5 into a hit").
9. **Situational modifier.** One signed value entered at the table, with an optional label
   ("Situational" when blank); 0 adds nothing and is not listed.
   Tests: `TestResolveAttack` ("situational +2 (rear attack)", "situational -4 without a label",
   "situational 0 is not listed").
10. **Ammunition.** A weapon with an `ammo_kind` needs an item of that kind; every attack spends
    one, in the same transaction as the attack's log entry. The attack is refused, with nothing
    rolled or spent, when no ammunition is chosen, when none is left, when the item is not of the
    weapon's kind, or when a weapon without an ammo kind is given some. Voiding the attack gives
    the ammunition back. Tests: `TestResolveAttack_Rejections` (the ammunition cases),
    `TestResolveAttack_RecordsAttackWithAmmoSnapshot`, and `itemsvc`
    `TestRestorers_WriteTheBeforeCountsBack` (the snapshot a void writes back).
11. **Target AC correction.** The target AC of the **latest roll**, when it is an attack that is
    not voided, can be corrected after the roll (within ±20). Hit, margin, crit and damage are
    worked out again from the stored dice (ADR 0001 step 6): no die is rolled, no ammunition is
    spent. The correction is an audited amend; the entry keeps the AC and outcome as rolled. Each
    correction is its own undoable step ([ADR 0010](0010-action-stack-and-undo.md) step 6).
    Tests: `TestAdjustAttackAC_Recompute`, `TestAdjustAttackAC_SpendsNoAmmo`,
    `TestAdjustAttackAC_Rejections`, and in `actionlogsvc` `TestAmendLatest`.
12. **Preview.** Before rolling, the attack form's current choices (weapon in hand, ammunition,
    target AC, situational modifier) give the to-hit modifiers of steps 4 to 9, the effective
    THAC0 (step 2) and the roll needed against the target AC (`effective THAC0 − target AC`), with
    the same code as the attack: nothing is rolled and nothing is spent. The chosen ammunition
    counts even when spent. The preview also sums the flat damage terms (weapon or buff "+N",
    ammunition, Strength and buff damage, [ADR 0003](0003-damage.md)) as the damage bonus shown
    beside the dice. Tests: `TestPreviewAttack_Step12_SameModifiersAsAttack` (every case of
    `TestResolveAttack`, no die rolled), `TestPreviewAttack_Step12_DamageBonus`, `TestGetAttackForm_Ammunition`.

## Consequences

- Every to-hit modifier is listed in the roll's `Modifiers`, in the order of the steps above
  (Strength, Dexterity, weapon or buff "+N", ammunition, buffs, situational), the
  non-proficiency penalty between the buffs and the situational one (ADR 0037 step 8), the
  race's to-hit bonus between Dexterity and the weapon's magic (ADR 0030 step 22.3).
- The Power Gem's "+5" (step 6) and its ×2 (ADR 0003 step 4) are its thunder hammer, an item
  power on while the effective STR is 18 or more (user, 2026-10-13: "not exactly 18 >= 18"), with
  a melee weapon only ([ADR 0016](0016-inventory.md) steps 10, 12 and 16). A hand-made buff with
  the same effects and a `min_str` gate still works as before ([ADR 0004](0004-buffs.md) step 10).
  Tests: `buffsvc` `TestStrength_Step10_PowerGemFrom18Up` (the gate: STR 17 off; 18, 19, 23 on),
  `TestResolveAttack_PowerGem` ("ignored at effective STR 17"), `buffsvc`
  `TestItemPowers_Step10_TriggerAtEffectiveStr`.
- A tie between the weapon's own magic and a buff "+N" shows the buff in the breakdown; the value
  is the same.
- Exceptional Strength (18/xx) applies to warriors only ([ADR 0030](0030-classes-and-races.md)
  step 9); a strength bow's rating may be an 18/xx (step 4).
- A spell's to-hit roll (`attack_roll` in the spell effects data, e.g. Chromatic Orb, a hurled
  spell) follows steps 2 to 5 and 8 but never crits: it rolls
  one d20 only (user, 2026-10-14: "same rules as combat"). Test: `TestRollSpellToHit`.
- Not covered: called shots, weapon speed, two-weapon fighting, proficiency penalties, range
  modifiers other than through the situational modifier. _(Amended by ADR 0037 step 8,
  implemented 2026-10-20: the non-proficiency penalty is covered; the related-weapons half
  penalty is not.)_
