# ADR 0016: Inventory and editors

**Status**: accepted, 2026-10-15 (critical-qki.37; collects the rules shipped in critical-qki.13,
critical-qki.31 and critical-qki.32, until then in `docs/backend.md`)

## Context

The Bard carries arrows, flasks, a wand and a Power Gem. User, 2026-10-13: *"these are all
consumables... it's just their type"*. One item model has to cover them all, and the player must be
able to fix the lists (a new quiver, a weapon enchanted by the DM) without a deploy. Every change
goes through the action log, which is a stack ([ADR 0007](0007-hp-ledger.md) step 8).

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
   `TestRestock_RejectsChargesAndDaily`, `TestInventory_SeededBehaviours`,
   `TestInventory_ArrowsAtZeroStayAndRestock`, `TestInventory_RestockRefusedForChargesAndDaily`.
2. **Use and restock are logged.** Spending one of any item (the attack's arrow, the inventory's
   minus) is one count change with an `items:<id>` snapshot; the use that takes a `charges` item to 0
   deletes it in the same entry, and popping that entry puts it back in its list position with one
   charge. Using an item with effect dice rolls them. A restock adds 1 to 999 to a `stock` item as
   one `item_restock` entry.
   Tests: `TestUseItem_RollsEffectDiceAndLogsWithSnapshot`, `TestUseItem_LastChargeRemovesTheItem`,
   `TestRestock_RejectsOutOfRangeCounts`, `TestInventory_WandAtZeroIsRemovedAndPopBringsItBack`,
   `TestUsingTheWandRollsLogsAndVoidRestoresTheCharge`.
3. **Long rest refills daily items only.** Each `daily` item goes back to its daily number; stock and
   charges are untouched. Passing time refills nothing ([ADR 0005](0005-rounds-and-time.md) step 7).
   Tests: `TestResetDaily_ResetsThenLists`, `TestLongRest_RefillsSlotsAndDailyItemsOnly`,
   `TestGemDispelOncePerDayUntilLongRest` (composition).
4. **Item editor.** Items are added, edited (only the fields sent) and removed through the editor;
   each is one entry (`item_add`, `item_edit`, `item_remove`) with the `items:<id>` snapshot.
   Popping the newest entry deletes a created item, writes back every "before" field of an edited
   one, or re-inserts a removed one with its id, position and count. A new item's id is the slug of
   its name (suffix `_2`, `_3` when taken) and it goes last in the list.
   Tests: `TestCreate_InsertsLogsAndSnapshots`, `TestUpdate_ChangesOneFieldAndLogsBeforeAfter`,
   `TestDelete_RemovesAndLogsWholeItem`, `TestRestorers_UndoCreateEditDelete` (itemsvc),
   `TestInventory_AddThenPopDeletesTheItem`, `TestInventory_EditAllFieldsAndPopRestoresThem`,
   `TestInventory_RemoveItemAndPopBringsItBackWithItsCount`.
5. **Item validation.** An invalid item is refused with a message naming the field (400
   `{error, field}`) before anything is stored or logged: a name is required; the behaviour is
   one of the three; the count is 0 to the maximum, and a `charges` item has at least 1 charge
   (removing it is what remove is for); a `daily` item has a daily number of at least 1 and a count
   not above it; the magical bonus is -10 to 10; effect dice are `NdM` or `NdM+K`; the effect save is
   one of the five save categories ([ADR 0006](0006-saving-throws.md)). An edit that changes nothing
   is refused.
   Tests: `TestCreate_ValidationNamesTheField` (itemsvc), `TestUpdate_RejectsBadValuesAndNoOps`,
   `TestParseDice`, `TestInventory_InvalidEditsLogNothing`, `TestInventory_ChargesAtZeroRefused`.
6. **Ammunition.** Ammo is any item whose ammo kind matches the weapon's; the attack offers those
   with a count above 0 ([ADR 0002](0002-attack-resolution.md) step 10).
   Tests: `TestWeapons_AddLongBowFiresArrowItems`, `TestWeapons_AmmoKindEditedToBoltRefusesArrows`.
7. **Weapon editor.** Weapons are added, edited and removed the same way (`weapon_add`,
   `weapon_edit`, `weapon_remove`, snapshot `weapons:<id>`, pop-only). A remove is a hard delete:
   old attack entries keep the weapon's name and id in their payload, and popping the remove
   re-inserts the same id, position and fields. The next attack reads the table, so an edit applies
   at once.
   Tests: `TestCreate_InsertsLogsAndSnapshots`, `TestDelete_RemovesAndLogsWholeWeapon`,
   `TestRestorers_UndoCreateEditDelete` (weaponsvc), `TestWeapons_EditBastardSwordIsUsedByNextAttackAndPopRestores`,
   `TestWeapons_RemoveDartsKeepsOldLogLinesAndPopRestores`, `TestWeapons_AddThenPopDeletesIt`.
8. **Weapon validation.** Damage dice are `NdM` only (the attack rolls NdM; the weapon's magic is
   its own field), for the small/medium and the large column alike; to-hit and damage magic are
   separate, each -10 to 10; the kind is melee or missile and a melee weapon has no ammo kind. Field
   names in the error are snake_case (`damage_sm`, `magic_to_hit`...).
   Tests: `TestCreate_ValidationNamesTheField` (weaponsvc), `TestUpdate_RejectsBadValuesAndNoOps`,
   `TestWeapons_InvalidEditsLogNothing`, `TestWeapons_EditedDamageDiceAreRolled`.

## Consequences

- A `charges` item cannot sit at 0, so "empty wand" is not a state; a stock item can, and so can a
  `daily` one until the long rest.
- Because the log is a stack, fixing an old item edit means popping what came after it.
- Not covered: weight and encumbrance, item costs, magic item identification.
