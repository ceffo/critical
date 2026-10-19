# ADR 0016: Inventory and editors

**Status**: accepted, 2026-10-15 (critical-qki.37; collects the rules shipped in critical-qki.13,
critical-qki.31 and critical-qki.32, until then in `docs/backend.md`); steps 9 to 16 (an item's own
powers) added 2026-10-16 (critical-qki.45), amending [ADR 0002](0002-attack-resolution.md) step 6,
[ADR 0003](0003-damage.md) steps 4, 5 and 11 and [ADR 0004](0004-buffs.md) steps 10 and 11 and its
Consequences

_Amended by [ADR 0033](0033-equipment.md) (equipment, accepted 2026-10-18), on implementation: steps 1 and 4 (one possessions list with categories instead of the type label), steps 7 and 8 (weapons are pieces of the same list and editor), step 9 (powers on only while the piece is worn or wielded) and step 15 (the editor writes powers), ADR 0033 steps 1 to 9, 17 and 24 to 27; step 8 (damage as printed, 2026-10-19)._

_Vocabulary: one possession of any category is an item, the list is the inventory ([ADR 0036](0036-vocabulary.md) step 5)._

## Context

The Bard carries arrows, flasks, a wand and a Power Gem. User, 2026-10-13: *"these are all
consumables... it's just their type"*. One item model has to cover them all, and the player must be
able to fix the lists (a new quiver, a weapon enchanted by the DM) without a deploy. Every change
goes through the action log, which is a stack ([ADR 0007](0007-hp-ledger.md) step 8).

Some items do something by themselves. The Power Gem is set in Kelsen's forehead: it is not going
anywhere without him dying. It is a warrior's gem, but with Strength magic the bard reaches its
requirement and its powers wake up. User, 2026-10-15: *"the power gem is supposed to activate its
powers when its user has STR >= 18"*; *"its powers on the weapon. the dispel magic once a day is
available regardless"*. Until now the gem's "+5" and "×2" were a hand-made buff with a `min_str`
gate ([ADR 0004](0004-buffs.md) step 10): something the player had to put on the list, keep and
never dismiss by mistake, for an effect that belongs to the item. Steps 9 to 16 give items their own
powers: effects that apply while a trigger is met, like a buff that is never on the list. Decided
in triage with the user, 2026-10-15/16.

## Decision

Tests live in `api/internal/domain/service/` (`itemsvc`, `weaponsvc`) and `api/internal/composition/`
unless named otherwise.

1. **One model, three count behaviours.** An item has a name, a free-text type label (arrow, flask,
   wand, gem...), a count, a note and optional mechanics (effect dice and save, a magical bonus,
   an ammo kind). Only its `behaviour` changes how the count moves:

   | Behaviour | Use (-) | Restock (+) | At 0 | Long rest |
   |---|---|---|---|---|
   | `stock` | yes | yes | stays listed | untouched |
   | `charges` | yes | refused (409) | removed | untouched |
   | `daily` | yes | refused (409) | stays listed | refilled to its daily number |

   Tests: `TestBehaviour_Rules`, `TestUse_RemovesChargesItemAtZeroOnly`, `TestRestock_AddsToStockAndLogs`,
   `TestRestock_RejectsChargesAndDaily`, `TestUseItem_SpendsStockAmmo`.
2. **Use and restock are logged.** Spending one of any item (the attack's arrow, the inventory's
   minus) is one count change with an `items:<id>` snapshot; the use that takes a `charges` item to 0
   deletes it in the same entry, and popping that entry puts it back in its list position with one
   charge. Using an item with effect dice rolls them. A restock adds 1 to 999 to a `stock` item as
   one `item_restock` entry.
   Tests: `TestUseItem_RollsEffectDiceAndLogsWithSnapshot`, `TestUseItem_LastChargeRemovesTheItem`,
   `TestRestock_RejectsOutOfRangeCounts`, `TestRestorers_PutBackARemovedItem`,
   `TestUseSnapshot_MarksARemovedItem`.
3. **Long rest refills daily items only.** Each `daily` item goes back to its daily number; stock and
   charges are untouched. Passing time refills nothing ([ADR 0005](0005-rounds-and-time.md) step 7).
   Tests: `TestResetDaily_ResetsThenLists`, `TestLongRest_ResetsBothInOneLoggedTransaction`.
4. **Item editor.** Items are added, edited (only the fields sent) and removed through the editor;
   each is one entry (`item_add`, `item_edit`, `item_remove`) with the `items:<id>` snapshot.
   Popping the newest entry deletes a created item, writes back every "before" field of an edited
   one, or re-inserts a removed one with its id, position and count. A new item's id is the slug of
   its name (suffix `_2`, `_3` when taken) and it goes last in the list. Names are language maps,
   written in the player's language ([ADR 0023](0023-localized-text.md) steps 9 to 12).
   Tests: `TestCreate_InsertsLogsAndSnapshots`, `TestUpdate_ChangesOneFieldAndLogsBeforeAfter`,
   `TestDelete_RemovesAndLogsWholeItem`, `TestRestorers_UndoCreateEditDelete` (itemsvc).
5. **Item validation.** An invalid item is refused with a message naming the field (400
   `{error, field}`) before anything is stored or logged: a name is required; the behaviour is
   one of the three; the count is 0 to the maximum, and a `charges` item has at least 1 charge
   (removing it is what remove is for); a `daily` item has a daily number of at least 1 and a count
   not above it; the magical bonus is -10 to 10; effect dice are `NdM` or `NdM+K`; the effect save is
   one of the five save categories ([ADR 0006](0006-saving-throws.md)). An edit that changes nothing
   is refused.
   Tests: `TestCreate_ValidationNamesTheField` (itemsvc), `TestUpdate_RejectsBadValuesAndNoOps`,
   `TestParseDice`.
6. **Ammunition.** Ammo is any item whose ammo kind matches the weapon's; the attack offers those
   with a count above 0 ([ADR 0002](0002-attack-resolution.md) step 10).
   Tests: `TestResolveAttack_Rejections` (the ammunition cases), `TestGetAttackForm_Ammunition` (`combatsvc`).
7. **Weapon editor.** Weapons are added, edited and removed the same way (`weapon_add`,
   `weapon_edit`, `weapon_remove`, snapshot `weapons:<id>`, pop-only). A remove is a hard delete:
   old attack entries keep the weapon's name and id in their payload, and popping the remove
   re-inserts the same id, position and fields. The next attack reads the table, so an edit applies
   at once.
   Tests: `TestCreate_InsertsLogsAndSnapshots`, `TestDelete_RemovesAndLogsWholeWeapon`,
   `TestRestorers_UndoCreateEditDelete` (itemsvc since ADR 0033), `TestResolveAttack` (`combatsvc`: the weapon is read
   from the table at each attack).
8. **Weapon validation.** Damage dice are `NdM` only (the attack rolls NdM; the weapon's magic is
   its own field), for the small/medium and the large column alike; to-hit and damage magic are
   separate, each -10 to 10; the kind is melee or missile and a melee weapon has no ammo kind. Field
   names in the error are snake_case (`damage_sm`, `magic_to_hit`...).
   Tests: `TestCreate_ValidationNamesTheField` (weaponsvc), `TestUpdate_RejectsBadValuesAndNoOps`.
   _Amended 2026-10-19 (ADR 0033 steps 4 and 24, the supervisor's ruling on picking from the
   catalogue)_: the damage is stored **as printed**: `NdM`, `NdM+K`, `NdM-K` (1 to 100 dice of 2 to
   1000 sides, `|K|` at most 20) or a flat `K` (1 to 20), as the PHB prints the footman's mace
   (`1d6+1`), the jousting lance (`1d3-1`) or the needle (`1`); the attack rolls the dice and adds K
   at its place in the formula ([ADR 0003](0003-damage.md) step 2). A launcher, and a catalogue piece
   with no damage type (the mancatcher, picked with its catalogue code), may have no damage, both
   columns or neither; a homebrew weapon (no catalogue code) needs its dice even with no damage type,
   so a typed weapon cannot be saved into one its attack refuses. Field names are the
   wire's camelCase (`damageSm`). Tests: `itemsvc` `TestValidate_Steps2To5_CategoryFields`.

### An item's own powers (steps 9 to 16)

Tests live under `api/internal/` unless their path starts with `web/`.

9. **Powers are part of the item.** An item may carry a list of powers. A power has an id (a slug,
   unique within its item), an optional **trigger** and a set of **effects**. The effects are the
   buff effect kinds of [ADR 0004](0004-buffs.md) step 11 (`weapon_buff`, `multiplier`,
   `spells_buff`, `to_hit`, `damage`, `save_bonus`, `ac`) with their conditions (ADR 0004 step 18);
   a Strength effect (`str_bonus`, `str_max`) is refused, so a power never changes the effective
   STR that its own trigger reads. A power has no name, rounds or charges: it is named by its item
   (step 14) and lasts as long as the item is carried. Its count does not matter: the Power Gem's
   powers work at 0 dispels left, and spending the dispel does not need STR 18 (user: "the dispel
   magic once a day is available regardless"). Removing the item removes its powers.
   Tests: `domain/service/itemsvc` `TestItemPowers_Step9_StrengthEffectsRefused`,
   `TestItemPowers_Step9_CountDoesNotGateThePowers`; `storage/postgres`
   `TestSeed_Step9_RejectsAPowerWithAStrengthEffect`.
10. **Trigger.** A power's trigger says when it is on. The one trigger kind is `min_str`: the power is
    on while the effective STR (ADR 0004 steps 7 to 9) is at least that value (1 to 25), the same
    comparison as a buff's `min_str` gate (ADR 0004 step 10), 18 **or more** (user, 2026-10-13: "not
    exactly 18 >= 18"). `min_str` is a trigger here, not an effect: it decides whether the power
    exists for the roll, never what it adds. A power without a trigger is always on. Nothing is
    logged when a power switches on or off: it follows the STR, which follows the buffs, and those
    are logged (ADR 0004 step 13); voiding a Strength buff switches the power back by itself.
    Tests: `domain/service/buffsvc` `TestItemPowers_Step10_TriggerAtEffectiveStr` (STR 17
    off; 18, 19 and 23 on; a power without a trigger always on), `TestStrength_Step10_PowerGemFrom18Up`
    (the gem follows the effective STR).
11. **Powers join the buffs' aggregate.** The buffs' aggregate (ADR 0004 step 11), used by the
    attack, the attack preview, the save and the spell to-hit, reads the carried items' powers
    through an items port of `buffsvc` (wired in the composition root) and folds every power that is
    on into the same totals as the buffs that apply, by the same rules: factors multiply, flat
    bonuses add, `weapon_buff` is the best single +N across buffs and powers. The effective STR is
    worked out from the buffs alone, then the triggers are read against it. Order: the buffs in
    list order, then the powers in item list order, then power order; on a `weapon_buff` tie the
    first one wins (a buff before a power).
    Tests: `domain/service/buffsvc` `TestItemPowers_Step11_PowerJoinsTheAggregate` (a power
    and a buff `multiplier` multiply, `to_hit` adds, the best `weapon_buff` wins across both, a tie
    goes to the buff), `TestItemPowers_Step11_PowerOffContributesNothing`.
12. **Melee only.** A power's effects may carry the `weapon_kind` condition of ADR 0004 step 18.
    The thunder hammer's is `melee`: its +5 and ×2 count for an attack (and its preview) with a
    melee weapon, not with a bow nor a hurled weapon (a hurled weapon is a missile weapon,
    [ADR 0002](0002-attack-resolution.md) steps 4 and 5), and never for a save or a spell's to-hit,
    which have no weapon. The power is still on (step 10) and still shown (step 13) while a bow is
    in hand; it just adds nothing to that roll.
    Tests: `domain/service/combatsvc` `TestItemPowers_Step12_HammerOnMeleeOnly` (long sword:
    +5 and ×2; short bow and darts: the weapon's own magic, ×1), `TestItemPowers_Step12_PreviewFollowsTheWeaponInHand`.
13. **Shown only while it is on.** A power whose trigger is not met appears nowhere: not on the
    effects timeline, not in any breakdown, not in the preview. While it is on, the effects list
    (`GET` of the active buffs) serves it in its own list next to the buffs, never mixed into them:
    the item's id, the power's id and its effects, no counters, nothing to edit, absorb or dismiss.
    The timeline shows it under the item's name, **only when it has a trigger**: a power with none
    (magic armour, a ring, a cloak) is on for as long as the piece is equipped, so it would hold a
    permanent row on a timeline meant for what comes and goes; it gets no row, yet joins the
    aggregate (step 11) and keeps its item's name in the breakdowns (step 14). Hand-made buffs with a `min_str` keep ADR 0004
    step 10: on the list, shown as not applying ("needs STR N").
    The list answers every buff change too (`{buffs, powers}`), so a Strength buff added or
    dismissed brings the power with it. The timeline shows how long it lasts from the item's
    trigger ("while STR ≥ 18").
    Tests: `domain/service/buffsvc` `TestItemPowers_Step13_ListedOnlyWhileOn`; `rest/router`
    `TestItemPowers_Step13_PowersOnTheWire`; `web/src/lib/itemPowers.test.ts` "powerRows" (the
    power at STR 18 under the item's name, none at 17, none without a trigger, nothing to edit or
    dismiss); e2e
    `buffs.spec` "a Strength buff reaching 18 turns the Power Gem's hammer on".
14. **Named by its item.** Every contribution of a power in the aggregate, in the attack and save
    breakdowns (`Modifiers`, `DamageTerms`) and in the log entries that store them carries the
    item's id (`itemId`) next to the stored name of the item (the `Detail`, a fallback). The client
    names it from the item by id, so it shows the item's current name and, with the localization of
    item names, the player's language; a `Detail` with no `itemId`, or an id no item has any more
    (removed since), shows as stored. A log entry recorded before keeps its stored name; payloads are
    never rewritten. The backend sends no wording ([ADR 0023](0023-localized-text.md)).
    Tests: `domain/service/combatsvc` `TestItemPowers_Step14_BreakdownCarriesTheItemID` (the
    `weapon_buff` modifier, the `weapon` and `dice_multiplier` damage terms, the log payload);
    `web/src/lib/rollText.test.ts` "names an item power's term from the item, the stored name
    otherwise".
15. **Stored with the item, logged with it.** Powers are stored on the item row and are part of its
    `items:<id>` snapshot, so every item entry (use, restock, add, edit, remove) carries them and
    popping it restores them with the item (steps 2 and 4): a removed Power Gem popped back has its
    powers again. Powers come from the seed and from data migrations only: the item editor shows no
    power field and sends none, an edit or create body's `powers` key is accepted and ignored (an
    item served back as is still decodes; any other unknown key stays refused), and a created item
    has none. An item is served with its powers, read only. (*Amended by
    [ADR 0033](0033-equipment.md) step 17:* the editor writes powers, validated as the seed's.) A snapshot recorded before powers
    existed has no `powers` key: popping it leaves the item's stored powers as they are, and an item
    it puts back while it is gone comes back without powers (see Consequences).
    Tests: `domain/service/itemsvc` `TestItemPowers_Step15_SnapshotCarriesThePowers`,
    `TestItemPowers_Step15_AnEditWithoutPowersKeepsThem`, `TestItemPowers_Step15_OldSnapshotKeepsTheStoredPowers`;
    `rest/router` `TestItems_ADR0033Step17_PowersInTheBody` (since ADR 0033 step 17).
16. **The Power Gem.** One item, "Power Gem" (type `gem`, `daily`, 1 a day), with two powers: its
    daily Dispel Magic, which stays the item's daily use (steps 1 to 3: no STR condition, 1/day, the
    10 m radius in its note, as today; not an entry of its power list), and the **thunder hammer**,
    its one listed power: trigger `min_str` 18, effects `weapon_kind`
    `melee`, `weapon_buff` 5 ("the weapon becomes +5", to hit and damage) and `multiplier` 2 (the
    weapon's raw damage dice doubled; the weapon keeps its own dice). The damage is ADR 0003 step 2
    unchanged: `(((dice × enlarge × 2) + weapon) × crit) + strength + buff damage`, and the +5
    against the weapon's own magic is ADR 0002 step 6 unchanged: the higher one, never both. The
    seed (`seeds/items.json`) carries it for a new database; a data migration gives it to an
    existing one: on the row `power_gem_dispel` (the id stays: log entries and snapshots refer to
    it), it sets the powers and renames "Power Gem: Dispel Magic" to "Power Gem" when the name is
    still that one exactly (a name the player changed is kept). The gem's other forms (the trident of
    submission, the undead-destroying mace) and the hammer's stun are not modelled.
    The migration (032) also replaces the seed's old note ("handled by buffs") when it is still
    that one word for word.
    Tests: `storage/postgres` `TestSeed_Step16_PowerGemHasTheHammer` (SQLite migration 032 gave the
    played gem its powers; a past migration, the per-migration test was deleted with the squash into the PostgreSQL baseline, [ADR 0025](0025-postgresql-storage.md) step 2.3);
    `buffsvc` `TestItemPowers_Step10_TriggerAtEffectiveStr`, `TestItemPowers_Step11_PowerJoinsTheAggregate`;
    `combatsvc` `TestItemPowers_Step12_HammerOnMeleeOnly` (the hammer's bonus and doubled dice on a
    melee weapon, nothing on a bow).

## Consequences

- A `charges` item cannot sit at 0, so "empty wand" is not a state; a stock item can, and so can a
  `daily` one until the long rest.
- Because the log is a stack, fixing an old item edit means popping what came after it.
- An item power is not a buff: it cannot be dismissed, edited or renewed, it has no counters and
  Next Round never touches it. A hand-made "Power Gem" buff from before step 16 is left as it is; the
  player dismisses it once (with the item's power on, its ×2 would multiply with the power's into
  ×4).
- Powers have no editor fields; changing one is a data migration.
- A snapshot from before step 16 has no powers, and with the item gone there are no stored powers to
  keep: popping it puts the item back without its powers. Only a remove (by hand, or a use that
  took a `charges` item to 0) recorded before migration 032 and popped after it does that, and
  the gem is `daily`, so only a gem removed by hand before 032 could come back powerless. Restoring
  from the seed instead would make the item service read the seed, which storage loads once into
  an empty table, for entries that no database has: the live log holds no remove of the gem (its
  entries are uses, long rests and one edit, all with the row present). Such an item gets its
  powers back from a new data migration.
- Not covered: weight and encumbrance, item costs, magic item identification, an item that raises
  Strength (a power has no Strength effect, step 9), triggers other than `min_str`, the Power Gem's
  other forms and the thunder hammer's stun.
