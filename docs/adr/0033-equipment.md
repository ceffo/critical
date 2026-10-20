# ADR 0033: Equipment: one possessions list, worn / wielded / carried, the derived AC

**Status**: accepted, 2026-10-18 (CRI-31, research for the epic CRI-29; decisions: the user's triage,
2026-10-18, and the supervisor's acceptance of every point proposed in the research, 2026-10-18). Amends
[ADR 0002](0002-attack-resolution.md) (steps 2, 11 and 12: the armour-type modifier),
[ADR 0003](0003-damage.md) (step 1: a launcher's dice come from its ammunition),
[ADR 0004](0004-buffs.md) (step 11: the `ac` sign, the `base_ac` and `initiative` effects; step
18: the `without_magic_armour` condition), [ADR 0008](0008-spell-slots-and-long-rest.md) (step 10:
a spell spends a piece by its component code, step 1.1), [ADR 0012](0012-attack-against-unknown-ac.md) (the
target's armour is optional like its AC), [ADR 0013](0013-initiative.md) (step 3 and Consequences:
weapon speed and casting time), [ADR 0016](0016-inventory.md) (steps 1, 4, 7, 8, 9 and 15),
[ADR 0021](0021-spell-catalogue.md) (step 13 added: format 7),
[ADR 0023](0023-localized-text.md) (steps 9 to 12: one equipment kind) and
[ADR 0030](0030-classes-and-races.md) (steps 1, 17, 18 and 23, and its Consequences on class
armour and weapon limits).

_Amended by [ADR 0035](0035-equipment-screens.md) (the design round, accepted 2026-10-18), on implementation: steps 15 and 16 (initiative: no declaration, the weapon in hand's speed, a cast or item used corrects the round's initiative) and steps 18 and 19 (removed: no target armour) are replaced; the web says Equip / Equipped for steps 6 and 7._

## Context

User, CRI-29: _"The equipment "type" is deeply flawed. We need to research into phb and dmg to
extract the right domain topology for this."_ Today:

- Items ([ADR 0016](0016-inventory.md)) are one model with a free-text type label (arrow, flask,
  wand, gem); only the count behaviour (`stock`, `charges`, `daily`) means anything.
- Weapons are a separate model, table and card. Every stat is typed by hand. The weapon type code of
  ADR 0030 step 23 (13 codes) is read by the class rules.
- The character has no AC. The `ac` effects of buffs and item powers add up to nothing; the only AC in
  the app is the target's ([ADR 0002](0002-attack-resolution.md), [ADR 0012](0012-attack-against-unknown-ac.md)).
- A thief's skill armour is an entered drop-down (CRI-24, #314). User: _"completely the wrong way
  to address this"_.
- Initiative is a plain d10 ([ADR 0013](0013-initiative.md) step 3, "speed will come later").
- The catalogue carries rule tables but no equipment.

### What the books say (restated, cited; no book text)

- **PHB Table 44, Weapons (p.68-69)**: each weapon row has a size (S, M, L), a damage type (B, P or
  S; a few rows carry two, P/B or P/S), a speed factor and damage against small-medium (S-M) and
  large (L) targets. Bows and crossbows have a speed but **no damage**: the arrows, quarrels, bullets
  and blowgun darts are their own rows with the damage and the type. A few weapons have one row per
  grip (bastard sword one- and two-handed). PHB p.73 describes the size, type and speed columns.
- **PHB Table 44, Armor (p.69) and Table 46 (p.75)**: each body armour gives a base AC (none 10,
  leather or padded 8 … full plate 1); a shield betters it by 1. Helmets come with the suit ("Armor Sizes", PHB p.76).
  PHB p.75-76 describes the shields: a buckler, a small and a medium shield each better the AC by 1
  against a limited number of attacks a round (1, 2, all frontal); a body shield by 1 against melee
  and 2 against missiles.
- **DMG p.181**: each +1 of magic armour betters the AC by 1, whatever the armour; a magical shield
  adds its plus on top of the shield's own point; no AC better than -10. Magic does not change the
  armour's type (PHB p.90).
- **DMG p.148-149 (ring of protection)**: betters AC and saves by its plus, stacks with other
  protection items, but gives no AC with magic armour worn, and two rings do not add (the best one
  counts). **Bracers of defense (DMG p.162)**: an AC as if armoured, no effect when armour is worn,
  stack with other protection items. **Cloak of protection (DMG p.164)**: betters AC and saves by its
  plus.
- **PHB Armor spell (wizard 1)**: an AC 6 field (as scale mail) on an unarmoured target; it adds
  Dexterity and a shield, not the Shield spell. **PHB Shield spell (wizard 1)**: a different AC by
  attack form (hurled missiles, device missiles, other attacks).
- **PHB Table 29 (p.39)**: the thief's skill adjustments have four columns: no armour, leather (the
  baseline), elven chain, padded or studded leather; a bard in non-elven chain takes 5% more. PHB
  p.39: a thief with bracers of defense or a cloak and no armour uses the no-armour column.
- **PHB p.89-90, Table 52 (DMG Table 36)**: optional rule. Each armour type has a modifier per
  damage type (e.g. plate +3 against slashing, leather -2 against piercing), applied to the
  attacker's THAC0; padded and hide count as leather, bronze chain as chain. A weapon with two types
  uses the one that favours the attacker. It needs the target's **armour type** as well as its AC, and
  is not used against a natural AC.
- **PHB p.94-96 (combat sequence)**: the DM decides the monsters' actions, the players state theirs
  (spells named; a spell cannot change once the die is rolled), initiative is rolled (1d10, lowest
  first, ties simultaneous), then actions resolve in order. Table 55 lists standard modifiers
  (hasted -2, slowed +2, higher ground -1, …). Table 56 (optional) adds the weapon's speed factor,
  the spell's casting time, or a magic item's use (potion +4, ring +3, rod +1, staff +2, wand +3,
  miscellaneous +3, scroll: the spell's casting time). A casting time "in rounds" is not a modifier:
  the spell takes effect at the end of the round (or later). PHB p.96: each plus of a magical weapon
  lowers its speed factor by 1, the lesser plus when it has two, never below 0.
- **Class limits (PHB chapter 3)**: warriors any armour and weapon; wizards no armour, and only
  dagger, staff, dart, knife and sling (Wizard); clerics any armour and shield, only bludgeoning
  weapons (Cleric); thieves only leather, padded, studded leather or elven chain, no shield named,
  and the weapons club, dagger, dart, hand crossbow, knife, lasso, short bow, sling, broad sword,
  long sword, short sword, staff (PHB p.39); bards any armour up to chain mail, no shield, any weapon
  (Bard).

## Decision

Tests are named by the slice that builds each step (the citation `ADR 0033 step N` in the test's
name or comment); this ADR names none yet.

### The topology

1. **One possessions list.** A character's equipment is one list. Every piece has, whatever its
   category: an id (kept for life, step 26), a position, a name (a language map, [ADR
   0023](0023-localized-text.md)), a **category** (step 2), an optional **catalogue code** (the
   catalogue row it was picked from, step 21), a count and its **behaviour** (`stock`, `charges`,
   `daily`, [ADR 0016](0016-inventory.md) steps 1 to 3, unchanged), a note, its **powers** (ADR 0016
   steps 9 to 16), its **state** (step 6), and the fields of its category (steps 3 to 5). Weapons are
   pieces of equipment: there is no second model, table or editor. The free-text type label is gone;
   its meaning is the category.
   1.1. **Spell component** (amendment, 2026-10-18, the supervisor's decision on the review of the
   slice: the label the spells matched could no longer be set). A piece of any category may carry a
   **component** code: what a spell that spends an item (`consume_item`, [ADR
   0008](0008-spell-slots-and-long-rest.md) step 10) names, so a spell spends a piece of that code
   whatever its category, never by its name or a label. The codes are one closed list owned by the
   service (`gem`, the Chromatic Orb's), and every spell's `consume_item` names one of them; the web
   names them (ADR 0023). The editor sets it (none by default); the conversion copies a type label
   that names a component (any case, singular or plural) into it (step 26.5). Matching by category or
   magic kind is not used: a plain gem is `gear`, and `magic` `misc` holds bracers and cloaks too.

   ```mermaid
   classDiagram
     class Piece {
       id, position, name (language map)
       category, catalogue code?
       count, behaviour, note
       powers[], state
     }
     Piece <|-- Weapon
     Piece <|-- Ammunition
     Piece <|-- Armour
     Piece <|-- Shield
     Piece <|-- MagicItem
     Piece <|-- Gear
     class Weapon { weapon type, kind, size, damage types, speed, damage S-M / L, magic to hit / damage, two-handed, ammo kind fired, specialized }
     class Ammunition { ammo kind, damage types, damage S-M / L, magic }
     class Armour { base AC, Table 29 column, Table 52 row }
     class Shield { shield kind, AC bonus melee / missile }
     class MagicItem { magic kind }
   ```

2. **Categories.** A piece's category is a code: `weapon`, `ammunition`, `armour` (body armour),
   `shield`, `magic` (a magic item that is not a weapon, ammunition, armour or shield) and `gear`
   (everything else: rope, flasks, rations, tools). A `magic` piece has a **magic kind**: `potion`,
   `scroll`, `ring`, `rod`, `staff`, `wand` or `misc` (the DMG's categories; `misc` covers bracers,
   cloaks, gems, boots…). Magic weapons, ammunition, armour and shields keep their category and carry
   their magic as in steps 3 to 5. A piece the conversion cannot place is `gear` and on the review
   list (step 27). The codes go on the wire; the web names them (ADR 0023).
   A `scroll` may **name the spell it holds** (`spellId`, a spell catalogue id; one spell per scroll,
   a scroll of several spells is several pieces), set and cleared in the editor like any field (an
   `item_edit` change, undone by its snapshot). No other piece holds a spell (400 on `spellId`,
   `not_allowed`), and a spell the catalogue doesn't know is refused (`unknown`); a piece that stops
   being a scroll drops it. Step 15.1's "a scroll as the spell it holds" reads it (ADR 0035 item 6.2).
   *Added (CRI-73).*
3. **A weapon** carries: the **weapon type** (ADR 0030 step 23's codes, unchanged; the class rules
   keep reading it), the **kind** (`melee` or `missile`, ADR 0016 step 8; a hurled weapon is
   `missile`), the **size** (`s`, `m`, `l`, informative: armour and weapon sizes against the
   character's size are not covered), the **damage types** (one or two of `b`, `p`, `s`), the **speed
   factor** (0 to 20), the damage S-M and L (`NdM`, ADR 0016 step 8; empty for a launcher, step 4),
   its magic to hit and to damage (ADR 0002 step 6, unchanged), two-handed, the **ammo kind it
   fires** (a launcher) and the fighter's specialization flag (ADR 0030 step 12; since
   [ADR 0037](0037-proficiencies.md) step 9 derived from the character's specialization pick of
   the weapon type, read-only, never stored or edited). A weapon with one
   catalogue row per grip (bastard sword) is one piece with one grip; changing the grip is an edit.
4. **Ammunition** carries its ammo kind (`arrow`, `quarrel`, `bullet`, `needle`, `blowgun_dart`;
   the hand, light and heavy quarrels share `quarrel` and differ by their dice), its damage types, its
   damage S-M and L, and its magic (ADR 0002 step 7). A launcher's attack rolls the
   ammunition's dice; a launcher that has dice of its own (every weapon today, and homebrew) uses them
   when the ammunition has none. Amends ADR 0003 step 1.
5. **Armour and shields.** A body armour carries its **base AC** (Table 46 without the shield: 10 to
   1), its **Table 29 column** (`none`, `leather`, `padded_studded`, `elven_chain`, `metal` for a
   bard's non-elven chain up to chain mail, ADR 0030 step 1, or `none_allowed` for an armour a thief
   or bard may not wear) and its **Table 52 row** (`banded`, `brigandine`, `chain`, `field_plate`,
   `full_plate`, `leather`, `plate`, `ring`, `scale`, `splint`, `studded_leather`). A shield carries
   its kind (`buckler`, `small`, `medium`, `body`) and its AC bonus against melee and against missiles
   (1 and 1; a body shield 1 and 2). Their magic "+N" is a power (step 17). Every one of these numbers
   and codes is prefilled from the catalogue and editable (the DM's special armour); the rules read
   the piece, never the catalogue.

### Worn, wielded, carried

6. **Three states.** Every piece is `carried` (the default), `worn` or `wielded`.
   - `worn`: body armour, a shield, a `magic` piece (a ring on a finger, a cloak, bracers, the
     Power Gem set in Kelsen's forehead). At most **one body armour** and **one shield** are worn.
   - `wielded` (readied): weapons. The hands are not counted, but a two-handed weapon
     wielded with a worn shield is a warning (step 20), never a refusal.
   - `gear`, `ammunition` and the other `magic` kinds (potion, scroll, wand, rod, staff) can be
     `carried` only; a staff or rod used as a weapon is a `weapon` piece.
   A state the category does not allow, or a second body armour or shield, is refused (400 naming the
   field `state`, code `not_allowed` or `slot_taken`). Wearing a second armour means taking the first
   off: the web offers the swap as one move.
7. **A state change is a logged move.** Wearing, taking off, wielding and putting away are one
   `equipment_state` entry each (payload `{id, before, after}`, the piece's snapshot), in one
   transaction ([ADR 0018](0018-transactions-and-undo.md)), undone and redone like every move
   ([ADR 0010](0010-action-stack-and-undo.md), [ADR 0019](0019-redo.md)). A swap is one entry with
   both pieces. The same state again logs nothing.
8. **The attack form reads the wielded weapons.** The attack form's weapon in hand
   (ADR 0002 step 12) is chosen among the wielded weapons. Picking a carried weapon there wields it
   (one `equipment_state` entry) before the form changes; putting away the weapon in hand empties
   the form's weapon.
9. **Powers follow the state.** A piece's powers (ADR 0016 steps 9 to 14) are on only
   while it is `worn` or `wielded`; a carried ring protects nothing. Amends ADR 0016 step 9 ("as long
   as the item is carried"). Its daily use and its count (ADR 0016 steps 1 to 3) do not depend on
   the state. The conversion makes every piece that has powers `worn` (step 26), so the Power Gem
   keeps working.

### The character's AC

10. **Derived, never entered.** The character's AC is computed on every read (like ADR 0030's
    derived block), from these terms, in this order; each is one row of the breakdown with its source
    (the piece's id, as ADR 0016 step 14, or the buff's name):
    1. **Base**: the worn body armour's base AC; with none worn, the best (lowest) `base_ac` effect
       that applies (step 11), else 10.
    2. **Armour magic**: the worn armour's `ac` powers (step 17).
    3. **Shield**: minus the worn shield's AC bonus, then its `ac` powers.
    4. **Dexterity**: PHB Table 2's defensive adjustment for the sheet's Dexterity (the catalogue's
       `dexterity.defensive`, ADR 0021 step 9).
    5. **Protection**: the `ac` effects of the other worn pieces' powers, in list order (step 12).
    6. **Buffs**: the `ac` effects of the buffs that apply, in list order ([ADR 0004](0004-buffs.md)
       steps 10 and 11).
    7. **Floor**: never better than -10 (DMG p.181).
    The sum is the AC. With a body shield worn, the AC against missiles is served too (its missile
    bonus instead of its melee one); otherwise the two are equal and the web shows one.
11. **AC sign and the two new effects.** An `ac` effect is a change of the AC number (`-2` betters
    it by 2), as the spell effects seed already writes it; amends ADR 0004 step 11 to say so. Two
    effect kinds join ADR 0004 step 11, for buffs and powers alike:
    - `base_ac` N: "an AC of N as if armoured" (the Armor spell, AC 6; bracers of defense, AC N). It
      replaces the base 10 only while no body armour is worn; the best one counts; Dexterity, the
      shield and the other terms still add (PHB Armor spell; DMG bracers).
    - `initiative` N: added to the modified initiative (step 15; haste -2, slow +2).
12. **Protection does not stack where the DMG says so.** An effect may carry the condition
    `without_magic_armour` (next to ADR 0004 step 18's `weapon_kind`): it counts only while the worn
    body armour has no `ac` power. A ring of protection's `ac` effect carries it (its `save_bonus`
    does not: DMG p.148). Two worn pieces of the same catalogue code with an `ac` effect:
    only the best counts (DMG p.149's two rings), the other is shown as not adding.
13. **The override, for the one case the model cannot hold.** The Armor spell and
    bracers are covered by `base_ac` (step 11), so they need no override. The case the model cannot
    hold is **an AC that depends on the attack form beyond melee / missile**: the **Shield spell**
    (wizard 1: a different AC against hurled missiles, against device missiles and against the rest,
    and none of them is a sum). The player then sets an **AC override** (a number -10 to 10, with an
    optional label), one logged move (`ac_override`, before and after), cleared the same way. While
    it is set the AC shown is the override, marked as such, and the derived AC and its breakdown stay
    visible beside it. Nothing else reads the override (no rule in the app reads the character's AC
    today). (Not chosen: no `base_ac` effect, with the Armor spell as the override's case, triage's
    example.)

### Thief and bard skill armour

14. **The skill armour is the worn armour.** ADR 0030 steps 17 and 18's armour
    term reads the worn body armour's Table 29 column; with none worn, the `none` column (PHB p.39,
    bracers and cloaks included). The `skillArmor` entry (`PATCH /api/characters/{id}`, the creation
    field and the Skills tab's drop-down, CRI-24) is removed. An armour whose column is
    `none_allowed`, or `metal` for a thief, uses the `padded_studded` column (the worst one) and
    shows the class warning (step 20) (not chosen: the PHB chapter 3 multi-class reading, a
    multi-classed thief in armour a thief may not wear having every skill but Open Locks and Detect
    Noise unusable). A shield changes no skill (Table 29 has no
    column for it).
    **Carry-over** (in the conversion, step 26): a thief or bard with no worn body armour gets one,
    worn, from its stored `skill_armor`: `leather` and an empty value (which read as leather) give
    Leather armour; `padded_studded_leather` gives Studded leather; `elven_chain` gives Elven chain
    mail; a bard's `metal` gives Chain mail; `none` gives nothing. The piece is named in the player's
    language and its stats come from the catalogue; padded-or-studded and metal pieces are on the
    review list ("which armour?"). So every skill percentage is the same before and after. The
    `characters.skill_armor` column stays until a later cleanup migration; an old `skill_armor` log
    entry still pops and still shows in the journal (it writes the column back, which nothing reads
    any more).

### Initiative

15. **Declare, roll, modify.** Amends ADR 0013 step 3 ("no modifier"). The round's
    initiative is `d10 + the declared action's modifier + initiative effects + a situational
    modifier`; the lowest acts first, ties at the same time (PHB p.94); the app still orders nothing.
    1. **Declaration.** The player declares the next round's action at any time during the current
       one: `weapon` (the weapon in hand: its speed factor, lowered by its plus, the lesser of its
       to-hit and damage magic, or the "+N" ADR 0002 step 6 picks for it when higher, never below
       0), `spell` (a spell from the character's spells: its casting time in segments from the
       catalogue's parsed stats; a casting time of a round or more adds nothing and the initiative
       reads "end of round"), `item` (a magic piece by its magic kind, PHB Table 56: potion +4, ring
       +3, rod +1, staff +2, wand +3, misc +3; a scroll as the spell it holds), or `other` (0). It
       defaults to the previous round's declaration, at round 1 to `weapon`.
    2. **Roll.** Next Round rolls the one d10 as today (ADR 0013 steps 1 and 2) and the entry records
       the declaration and each modifier (`{round, d10, declaration, modifiers, total}`), so the
       journal and the turn header show the breakdown.
    3. **Situational.** One signed modifier with an optional label, entered at the table (PHB Table
       55: higher ground -1, wading +2, …), as the attack's (ADR 0002 step 9). Haste and slow are
       buffs with an `initiative` effect (step 11).
    4. **Changing the declaration after the roll** is a logged amend of the round's initiative
       (`initiative_declare`, before and after; no die rolled), undoable. Changing to or from a spell
       after the roll is allowed with a warning (`spell_declared_after_roll`, PHB p.94).
    5. Reroll (ADR 0013 step 4) keeps the declaration and the situational modifier.
16. **What initiative does not cover.** Group initiative (one die for the side: the player rerolls to
    the table's die, ADR 0013 step 4), the staggering of multiple attacks (PHB p.96), creature sizes,
    breath weapons and innate abilities (monsters' rows), and the casting-while-hasted exception of
    the PHB example (the player dismisses or keeps the effect).

### Magic bonuses from the editor

17. **The editor writes powers.** Amends ADR 0016 step 15 ("powers come from the seed and from data
    migrations only"). The equipment editor shows a piece's powers and lets the player
    add, edit and remove them: each power is a trigger (none or `min_str`) and effects from the list
    of ADR 0016 step 9 plus `base_ac` and `initiative` (step 11), with their conditions (`weapon_kind`,
    `without_magic_armour`). Validation is the seed's (ADR 0016 step 9: no Strength effect). A power
    change is part of the piece's `item_edit`, so its snapshot already carries the powers and undo
    restores them (ADR 0016 step 15's snapshot rule unchanged).
    For armour and shields the editor has a **"+N" field** that is sugar for one power with the
    reserved id `magic` and one effect `ac: -N` (no trigger); the AC breakdown names it "armour
    magic" / "shield magic" (step 10). A weapon's and ammunition's magic stay their own fields (ADR
    0002 steps 6 and 7). Ready-made rows: the editor offers templates for the common
    protection items (ring of protection +N: `ac -N` without magic armour and `save_bonus +N`; cloak
    of protection +N: `ac -N`, `save_bonus +N`; bracers of defense AC N: `base_ac N`), as web
    constants of effect codes, not catalogue data.

### Weapon type against armour type

18. **The target's armour is given like its AC.** Amends ADR 0012 and ADR 0002 steps 11 and 12.
    The attack form gets a **target armour**: a Table 52 row code (step 5), `natural`,
    or unknown (the default, as the target AC, ADR 0012 step 1). It is sticky on the form like the
    target AC and can be corrected after the roll on the latest attack, like the target AC (ADR 0002
    step 11): an audited amend, hit and crit recomputed from the stored dice, its own undoable step.
19. **The modifier.** With a known armour row (not `natural`, not unknown), the attack gets one
    to-hit modifier `armor_type`: minus the Table 52 value (catalogue `weapon_vs_armor`) for the
    attack's damage type against that row. (The reading of PHB p.90 "applied to the
    attacker's THAC0": plate's +3 against slashing makes the attack 3 harder.) The damage type is the
    ammunition's for a launcher, else the weapon's; with two types, the one that favours the attacker
    (PHB p.89). A weapon or ammunition with no damage type gets none. The modifier is in the preview,
    the breakdown and the log entry like every to-hit modifier, and in `hitsAc` / `critsFromAc` (ADR
    0012 steps 2 and 3). Magic armour does not change the row (PHB p.90).

### Class restrictions

20. **Warn, never refuse.** Computed on read, served as codes with the piece's id,
    shown on the piece and on the AC or attack card; nothing is refused or changed:
    - wizard (mage): any worn armour or shield (`armour_not_allowed`); a wielded weapon whose type is
      not dagger, dart or sling, or whose catalogue code is not knife or quarterstaff
      (`weapon_not_allowed`);
    - cleric: a wielded weapon with no `b` damage type (`weapon_not_allowed`) (a P/B
      weapon such as the morning star is allowed);
    - thief: worn armour whose Table 29 column is not `none`, `leather`, `padded_studded` or
      `elven_chain`; any worn shield; a wielded weapon outside PHB p.39's list (by catalogue code or
      weapon type);
    - bard: worn armour better than chain mail (catalogue row other than those of the `leather`,
      `padded_studded`, `elven_chain` and `metal` columns); any worn shield;
    - fighter: none;
    - everyone: a two-handed weapon wielded with a worn shield (`hands`).
    A homebrew weapon without a catalogue code is judged by its weapon type and damage types; when
    those do not settle it (`other_melee` for a thief), no warning. Amends ADR 0030's Consequences
    ("class armour limits (not enforced)", "the cleric's blunt-weapon restriction"): still not
    enforced, now warned.
    As built: a launcher is judged by the ammunition it fires (PHB p.33's "only blunt, bludgeoning
    weapons", with step 4: a launcher deals its ammunition's damage type): the cleric's sling (bullets
    are `b`) is not warned, its bow (arrows pierce) is. Every answer that serves pieces carries
    their warnings, computed once per request from the full list (`hands` needs the other pieces).
    Wire: `GET /api/items`, the item create, edit and state answers and the long rest's items serve
    `warnings` (codes) on each piece, `GET /api/ac` serves `warnings`
    (`{itemId, code}`) for the worn armour and shield; the web words them ("Pas pour un barde").
    `hands` is on both the weapon and the shield. The lists are the table in `itemsvc/classwarn.go`.
    Tests: `TestClassWarnings_Step20_Wizard`, `_Cleric`, `_ClericLauncherJudgedByItsAmmunition`, `_ThiefShieldAndQuarterstaff`, `_ArmourByColumn`, `_Thief`, `_Bard`, `_FighterNone`, `_HandsEveryone`
    (itemsvc), `TestClassWarnings_Step20_ServedOnReadNeverRefused` (composition), `TestAC_GetWireShape`
    and `TestItems_ListWireShape` (router), e2e `layout.spec` ("class warnings").

### Catalogue content

21. **Format 7: equipment reference data.** Adds step 13 to ADR 0021. Format 7 is
    format 6 plus a top-level `equipment` object and two `rule_tables` keys, numbers and codes only,
    names as language maps (ADR 0021 step 1, ADR 0023):
    - `equipment.weapons`: PHB Table 44's weapon rows (polearms included, the arquebus excluded:
      "only if allowed by the DM"): `code`, `name`, `weapon_type` (ADR 0030 step 23), `kind`,
      `size`, `damage_types`, `speed`, `damage_sm`, `damage_l` (empty for launchers), `two_handed`
      (for a medium character, PHB p.73), `fires` (a launcher's ammo kind);
    - `equipment.ammunition`: the ammunition rows: `code`, `name`, `ammo_kind`, `damage_types`,
      `damage_sm`, `damage_l`;
    - `equipment.armour`: PHB Table 44's armour rows with elven chain mail (DMG p.181): `code`,
      `name`, `base_ac` (Table 46), `skill_column` (Table 29), `armor_row` (Table 52);
    - `equipment.shields`: `code`, `name`, `kind`, `ac_melee`, `ac_missile`;
    - `rule_tables.weapon_vs_armor` (Table 52: one row per armour row, one integer per damage type);
    - `rule_tables.initiative_item` (Table 56's magic item rows: magic kind and modifier).
    The rates of fire stay Table 45's rule table (ADR 0030 step 23). Rollout as ADR 0021 step 12.4:
    the API reads formats 6 and 7, the fixture becomes format 7 with invented names and the real
    numbers, the builder's change is a `critical-rulebooks` item.
    _As built (CRI-33, [ADR 0021](0021-spell-catalogue.md) step 13)_: `fires` is a list of ammo kinds
    (the blowgun fires the barbed dart and the needle); `damage_sm` and `damage_l` are strings as printed
    (`1d4+1`, `1d3-1`, `1`), absent for a launcher, so the picking slice reduces them to ADR 0016's `NdM`
    where the model needs it; a weapon with no damage type (mancatcher, whip, scourge) has an empty
    `damage_types`. Readings the books leave open, fixed in the builder: elven chain mail is AC 5 with
    Table 52's `chain` row (worn as chain mail, DMG); bronze plate mail has no Table 52 row and reads as
    `plate`, an approximation (at AC 4 with the plate row's +3 against slashing it is better against
    slashing than field plate, AC 2); hide counts as `leather` for Table 52 and as `metal` for Table 29 (a bard may wear it up to
    chain mail, a thief may not); padded armour is the `padded_studded` column and the `leather` row; the
    scroll's `initiative_item` row has no modifier. The French names come from the *Manuel des Joueurs*
    where its OCR reads cleanly; a name with no French falls back to English.
22. **What is not catalogue data.** The DMG magic items (hundreds of rows whose rules
    are text: the app models their numbers as powers, step 17), PHB miscellaneous gear, clothing,
    provisions, tack, animals and services (no rule reads them; cost and weight are out of scope),
    helmets (part of the suit, PHB p.76). The categories and magic kinds (step 2) are codes in the
    code, not catalogue rows.
23. **Possessions copy, never point.** A piece picked from the catalogue copies the row's numbers
    and codes and keeps its `code` as a reference (for the class warnings and the review list); the
    name is the row's language map, which the player may rename (ADR 0023 step 11). The rules read the
    piece. So the catalogue import never needs to keep a row a character uses (unlike spells, ADR
    0021 step 5.4.3), and a homebrew piece (no code) works the same.

### Adding a piece

24. **Pick or type.** The editor adds a weapon, ammunition, armour or shield by **picking a
    catalogue row** (prefilled, then renamed, made magical or changed) or by typing every field
    (homebrew). Magic items and gear are typed: a name, a category and magic kind, a count and
    behaviour, a note, powers. A new piece is `carried` (step 6); the web may offer "wear it now" as a
    second move.
25. **Validation.** As ADR 0016 steps 5 and 8 for the shared fields and the weapon's; for the rest:
    the category and magic kind are known codes; a speed is 0 to 20; damage types are one or two of
    `b`, `p`, `s`; a base AC is -10 to 10 (a negative base for the DM's special armour);
    the Table 29 column and Table 52 row are known codes; a shield's bonuses are 0 to 5; an unknown
    catalogue code is refused (`unknown`). Field names and codes as ADR 0030 step 1 (`{error, field,
    code}`).

### Conversion of the existing data

26. **Automatic, ids kept.** The slice that brings the one model (CRI-34) converts every
    character's items and weapons in a data step (migrations hold no data, [ADR
    0025](0025-postgresql-storage.md) step 2.1.1), additive first (ADR 0024 step 5.2):
    1. Every item and weapon keeps its **id**, position order (weapons after the items, in their
       order) and every stored field; its name rows stay.
    2. Old entries pop and redo: the `items:<id>` and `weapons:<id>` snapshots and the
       `item_*` / `weapon_*` entries are restored into the one model by the same id. Attack payloads
       keep their weapon and ammunition ids and names (ADR 0016 steps 7 and 14).
    3. An item and a weapon of the same character with the same id: the weapon keeps it, the item
       gets the next free suffix, and its old snapshots are mapped to the new id by a conversion
       table the restorer reads (the developer checks the live data: none is expected). _Since the
       action log codec ([ADR 0018](0018-transactions-and-undo.md) step 9)_: the table is read once,
       by the data step that re-encodes the snapshots, which writes the new id into their envelope.
    4. **Weapons** are matched to a catalogue row by their **weapon type** (ADR 0030 step 23):
       `long_sword` → long sword, `short_bow` → short bow, `dagger`, `dart`, `sling`, `short_sword`,
       `long_bow`, the three crossbows. The match adds the code, the size, the damage types and the
       speed; it never overwrites a stored stat. `other_melee`, `other_thrown`, `other_missile` and
       no type are not matched by type. _Amended in implementation (CRI-34, round 6)_: such a weapon
       (and one whose type has no row) is then matched by **name**: among the catalogue rows of its kind,
       the one whose code is its id, or whose code or name (any language, any case, a trailing magic
       suffix `+2` / `+1/+2` dropped from its own name) is its name; failing that the one whose name is
       its name followed by a qualifier (`Bastard sword, two-handed`) and whose two-handed flag is the
       weapon's. Exactly one row matches or none: an ambiguous or missing name stays unmatched and goes
       on the review list (nothing is guessed). The weapon keeps its own name and bonus.
       The conversion runs once, so the server **refuses** it when the loaded catalogue has no equipment
       section (format 6 and before) and a character holds items or weapons: the start fails with an
       error naming `just catalogue`, the run rolls back and the next start retries it.
    5. **Items** are categorised from their type label when it is obvious: `arrow`, `quarrel`,
       `bolt`, `bullet` → ammunition (its ammo kind); `potion`, `scroll`, `ring`, `rod`, `staff`,
       `wand` → magic of that kind; `gem` → magic `misc` (and the component `gem`, step 1.1); `flask` →
       gear. Ammunition of a kind a
       launcher fires and that has no dice gets the launcher's dice (step 4), so the attack rolls the
       same. Everything else becomes gear on the review list.
    6. **States**: every weapon is `wielded` (all are usable today), every piece with powers is
       `worn` (step 9), the rest `carried`; plus the skill armour of step 14.
27. **The review list.** The app shows the player the pieces the conversion could not settle
    (unmatched weapons, uncategorised items, the armour guessed from `skill_armor`), each with its
    reason as a code. Editing the piece (a category, a catalogue row, a confirmed armour) or
    dismissing the row takes it off; the list is per character and stored, and dismissing is not a
    game move (not logged).

## Migration path (the slices)

The slices that follow this ADR, in order; each amends the ADRs it touches, as built:

1. **CRI-33** (built): catalogue format 7 (steps 21 to 23): schema, importer, fixture; the builder in
   `critical-rulebooks`.
2. **CRI-34**: one equipment model, categories, the three states, the conversion and the review
   list (steps 1 to 9, 26, 27).
3. **CRI-35**: the derived AC, its breakdown, `base_ac`, `without_magic_armour` and the override
   (steps 10 to 13).
4. **CRI-36**: the skill armour from the worn armour, the drop-down removed, the carry-over (step 14).
5. **CRI-37**: pick from the catalogue; the launcher's dice from the ammunition (steps 4, 24, 25).
6. **CRI-38**: powers and the "+N" from the editor (step 17).
7. **CRI-39**: initiative with the declaration (steps 15, 16).
8. **CRI-40**: weapon type against armour type (steps 18, 19).
9. **CRI-41**: class warnings (step 20).

_As built (CRI-34, slice 2)_: the one model is the `items` table (migration 016, additive: the
category, magic kind, catalogue code, state and every category's columns; `weapons` is no longer read
and is dropped later), served and edited at `/api/items` (`POST /api/items/{id}/state {state, swap}`
for step 7, which answers the pieces changed; `DELETE /api/items/{id}/review` dismisses, step 27;
`GET /api/items` carries each piece's `review` reasons and its `equipState`, the state Equip moves it
to, so the web never re-derives step 6). `GET /api/weapons` stays as the attack's read of the weapon
pieces (with `wielded` and `speed`); `/api/weapons`' editor routes are gone. New pieces log
`item_add` / `item_edit` / `item_remove` (payload with `category`); old `weapon_*` entries pop into the
piece (step 26.2), old `items:<id>` snapshots are told from new ones by a `model` marker and read
through `item_id_renames` (step 26.3; since the action log codec, the re-encoding reads them, ADR
0018 step 9). Readings the ADR leaves open, fixed here: the review reasons are
`weapon_unmatched`, `uncategorised`, `ammo_kind_unknown` (an ammo kind none of step 4's codes,
kept as typed) and `powers_off`; an item with powers and an unknown label becomes `magic` `misc`, worn,
on the review list (so its powers stay on, step 9), one whose category cannot be worn stays carried
with `powers_off`; labels are read in English, singular or plural, `bolt` as `quarrel`, and an item
with an ammo kind is ammunition; any edit of a classifying field (category, magic kind, the category's
codes, speed, size, damage types) takes the piece off the review list, a rename does not, and popping
that edit does not put it back; the attack refuses a weapon that is not wielded (field `weaponId`);
a stored form weapon that was put away leaves the hand empty, one that is gone falls back to the first
wielded weapon. The free-text `type` label is kept as stored and read by nothing: the spells that spend an
item match the piece's `component` (step 1.1: `items.component`, `component` on the wire, optional in
a create or edit body, a patch without it keeps it; logged as an `item_edit` change and held by the
snapshots). The seeds keep their two files and are converted by the same function as the data
(`seedfile.Equipment`), so the seeded Power Gem is a `gem` (daily: never spent). Undoing a
classifying edit does not put the piece back on the review list (accepted: the review is not a game
move, step 27).

_The rolling deploy (as built)._ For the minutes both releases run after the migration (two machines,
ADR 0024 step 5.2), writes by the old release go wrong, accepted rather than coded around: a weapon it
adds or edits goes to `weapons`, which the new release never reads (lost to it); an item it adds lands
with the column defaults (`gear`, carried, no component, no review row), so new arrows are not
ammunition and a new gem is not a component until edited; its attack compares the converted, normalised
item `ammo_kind` with the raw `weapons.ammo_kind`. **The later migration that drops `weapons` first
converts the `weapons` rows absent from `items`** (by the same `itemsvc.Convert`, wielded, matched,
reviewed), so a weapon written in that window is not lost. The upgrade itself is rehearsed on a
throwaway PostgreSQL written by the release before it by `just equipment-rehearsal`
(`scripts/equipment-upgrade-rehearsal.sh`: conversion, old entries popped and redone, restarts, the
attack, Equip, the swap and the spell components).

_Step 14 carry-over (as built)._ The conversion runs once (it is the data step of its migration), so it
makes no skill armour worn. CRI-36 brings its own data step: it reads each character's `skill_armor`
and makes the matching armour piece worn (or adds it, on the review list as a guessed armour, step
27).

_Step 14 (as built, CRI-36)_: `charactersvc` derives a thief's or bard's skills on every read from the
worn body armour's Table 29 column (`charactersvc.Armour`, implemented in the composition root over
the items' store: the item service reads the characters, so the character service reads the pieces
through their DAL; `ruletablesvc.SkillArmorFor` maps the column to the skill armour Table 29 reads:
`none_allowed`, `metal` for a thief and an unknown column read `padded_studded_leather`). The
derived block (`GET /api/character`) and `GET /api/skills` carry `skillArmor`, the skill armour read
(the Skills tab names it; it is changed by wearing armour in the Inventaire, never there). The
`skillArmor` entry is gone from `POST /api/characters`, ~~`POST /api/characters/preview`~~ (since
2026-10-20 `POST /api/characters/draft`, ADR 0034) and `PATCH /api/characters/{id}` (a body naming it is an unknown field, 400) and from the creation options
(`skillArmors`); `characters.skill_armor` is no longer read or written, except by the restorer of an
old `skill_armor` entry. The carry-over is the data step of migration 020 (`carrySkillArmour`, schema:
a column comment), on `itemsvc.CarrySkillArmour`. Readings the ADR leaves open, fixed here: "the
matching armour piece" is the first **carried** armour of the entered column (none: the catalogue
piece is added; a piece worn already is kept, and its column decides); the added piece is the
catalogue row by code (`leather`, `studded_leather`, `elven_chain_mail`, `chain_mail`), its name in
every language the row has, `stock`, count 1, at the end of the list, the row's code as its id (a
free suffix if taken); the guessed pieces' review reason is `armour_guessed`; a carry-over that needs
a row a catalogue without armour lacks refuses the start, as step 26.4's conversion does. The seed
character (the database's and the memory store's) gets the same carry-over when its equipment is
seeded, so a new database holds what an upgraded one does (Kelsen, none entered: Leather armour,
worn). The class warning of a thief in armour it may not wear is step 20's (CRI-41). Tests:
`TestSkillArmorFor_Step14_TheWornArmoursColumn` (ruletablesvc),
`TestCarrySkillArmour_Step14`, `TestWornSkillColumn_Step14` (itemsvc),
`TestGetCharacter_Step14_TheSkillsReadTheWornArmour`, `TestRestorers_Step14_AnOldSkillArmourEntryStillPops`
(charactersvc), `TestCarrySkillArmour_Step14_*` (storage/postgres),
`TestSeed_Step14_TheSeedBardWearsTheArmourItEntered` (storage/memory),
`TestThief_Step14_TheSkillsFollowTheWornArmour`, `TestBard_Step14_MetalArmourReadsTheBardsColumn`,
`TestCharacters_Step14_SkillArmourIsNoEntry`, `TestBard_Step18_TheSeedBardKeepsHisSheetsPercentages`
(composition). Upgrade rehearsal: `just skill-armour-rehearsal` (`scripts/skill-armour-rehearsal.sh`:
a database written by the release before, every entered value and the bard's `metal`, a piece already
worn, a carried piece, every skill percentage compared, restarts, the old entries popped).

_Step 14 carry-over, accepted upgrade gap._ The carry-over changes the pieces without logging (it is
no move of the game), and an old `equipment_state` entry restores a piece without checking its slot.
So a take-off logged before 020 and undone after it can leave two worn body armours: a thief entered
`elven_chain` and took off its worn leather (the take-off still on the stack); 020 adds a worn Elven
chain mail; undoing the take-off puts the leather back on next to it. The AC and the skills then
read the first worn armour in the list (the rehearsal: the leather, so the sheet reads what the
player had put back on) and the player takes the other off. Accepted, not fixed: it needs a
take-off entry from before the upgrade still on the stack, and taking one piece off ends it.

_Steps 10 to 13 (as built, CRI-35)_: `acsvc` derives the AC on every read (`acsvc.Derive`, tests
`TestAC_Step10_*`, `TestAC_Step11_*`, `TestAC_Step12_*`) from the worn armour and shield
(`itemsvc`), the buffs that apply and the powers that are on (`buffsvc.ListStatus`, `ListPowers`)
and the sheet's Dexterity; served at `GET /api/ac` as rows of codes (`term`: `base`,
`armour_magic`, `shield`, `shield_magic`, `dexterity`, `protection`, `buffs`; a source `itemId` or
`buff`/`spellId`; `off`: `magic_armour`, `duplicate`, `armour_worn`, `not_best` for a row shown
that does not add). Readings the ADR leaves open, fixed here: an effect with a `weapon_kind`
condition never moves the AC (it conditions the character's own attack); a `base_ac` that does not
count is shown as not adding; the shield's own `ac` powers are their own term, `shield_magic`; the
spell seed's `ac_set` (the Armor spell, Spirit Armor) casts as `base_ac`; a seed's `condition` limits only the
modifiers its `condition_keys` names, all of them without it (Spirit Armor's and Blur's limit their
save bonus, so Spirit Armor's `ac_set` applies). The override (step 13) is
`characters.ac_override` and `ac_override_label` (migration 017), set by `PUT /api/ac/override
{value, label}` and cleared by `DELETE`, one `ac_override` entry (`{before, after}`, snapshot
`ac_override`) each, nothing logged for the same value again (`TestAC_Step13_*`). The `initiative`
effect of step 11 is left to CRI-39 (_built there, 2026-10-19_: `BuffEffects.Initiative`, JSON
`initiative`, on buffs and powers; see ADR 0035 item 6's as-built note). The step 12 duplicate rule reads the pieces' catalogue code; no
catalogue row is a magic item today (step 22), so it applies once the editor's templates (CRI-38)
give one.

_Steps 4, 24 and 25 (as built, CRI-37)_: the catalogue rows are served at `GET
/api/equipment/catalogue` (`{weapons, ammunition, armour, shields}`, codes and numbers, names as
language maps, damage as printed; empty lists before format 7); the web filters them by the name in
the player's language. `POST /api/items` with a `catalogueCode` picks: the service copies the row
(`itemsvc` `picked`) and reads only the name (a typed name replaces the languages it holds, the row's
others stay), the count (at least 1; 1 when not given, a negative one refused on field `count`, `TestCreate_Step24_PickedCountAtLeastOne`), the note and the magic of a weapon (`magicToHit`,
`magicDamage`) or ammunition (`magicalBonus`); one `item_add` entry whose payload carries
`catalogueCode`. An unknown code is refused on field `catalogueCode` with the code `unknown`
(`TestCreate_Step25_UnknownCatalogueCode`). `equip: true` equips the new piece in the same
transaction, one undo for the `item_add` and the `equipment_state` entry, a worn body armour or
shield in the way taken off (swap); a piece that cannot be equipped is refused on field `equip`
(`TestCreate_Step24_EquipNowIsOneTransaction`, `TestPick_Steps4_24_PickedBowShootsTheArrowsDiceAndEquipNowUndoesAsOne`).
Readings left open, fixed here: a launcher of several ammo kinds (the blowgun) gets the first, changed
by an edit (a piece fires one kind, step 3); the damage strings stay as printed and the attack adds
their flat part (ADR 0003 steps 1.1 and 2, ADR 0016 step 8, the supervisor's ruling of 2026-10-19);
a catalogue weapon with no damage type and no dice (the mancatcher) is valid and its attack is refused (no
dice); a typed homebrew weapon with neither is refused on `damageSm` (ADR 0016 step 8). The armour and shield "+N" (step 17) and the class warnings (step 20) are not part of the
pick. Step 4: `TestDamage_ADR0033Step4_LauncherRollsAmmunitionDice`; the attack form's preview
serves the dice pair the attack rolls (`damageSm`, `damageL`: a launcher's from its ammunition), which
the weapon-in-hand line shows (`TestGetAttackForm_ADR0033Step4_PreviewShowsTheDiceRolled`). The web
(ADR 0035 item 10): Ajouter replaces the possessions card in the stage, tabs Catalogue (first) and
Saisir; the catalogue is read once (`GET /api/equipment/catalogue`, reference data, no state key);
"Votre exemplaire" has the name, the "+N" (weapon and ammunition only), the count (ammunition) and
"Équiper tout de suite" (checked; not for ammunition).

_Step 17 (as built, CRI-38)_: `POST` and `PATCH /api/items` take `powers` in the shape `GET` serves
them (`{id, minStr?, effects}`, the buffs' effect keys): a PATCH's list replaces the stored one (`[]`
removes them, absent keeps them), a POST's (typed or picked) is the new piece's. Validation is the
seed's (`itemsvc.ValidatePowers`), refused as a field error on `powers`. Readings the ADR leaves open,
fixed here: a power sent with an empty id gets the first free `power_<n>`; the reserved `magic` power
is allowed on an armour or a shield only, as exactly one `ac` effect of -1 to -10 (`MaxBonus`), no
trigger and no condition, and an edit that makes the piece another category drops it (logged with the
edit); the picked armour or shield takes its "+N" too (ADR 0035 item 10's "Votre exemplaire"); the
`item_edit` payload carries one `powers` change whose `before` / `after` are the stored encodings
(`[]` for none), worded by the web ("magie +2", "Sauvegardes +1 · CA −1 (sans armure magique)").
The editor shows the powers for every category (each power: effects, a STR trigger, the weapon-kind
condition and `without_magic_armour`) and **Modèles** for a magic piece; armour and shields have the
"+N" stepper (0 to 10) and their other powers. The `initiative` effect of step 11 is not offered:
it does not exist before CRI-39 adds it. Tests: `TestEditor_ADR0033Step17_*` (itemsvc, composition:
the +N and a ring of protection through the aggregate, the AC rows and a pop),
`TestCreate_ADR0033Step17_PickedArmourKeepsItsPlusN`, `TestItems_ADR0033Step17_PowersInTheBody`
(router); `web/src/lib/powerEditor.test.ts`, `catalogue.test.ts`, the `item_edit` journal fixture
`editedPowers`; e2e `equipment.spec` "the editor's +N and a ring of protection template…" and
`layout.spec` "the piece editor's powers and +N uncut…".

CRI-32 (the design round) comes before the UI of slices 2 to 9. Slices 3 to 9 need slice 2; slices
5, 7 and 8 need slice 1's data.

## Consequences

- Weapons stop being a second list: the Inventaire's "Armes" card becomes a view of the wielded
  weapons (the design round decides the screens).
- The character's AC exists for the first time; nothing in the attack reads it (the AC the app
  attacks is still the target's).
- Buff and power `ac` effects, which added to nothing, now move the shown AC.
- A thief's or bard's skills now move when armour is worn or taken off, through the logged state
  change; the journal shows the state change, not a `skill_armor` entry.
- Every rule reads the possession's own copy of the numbers: a catalogue correction does not change
  a piece already owned (the player re-picks or edits it).
- Not covered: encumbrance and weight, cost and money, armour and weapon sizes against the
  character's size, shield per-attack limits (buckler 1, small shield 2 attacks), attacks from the
  rear or flanks (no shield, no Dexterity), the target's shield and Dexterity, weapon ranges beyond
  point-blank, a per-attack "thrown" option for a melee weapon that can be hurled (it stays a second
  piece or an edit of its kind), two-weapon fighting, magic item identification, charges of DMG items
  beyond ADR 0016's behaviours, intelligent weapons, the DMG item list itself.
