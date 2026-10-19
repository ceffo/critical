# Critical

The vocabulary of Critical, the AD&D 2e character-sheet engine for one table. One context: the
Go domain, the API, the web and the ADRs share these words. Each term gives the English name
(the one the ADRs, the code and the English UI use), the French UI word in brackets, and the ADR
that holds its rules. The glossary names things; the rules live only in the ADRs
([index](docs/adr/README.md)). The spelling is British in prose, UI and new identifiers
([ADR 0036](docs/adr/0036-vocabulary.md)).

## Language

### The character

**Character** (fr: personnage):
A player character of one of the five classes, owned by one user; its figures (level, THAC0,
saves, slots, skill percentages) are derived from class, race, abilities and XP, never entered.
[ADR 0029](docs/adr/0029-users-login-and-access.md) step 7, [ADR 0030](docs/adr/0030-classes-and-races.md).

**Character sheet** (fr: feuille de personnage):
The character's record the app keeps and shows during play: its figures, state and history.
The Google sheet the first character was imported from is not part of the domain (it was
ingestion); its figures are kept as imported and compared with nothing
([ADR 0030](docs/adr/0030-classes-and-races.md) step 24, retired).
_Avoid_: fiche, "the sheet" for the Google import.

**Player** (fr: joueur):
The person at the table playing the character. Also the ordinary user role (`player`).

**DM** (fr: MJ):
The Dungeon Master, whose rulings the player enters (the final damage, the real AC, an adjustment).
_Avoid_: GM, MD.

**Kelsen**:
The seed character: the table's level 15 bard, loaded from `seeds/character.json` on an empty
database.

**Class** (fr: classe), **Race** (fr: race), **Class group**:
The five classes (bard, fighter, mage, cleric, thief), the six PHB races and the four groups
(warrior, wizard, priest, rogue) the tables are read by. [ADR 0030](docs/adr/0030-classes-and-races.md) steps 1, 2 and 21.

**Ability score** (fr: caractéristique):
One of the six scores. The **score in effect** is the score with the app's effects applied
(today only Strength magic); the **effective Strength** feeds the attack and the Power Gem.
[ADR 0004](docs/adr/0004-buffs.md) steps 7 to 10, [ADR 0028](docs/adr/0028-ability-checks.md) step 2.

**Realized level** (fr: niveau acquis) and **Target level** (fr: niveau donné par l'XP):
The realized level is the level the character has taken; every benefit follows it. The target
level is the one the XP total gives; the gap is the **pending level-ups**. "Level" alone means
the realized level. [ADR 0030](docs/adr/0030-classes-and-races.md) steps 3 and 7.

**Level-up** (fr: passage de niveau):
Taking one pending level: the hit die rolled on the server (or the table's die entered), or the
fixed gain past the name level. [ADR 0030](docs/adr/0030-classes-and-races.md) step 7.

**XP** and the **XP ledger** (fr: XP, registre):
Experience points and the list of entries that add to the starting XP; the ledger's edits are
on the action stack. [ADR 0015](docs/adr/0015-experience.md).

**Hit points** (fr: points de vie):
**Max HP**, **current HP** and the **condition** (ok, badly wounded, unconscious, dead) derived
from them, and the HP **floor**. [ADR 0007](docs/adr/0007-hp-ledger.md),
[ADR 0011](docs/adr/0011-hp-below-zero.md).

**HP ledger**:
How current HP is derived from the action log, not a table.
[ADR 0007](docs/adr/0007-hp-ledger.md) step 2. _Avoid_: hp_events (a dead legacy table).

**THAC0** (fr: TAC0), **AC** (fr: CA):
To Hit Armour Class 0, derived from class group and level; the armour class, descending (lower is
better). The character's own AC is derived from the worn armour and effects
([ADR 0033](docs/adr/0033-equipment.md) step 10, not built yet); the target's AC is given by the DM.

**Saving throw** (fr: jet de sauvegarde), or **save**:
A d20 at or above the target of one of the five categories (paralysation/poison,
rod/staff/wand, petrification/polymorph, breath weapon, spell). [ADR 0006](docs/adr/0006-saving-throws.md).
_Avoid_: jet de protection.

### Rounds and time

**Round** (fr: tour):
The combat clock's unit, counted by hand: **Next round** (fr: Tour suivant),
**End combat** (fr: Fin du combat) and **Set round** move it. [ADR 0005](docs/adr/0005-rounds-and-time.md).
_Avoid_: turn (in English a turn is ten rounds, and "turn" means turning undead in this
project; the web's `turn` identifiers are legacy).

**Initiative** (fr: initiative):
The round's d10 rolled on Next round, lowest first; display only. [ADR 0013](docs/adr/0013-initiative.md),
to gain the weapon's speed by [ADR 0035](docs/adr/0035-equipment-screens.md) item 6.

**Pass time** (fr: heures écoulées):
Jumping ahead by hours outside combat.
[ADR 0005](docs/adr/0005-rounds-and-time.md) steps 5 to 8.

**Long rest** (fr: repos long):
The night's rest: slots unused again, daily items refilled. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) steps 11 and 12.

### Rolls

**Breakdown**:
The complete explanation a roll answers with: dice, every named **modifier**, target and
verdict. Every roll endpoint returns one; the UI words it.

**Attack** (fr: attaque):
The weapon attack: to-hit against the **target AC** (or an **unknown AC**, settled by the DM's
verdict afterwards), **critical hit** by the house **crit table**, then damage.
[ADR 0002](docs/adr/0002-attack-resolution.md), [ADR 0012](docs/adr/0012-attack-against-unknown-ac.md),
[ADR 0001](docs/adr/0001-critical-hits.md), [ADR 0003](docs/adr/0003-damage.md).

**Weapon in hand** (fr: arme en main):
The weapon the attack form attacks with; the previous one stays one click away.
Becomes a wielded weapon with [ADR 0033](docs/adr/0033-equipment.md) step 8.

**Verdict** (fr: verdict):
Hit or miss. The **DM's verdict** settles an attack against an unknown AC.

**Check** (fr: jet):
A roll judged at or under a target: the **proficiency check** (d20 under ability + modifier),
the **bard skill** and **thief skill** checks (d100 under the percentage) and the **ability
check** (d20 under the score in effect). [ADR 0014](docs/adr/0014-skill-checks.md),
[ADR 0028](docs/adr/0028-ability-checks.md). _Avoid_: test (fr).

**Proficiency** (fr: compétence diverse):
A non-weapon proficiency with its ability and modifier. **Bard skills** and **thief skills**
(fr: talents de barde, talents de voleur) are the rogue percentages derived from the entered
**skill points**, race, Dexterity and the **skill armour**. [ADR 0014](docs/adr/0014-skill-checks.md),
[ADR 0030](docs/adr/0030-classes-and-races.md) steps 17 and 18. _Avoid_: RogueSkill.

**Adjustment** (fr: ajustement):
The DM's correction of a save or check after the roll: it moves the target, + easier, - harder;
the roll never changes. [ADR 0006](docs/adr/0006-saving-throws.md) steps 5 to 8,
[ADR 0014](docs/adr/0014-skill-checks.md) steps 5 to 8.

**Dice composer** (fr: dés), **Formula**, **Favourite**:
The free roll of the dice footer, its `NdM±K` formula and the saved formulas.

### Effects

**Buff** (UI: Effect, fr: effet):
An active effect on the character: a spell received or cast, or one entered by hand. It is
**timed** (rounds left), **charged** (hits it can absorb), both, or lasts **until dismissed**.
The domain and the ADRs say buff; the UI says Effect / Effet everywhere.
[ADR 0004](docs/adr/0004-buffs.md). _Avoid_: bonus, buff (in the UI); effect (in prose, for a buff).

**Saved buff** (UI: Saved effect, fr: effet enregistré):
A buff kept as a template to activate again.

**Item power** (fr: pouvoir d'objet):
An effect an item carries, on while its trigger is met; never on the buff list.
[ADR 0016](docs/adr/0016-inventory.md) steps 9 to 16.

**Aggregate**:
The one combination of active buffs and item powers that feeds every roll: effective Strength,
to-hit, damage, saves, AC. [ADR 0004](docs/adr/0004-buffs.md) step 11.

**Received spell** (fr: sort reçu):
A spell someone else cast on the character, applied as a buff at the caster's level.
[ADR 0004](docs/adr/0004-buffs.md) step 19. _Avoid_: "spell cast on me" (fr: sort lancé sur moi).

**Spell effect data**:
What a spell does mechanically, read from `seeds/spell_effects.json`: buff, damage, exposure,
slots granted, item consumed. No per-spell code. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) step 8.
_Avoid_: "effect" alone.

### Spells

**Spell** (fr: sort):
A catalogue spell: class list, level, school or sphere, typed stats and localized texts.
[ADR 0021](docs/adr/0021-spell-catalogue.md).

**Spellbook** (fr: grimoire):
The spells a character knows; a mage or bard **learns** a wizard spell into it with the
Intelligence roll. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) step 3,
[ADR 0030](docs/adr/0030-classes-and-races.md) step 20. _Avoid_: livre de sorts.

**Spell slot** (fr: emplacement de sort):
One prepared spell, cast once until the next long rest; the slot list is the **preparation**
(fr: préparation). The **slot allotment** is the number of slots per spell level the class
gives at the realized level. An **extra slot** is one a spell granted.
[ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) steps 1, 2 and 9.

**Cast** (fr: lancer):
Spending a slot: the spell's effect data decides the buff, damage, **cast-time mode**, to-hit or
item to spend. **Spell failure** is the low-Wisdom d100. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) steps 6 to 16,
[ADR 0030](docs/adr/0030-classes-and-races.md) step 14.

**Exposure** (fr: exposition):
One round of damage to whoever stands in a spell's area, rolled on request.
[ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) step 16.

**Turn undead** (fr: renvoi des morts-vivants):
A cleric's turning roll against an undead type. [ADR 0030](docs/adr/0030-classes-and-races.md) step 16.
_Avoid_: turn (alone).

**Sphere** (fr: sphère), **School** (fr: école):
A priest spell's sphere of access; a wizard spell's school. [ADR 0030](docs/adr/0030-classes-and-races.md) step 15.

### Possessions

**Item** (fr: objet):
One possession of any kind: arrow, flask, wand, gem, and, once [ADR 0033](docs/adr/0033-equipment.md)
is built, weapon, armour and shield too. [ADR 0016](docs/adr/0016-inventory.md).
_Avoid_: piece, possession, equipment (for one item), combat item.

**Inventory** (fr: inventaire):
The character's whole list of items, and its screen. [ADR 0016](docs/adr/0016-inventory.md),
[ADR 0035](docs/adr/0035-equipment-screens.md) items 2 and 7.

**Behaviour** (fr: comportement):
How an item's **count** moves: `stock` (used and restocked), `charges` (gone at 0), `daily`
(refilled by a long rest). **Use** and **restock** are the moves. [ADR 0016](docs/adr/0016-inventory.md) steps 1 to 3.

**Ammunition** (fr: munition):
An item with an **ammo kind** a launcher fires; its magic adds to the shot. [ADR 0016](docs/adr/0016-inventory.md) step 6.

**Weapon** (fr: arme):
A possession that attacks: damage dice by target size, magic to hit and to damage, its **weapon
kind** (melee or missile; fr: mêlée / tir), its **weapon type** (the 13 codes the class rules
read, [ADR 0030](docs/adr/0030-classes-and-races.md) step 23), **specialized** for a fighter.
Today a list of its own; a **category** of the inventory after [ADR 0033](docs/adr/0033-equipment.md) step 2.

**Equipment** (fr: équipement):
The topic of [ADR 0033](docs/adr/0033-equipment.md) and the catalogue's reference data for
weapons, ammunition, armour and shields (format 7). Not a synonym of inventory or item.

**Equipped** (fr: équipé), **Equip** / **Unequip** (fr: équiper / retirer):
The UI's one word for a worn or wielded item; `worn`, `wielded` and `carried` stay the
internal states. [ADR 0033](docs/adr/0033-equipment.md) step 6, [ADR 0035](docs/adr/0035-equipment-screens.md) item 3.

**Skill armour** (fr: armure des talents):
The armour a thief's or bard's skills are adjusted for; the worn armour once ADR 0033 is built.
[ADR 0030](docs/adr/0030-classes-and-races.md) step 17, [ADR 0033](docs/adr/0033-equipment.md) step 14.

### The action log

**Action log** (UI: Journal, fr: journal):
The one append-only log of everything that happens at the table, and the HP ledger's store.
The UI shows it as the journal. [ADR 0007](docs/adr/0007-hp-ledger.md), [ADR 0010](docs/adr/0010-action-stack-and-undo.md).
_Avoid_: combat log, HP log.

**Entry** (fr: entrée):
One logged action: a **kind** (`attack`, `cast`, `hp_change`, `round_advance`…), a structured
**payload** the UI words, an **HP delta**, and **snapshots**. A **roll kind** is an entry whose
result takes the result card. [ADR 0018](docs/adr/0018-transactions-and-undo.md) step 4.

**Snapshot**, **State key**, **Restorer**:
The before and after of one piece of state an entry changed, named by its key (`buffs`,
`items:<id>`…); the restorer of that key writes the before back on undo.
[ADR 0010](docs/adr/0010-action-stack-and-undo.md) step 2.

**Transaction** (fr: action):
Everything one request logged, popped whole by one undo. [ADR 0018](docs/adr/0018-transactions-and-undo.md) step 1.

**Undo** (fr: annuler), **Redo** (fr: rétablir):
Undo pops the newest transaction (the **top**) and voids its entries; redo replays the newest
popped one. [ADR 0010](docs/adr/0010-action-stack-and-undo.md), [ADR 0018](docs/adr/0018-transactions-and-undo.md),
[ADR 0019](docs/adr/0019-redo.md). _Avoid_: void, pop (in the UI: it says Undone / Annulé).

**Correction** (fr: correction):
An audited event on an entry, never an edit: a **void** (the pop), an **amend** (the note only,
or a recomputed payload), and the **correction step** that a post-roll adjustment logs as its
own undoable entry. [ADR 0007](docs/adr/0007-hp-ledger.md) step 7, [ADR 0010](docs/adr/0010-action-stack-and-undo.md) step 6.

### Reference data

**Catalogue** (fr: catalogue):
The one JSON artifact of reference data built from the rulebooks: spells, rule tables, class
and race tables, level limits, spheres, equipment, languages; imported at start-up, never game
state. [ADR 0021](docs/adr/0021-spell-catalogue.md). _Avoid_: catalog.

**Seeds**:
The JSON files under `seeds/` that fill an empty database: spell effect data, Kelsen, his
skills, weapons and items. [ADR 0021](docs/adr/0021-spell-catalogue.md) step 2.

**Rule table** (fr: table):
A PHB or DMG table carried by the catalogue and read by the rules (Strength, Dexterity, THAC0,
saves, experience, spell progression…). [ADR 0021](docs/adr/0021-spell-catalogue.md) steps 9 to 11.

**Localized text** (fr: texte localisé):
A text in a human language, served as a language map (`{"en": …, "fr": …}`), never a
per-language field. [ADR 0023](docs/adr/0023-localized-text.md).

### Users and access

**User** (fr: utilisateur):
A person who logs in with Google; role `player` or `admin`; owns characters. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 1.
_Avoid_: account (except for the cog menu's account group).

**Admin** (fr: administrateur), **Root admin** (fr: administrateur principal):
A user who manages users and invites; the one admin the app cannot disable or demote.
[ADR 0029](docs/adr/0029-users-login-and-access.md) steps 1 and 5.

**Invite** (fr: invitation):
An email an admin authorises to log in; no email is sent. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 2.

**Session** (fr: session):
A logged-in browser, 30 days. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 4.

**Current character** (fr: personnage joué):
The one of the user's characters a request acts on; **switch** changes it, **retire**
(fr: retirer) hides a character without deleting it. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 7.

**Roster** (fr: vos personnages):
The user's list of characters, with Play, Create and Retire. [ADR 0034](docs/adr/0034-onboarding-screens.md) step 7.

### The screen

**Dashboard** (fr: tableau de bord):
The play screen: header card, character card, rail, stage and dice footer, one screen with no
page scroll. [ADR 0022](docs/adr/0022-activity-focus-layout.md) step 2. _Avoid_: "the sheet" for the screen.

**Header card**, **Character card** (fr: Personnage), **Rail**, **Stage**, **Dice footer**:
The regions of the dashboard. [ADR 0022](docs/adr/0022-activity-focus-layout.md) steps 2.2 to 2.6.

**Activity** (fr: activité):
What the stage shows: **Act** (fr: Agir), **Spells**, **Experience**, **Inventory**; the rail
switches them. [ADR 0022](docs/adr/0022-activity-focus-layout.md) steps 2.4 and 2.5.

**Now band** and **Result card** (fr: dernier résultat):
The Act stage's top: the action tabs beside the card showing the latest roll, where post-roll
adjustments are made. **Last result** is its one-line echo in the header.
[ADR 0022](docs/adr/0022-activity-focus-layout.md) step 2.5, [ADR 0027](docs/adr/0027-space-follows-frequency-of-use.md).

**Journal** (fr: journal):
The action log on screen, under the now band, resizable, collapsible to the **journal bar**.
[ADR 0022](docs/adr/0022-activity-focus-layout.md) step 5.

**Cog menu** (fr: réglages):
The settings popover of the dice footer: language, appearance, theme, account, admin.
[ADR 0034](docs/adr/0034-onboarding-screens.md) step 8.

**Intro** and **Creation flow** (fr: création):
The first screen of a user without a character and the full-screen, stepped creation.
[ADR 0034](docs/adr/0034-onboarding-screens.md) steps 1 to 6.

**Theme** (fr: thème):
One of the four colour themes, a dark and a light one per device. [ADR 0026](docs/adr/0026-contrast-ladder-raised-stage-four-themes.md).

**Fil**, **Moi** (legacy):
The two columns of the dashboard v3 of [ADR 0009](docs/adr/0009-dashboard-layout-and-theme-system.md),
replaced by [ADR 0022](docs/adr/0022-activity-focus-layout.md). They survive only as code and i18n
namespace names (`fil`, `moi`). _Avoid_ in anything new.
