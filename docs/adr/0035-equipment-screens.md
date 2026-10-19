# ADR 0035: Equipment screens: one flat list, one term, the AC in a shield, initiative without a declaration

**Status**: accepted 2026-10-18 (the user approved the design round after two rounds: "agree"; the
points marked _user_ are his decisions, the points marked _proposed_ are the designer's, accepted with
"agree"). Not implemented. Settles the layout that [ADR 0033](0033-equipment.md) left to the design
round (CRI-32) and amends it: **steps 15 and 16 (initiative) are replaced by item 6 below, steps 18
and 19 (weapon type against armour type) are removed, and the web's wording of steps 6 and 7** is
fixed by item 3. Through step 18 and 19, the amendments ADR 0033 made to
[ADR 0002](0002-attack-resolution.md) (steps 2, 11, 12) and [ADR 0012](0012-attack-against-unknown-ac.md)
(the target's armour) are removed with them; through step 15, the one ADR 0033 made to
[ADR 0013](0013-initiative.md) (step 3 and Consequences) becomes the one of item 6. Follows
[ADR 0022](0022-activity-focus-layout.md), [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md)
and [ADR 0027](0027-space-follows-frequency-of-use.md) (French is the layout reference).

## Context

[ADR 0033](0033-equipment.md) decided the rules and left every screen to a design round. That round
produced the mock-up pages (`web/design/equipment/`), the screenshots
(`docs/design/equipment/shots/`, `just equipment-shots`) and the table of screens in
[docs/design/equipment/README.md](../design/equipment/README.md). This ADR records the decisions; the
README and the shots are the picture, French first. Rules of ADR 0033 not named here stand.

## Decision

### The user's decisions

1. **No target armour on the attack bar** (_user_: "the dm gives (or not) the final AC"; the DM is
   always free to disclose as little as they choose). ADR 0033 **steps 18 and 19 are removed**: the
   attack form has no target armour, the attack has no `armor_type` modifier, nothing is corrected
   after the roll beyond what ADR 0012 and ADR 0002 already allow. The amendments to ADR 0002 steps 2,
   11 and 12 and to ADR 0012 that came with them are removed too (those ADRs are as before ADR 0033
   on this point). The catalogue's weapon-against-armour table (ADR 0033 step 21) stays as reference
   data and is not read by any rule. **CRI-40 is dropped.**
2. **One flat possessions list** (_user_: the partition "adds nothing but confusion"). The Inventaire
   lists every piece in the player's order; the **category chips** are the only filter. There are no
   "on you / in the pack" groups. A piece's state shows as a tag on its row.
3. **One UI term** (_user_: "Use one way to convey this concept"). The web says **Équiper / Retirer**
   (en: Equip / Unequip) for the move and **Équipé** (Equipped) for the state, for worn armour, shield
   and magic pieces and for wielded weapons alike: on rows, in the journal, in the weapon drop-down
   ("Équipées / Non équipées") and in "Équiper tout de suite" when adding. `worn` and `wielded` stay
   the internal states and the wire codes of ADR 0033 step 6; the web does not use them as words.
   (This is the web's wording of ADR 0033 steps 6 and 7; the journal row of a state change reads
   "équipé / retiré".)
4. **The AC is a number in a filled shield beside the HP** (_user_: "the placement is good, not the
   visuals"). On the character card, the AC sits beside the HP: the number alone, no label, inside a
   shield icon with a filled background so that the number pops, in the selected tone with a tinted
   outline, smaller than the HP. An override (ADR 0033 step 13) turns the outline amber; a class
   warning (item 9) adds a warning badge. Clicking it opens the breakdown (item 8).
5. **No initiative declaration** (_user_: "this app should make things easier, not harder"; "yes,
   we're aligned now"). See item 6.

### Initiative (replaces ADR 0033 steps 15 and 16)

6. **d10, the weapon in hand's speed, effects.** Next turn rolls the one d10 (ADR 0013 steps 1, 2 and
   4) and the round's initiative is `d10 + the speed factor of the weapon in hand + initiative
   effects` (step 11 of ADR 0033: haste, slow); the lowest acts first, ties at the same time; the app
   still orders nothing. The weapon in hand's speed is ADR 0033 step 15.1's `weapon` modifier
   (speed factor lowered by its plus, the lesser of to-hit and damage magic, or the "+N" ADR 0002 step
   6 picks when higher, never below 0); with no weapon in hand it adds 0.
   1. **Nothing is declared**: no declaration, no `initiative_declare` amend and no
      `spell_declared_after_roll` warning exist; the situational modifier of ADR 0033 step 15.3 is not
      built either.
   2. **A spell cast or an item used corrects that round's initiative automatically.** When the
      character casts a spell or uses a magic item in a round, that round's initiative is replaced by
      `d10 + the spell's casting time in segments (ADR 0033 step 15.1's rules) or the item's modifier
      (PHB Table 56) + effects`, with the same d10. The correction is logged and undoable, no die is
      rolled, and the journal reads, e.g., "Initiative du tour 2 corrigée : 6 → 4".
   3. **The breakdown** (d10, weapon speed, effects, and the cast or item that corrected it) is the
      tooltip of the initiative number, and the log entry's payload.
   4. Reroll (ADR 0013 step 4) keeps its meaning. Group initiative, multiple attacks, sizes, breath
      weapons and the casting-while-hasted exception stay uncovered (ADR 0033 step 16).

   _As built (CRI-39)_. `buffsvc` owns it. Next turn rolls the d10, then adds the modifiers, each
   a term `{source, value, itemId, name, spellId, magicKind, timing}`, source `weapon` | `effect` |
   `spell` | `item`:
   - **Weapon** (`buffsvc.WeaponSpeed`): the weapon in hand is the attack form's, as the form
     resolves it (`combatsvc.InHand`, read through buffsvc's `Weapons` port, wired in the root).
     Its speed is lowered by max(min(to-hit magic, damage magic), the "weapon becomes +N" that
     applies to its kind), never below 0. A negative magic (a cursed weapon) does not slow it. With
     no weapon in hand there is no term.
   - **Effects**: the `initiative` effect kind (ADR 0033 step 11, built here) on buffs and item
     powers, one term per source. Buffs are read after the advance, so one that expires with it does
     not count. The aggregate's gating applies (MinStr, power triggers), and an effect with a
     `weapon_kind` condition does not count (it conditions an attack). The spell seed's `initiative`
     (Haste −2) now applies instead of being listed as untracked.
   - **Correction** (`CorrectInitiative`): `castsvc.Cast` (a failed cast too, since the casting time
     was spent) and `itemsvc.UseItem` on a `magic` piece. A spell received, an effect added or an
     ammunition or gear use corrects nothing.
     - The spell term (`buffsvc.SpellInitiative`) is the catalogue's parsed casting time at the
       caster level: segments under 10 add their amount (a dice part is not counted). A round or
       more adds 0 with `timing: end_of_round`. A special or unknown casting time adds 0 with
       `timing: special`.
     - The item term is Table 56 by magic kind. The scroll row has no modifier, so a scroll adds 0.
       Before catalogue format 7 (no Table 56) a use corrects nothing.
     - The correction keeps the d10 and the effect terms recorded at the roll. Its cause replaces the
       weapon term, or an earlier cause. Effects are not re-read, so casting Haste does not speed up
       its own round.
     - It is an `initiative_correct` entry `{round, d10, before, after, modifiers, cause}` with the
       `initiative` snapshot, recorded as the last entry of the cast's or the use's transaction. So
       one undo pops both, redo replays it (the snapshot's replay names the entry), and the reroll
       guard (the initiative's entry on top) still lets a reroll follow.
     - Out of combat or with no initiative it does nothing.
   - **Reroll** keeps the modifiers and rolls a new d10.

   Storage: migration 021 adds `round_state.initiative_modifiers` (TEXT, a JSON list of terms,
   nullable and additive; NULL, as every row before it, reads as a plain d10). The `initiative`
   snapshot gains `modifiers` (absent: a plain d10). The `initiative` payload becomes `{round, d10,
   modifiers, total}`; old `{round, d10}` entries read their total as the die.

   Wire: `GET /api/round`, Next turn and the reroll serve `initiative` as `{value (the total, was
   the die), d10, modifiers, endOfRound, logEntryId}`.

   Web: the turn header's number has the breakdown as its tooltip (d10, each term, "= total, the
   lowest acts first"), plus a "fin du tour" tag when a spell takes a round or more. The journal
   words both entries, e.g. "Initiative du tour 2 corrigée : 8 → 4 (Projectile magique : +1)".

   Tests:
   - `buffsvc`: `TestWeaponSpeed_ADR0035Item6_*`, `TestSpellInitiative_ADR0035Item6_2_*`,
     `TestInitiative_ADR0035Item6_NextRoundAddsTheWeaponSpeedAndEffects`,
     `TestInitiative_ADR0035Item6_2_ACastOrAnItemUsedCorrectsTheRound`,
     `TestInitiative_ADR0035Item6_4_RerollKeepsTheModifiers` and
     `TestInitiativeRestorer_ADR0035Item6_RestoresTheModifiers`.
   - `castsvc`: `TestCast_ADR0035Item6_2_CastingCorrectsTheInitiative`.
   - `itemsvc`: `TestUseItem_ADR0035Item6_2_AMagicItemUsedCorrectsTheInitiative`.
   - `composition`: `TestInitiative_ADR0035Item6_WeaponSpeedThenAnItemUsedCorrectsTheRound` (undo,
     redo, reroll).
   - `router`: `TestRoundInitiative_WireShape`.
   - e2e: `round.spec` and `layout.spec`.

### The screens (proposed, accepted with "agree")

7. **Inventaire.** The possessions as the stage card; on the right the AC card
   (`clamp(300px, 22vw, 420px)`, [ADR 0027](0027-space-follows-frequency-of-use.md) step 5) with the
   breakdown rows and **Forcer** (the override, ADR 0033 step 13). The review of the conversion (ADR
   0033 step 27) is an amber band "N possessions à vérifier" with **Vérifier**, which opens a card
   above the list: the armour guess confirmed by chips, and per piece **Choisir / Modifier /
   Ignorer**.
8. **The AC breakdown** is a popover from the shield on the character card and the AC card of the
   Inventaire, the same rows. An override shows the derived AC beside it, with **Effacer**.
9. **Class warnings** (ADR 0033 step 20) show in amber on the row, on the AC card and on the AC
   shield, as words ("Pas pour un barde"); they never refuse.
10. **Ajouter** opens in the stage, with the tabs **Catalogue** and **Saisir**. The catalogue tab has
    the category, a search, the PHB rows with their class warnings as tags on the row, and **Votre
    exemplaire** (name, "+N", **Équiper tout de suite**); the typed tab is the homebrew form.
11. **Editors.** Armour and shield editors have a "+N" field (ADR 0033 step 17); the magic piece
    editor has the powers list with a **Modèles** menu.
12. **Act > Armes.** The weapon drop-down lists the equipped weapons, then the unequipped ones;
    picking an unequipped weapon equips it (one logged move, ADR 0033 step 7). The attack bar is
    otherwise unchanged (item 1).
13. **Skills tab.** It shows the worn armour's line in place of the drop-down (ADR 0033 step 14).

## Migration path

ADR 0033's slices stand, with these changes to their content:

- **CRI-39**: initiative as item 6 (no declaration, no situational modifier; the correction on a cast
  or an item used), not ADR 0033 steps 15 and 16 as written.
- **CRI-40**: dropped (ADR 0033 steps 18 and 19 removed).
- **CRI-34 to CRI-38, CRI-41**: build the screens of items 2 to 4 and 7 to 13 with their rules, and
  the one term of item 3.

Tests cite `ADR 0035 item N` where they build a decision here.

## Consequences

- Fewer inputs on the attack bar and none before Next turn: the player does one thing per action.
- The weapon-against-armour optional rule (DMG Table 36) is not in the app; the data is kept in the
  catalogue (format 7) in case it returns.
- Initiative changes during a round when the character casts or uses an item; the journal and the
  tooltip say why.
- Equipped is the only state word the player sees; the wire keeps `worn` / `wielded`.
