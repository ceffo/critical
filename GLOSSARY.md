# Critical

The vocabulary of Critical, the AD&D 2e character-sheet engine for one table. One context: the
Go domain, the API, the web and the ADRs share these words. A term gives the English name (the
one the ADRs, the code and the English UI use), the French UI word in brackets, a one-line
meaning and the ADR that holds its rules. Nothing else lives here: how terms relate, the
invariants and the flow of the mechanics are in [`docs/domain-model.md`](docs/domain-model.md),
and the rules only in the ADRs ([index](docs/adr/README.md)). A PR that adds a concept updates
both files. The spelling is British in prose, UI and new identifiers
([ADR 0036](docs/adr/0036-vocabulary.md)).

A `_Code_:` line lists the exported Go domain types that are this concept's parts (`just check-glossary`
requires every exported domain type to be a term, listed on such a line, or allowlisted with a reason in
`scripts/glossary-allowlist.txt`).

Marks: _(future)_ a concept the project will model but has not yet; _(planned)_ decided and
scheduled. A French word marked _(to confirm)_ comes from a book that is not in the rulebooks
clone.

## Language

### The character

**Character** (fr: personnage):
A player character, owned by one user. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 7, [ADR 0030](docs/adr/0030-classes-and-races.md).

**Character sheet** (fr: feuille de personnage):
The record the app keeps and shows during play. [ADR 0030](docs/adr/0030-classes-and-races.md).
_Avoid_: fiche, "the sheet" for the Google import (not part of the domain).

**Player** (fr: joueur):
The person at the table playing the character; also the ordinary user role (`player`).

**DM** (fr: MJ):
The Dungeon Master, whose rulings the player enters. _Avoid_: GM, MD.

**Kelsen**:
The seed character, the table's level 15 bard, loaded from `seeds/character.json`.

**Class** (fr: classe):
What a character is trained as: bard, fighter, mage, cleric, thief today. [ADR 0030](docs/adr/0030-classes-and-races.md) steps 1 and 21, [ADR 0040](docs/adr/0040-classes-are-data.md).

**Class definition** (fr: définition de classe):
A class as catalogue data (format 11): group, prime requisites, ability minimums, alignments, XP rows, hit dice, spell progression and its class features. The app reads the group, prime requisites, minimums, alignments, XP rows, spell progression and class features so far; the rest moves with its readers. [ADR 0040](docs/adr/0040-classes-are-data.md) step 1.
_Avoid_: class table (a rule table is something else).
_Code_: Class

**Class registry** (fr: registre des classes):
The validated class, group and race definitions of the catalogue, in its order; every reader asks it for a class's group, prime requisites, minimums, alignments or XP, never a Go list of classes. Built at start-up, which refuses a broken definition. [ADR 0040](docs/adr/0040-classes-are-data.md) steps 5 and 8.
_Code_: Registry

**Class group** (fr: groupe de classes):
Warrior, wizard, priest or rogue: the grouping the tables are read by; a field of the class definition. [ADR 0030](docs/adr/0030-classes-and-races.md) step 2, [ADR 0040](docs/adr/0040-classes-are-data.md) step 1.2.

**Class entry** (fr: classe du personnage):
One class of a character with its own realized level and XP, **active** (it earns XP and levels) or **frozen** (a dual-class's old class, with the level it was left at); the class features are read through it. [ADR 0040](docs/adr/0040-classes-are-data.md) step 5, [ADR 0042](docs/adr/0042-multi-class-and-dual-class.md) step 1. _Avoid_: "class" for the entry.
_Code_: Entry
_Code_: EntryState

**Multi-class** (fr: multi-classé):
Several classes from the start (two or three active class entries), each levelling on its own, the XP split evenly between them. The user also says "dual". PHB p.43, [ADR 0042](docs/adr/0042-multi-class-and-dual-class.md).
_Avoid_: dual (for this).

**Dual-class** (fr: classes successives):
Leaving one class for a new one, the old one frozen; its abilities are used without penalty again once the new class's level passes it (the **crossover**). The user also says "serial". PHB p.43, [ADR 0042](docs/adr/0042-multi-class-and-dual-class.md).
_Avoid_: serial, multi (for this).

**Crossover** (fr: dépassement):
The moment a dual-class character's new class level passes the frozen old class's level; from then on the old class's abilities work again without penalty. PHB p.43, [ADR 0042](docs/adr/0042-multi-class-and-dual-class.md).

**Class feature** (fr: aptitude de classe):
One ability a class grants from a level, as data: a feature kind with its parameters (turning, backstab, a skill set). [ADR 0040](docs/adr/0040-classes-are-data.md) step 3. _Avoid_: power (that is a psionic word), flag.
_Code_: Feature

**Feature kind** (fr: type d'aptitude):
The mechanic a class feature drives, one of a closed list the code implements once (`turn_undead`, `rogue_skills`, `spellbook`…); the code switches on it, never on a class id. [ADR 0040](docs/adr/0040-classes-are-data.md) step 3.

**Kit** (fr: kit) _(future)_:
An optional specialisation of a class from the supplements; none is modelled.

**Prime requisite** (fr: caractéristique principale):
The ability a class is built on; 16 or more in every one gives the 10% XP bonus. [ADR 0040](docs/adr/0040-classes-are-data.md) step 6.

**Race** (fr: race):
One of the six PHB races, catalogue data like a class. [ADR 0030](docs/adr/0030-classes-and-races.md) step 21, [ADR 0040](docs/adr/0040-classes-are-data.md) step 2.

**Racial ability requirements** (fr: conditions raciales de caractéristiques):
A race's lowest and highest score in each ability (PHB Table 7). A race with them is restricted: it has its rows in every per-race table (level limits, thief skill adjustments); the human has none and no limit. [ADR 0040](docs/adr/0040-classes-are-data.md) step 8.
_Code_: AbilityRange

**Race trait** (fr: trait racial):
What a race brings; a computed one is typed, a trait kind with parameters (`con_save_bonus`, `no_exceptional_strength`), the others are text. [ADR 0040](docs/adr/0040-classes-are-data.md) step 4.

**Ability score** (fr: caractéristique):
One of the six scores; the **score in effect** has the app's effects applied.
[ADR 0004](docs/adr/0004-buffs.md) steps 7 to 10, [ADR 0028](docs/adr/0028-ability-checks.md) step 2.
_Code_: Ability, Abilities, AbilityMinimum

**Exceptional Strength** (fr: force exceptionnelle):
The percentile (18/01 to 18/00) a character of a class with the `exceptional_strength` feature rolls at Strength 18, unless its race has `no_exceptional_strength`. The server's derived data says when the creation form offers it. [ADR 0030](docs/adr/0030-classes-and-races.md) steps 9 and 10, [ADR 0040](docs/adr/0040-classes-are-data.md) step 3. _Avoid_: 18/xx as a name (it is the notation).

**Realized level** (fr: niveau acquis) and **Target level** (fr: niveau donné par l'XP):
The level taken, and the one the XP total gives; "level" alone is the realized one; the gap is the
**pending level-ups**. [ADR 0030](docs/adr/0030-classes-and-races.md) steps 3 and 7.

**Level-up** (fr: passage de niveau):
Taking one pending level. [ADR 0030](docs/adr/0030-classes-and-races.md) step 7.

**XP** and the **XP ledger** (fr: XP, registre):
Experience points, and the entries that add to the starting XP. [ADR 0015](docs/adr/0015-experience.md).
_Code_: XPLedger, LevelXP

**Hit points** (fr: points de vie):
**Max HP**, **current HP**, the **condition** (ok, badly wounded, unconscious, dead) and the HP
**floor**. [ADR 0007](docs/adr/0007-hp-ledger.md), [ADR 0011](docs/adr/0011-hp-below-zero.md).
_Code_: HPBreakdown, HPEvent, HPInput, HPLevel, HPResult, HPState

**HP ledger**:
Current HP derived from the action log, not stored. [ADR 0007](docs/adr/0007-hp-ledger.md) step 2.
_Avoid_: hp_events (a legacy table, dropped).

**THAC0** (fr: TAC0), **AC** (fr: CA):
To Hit Armour Class 0; the armour class, descending. [ADR 0002](docs/adr/0002-attack-resolution.md), [ADR 0033](docs/adr/0033-equipment.md) step 10.

**Saving throw** (fr: jet de sauvegarde), or **save**:
A d20 at or above the target of one of the five categories. [ADR 0006](docs/adr/0006-saving-throws.md).
_Avoid_: jet de protection.
_Code_: Save, SaveAdjustment, SaveBreakdown, SaveCategory, SaveEffect, SaveOptions, Saves, SaveType

**Mind-affecting** (fr: contre l'esprit):
A flag on a saving-throw request: the save is against mind-affecting magic, so Wisdom's magical
defence adjustment applies. [ADR 0006](docs/adr/0006-saving-throws.md) step 9.

**Magical defence adjustment** (fr: ajustement défensif contre la magie):
PHB Table 5's signed bonus by Wisdom (-6 to +4) on a mind-affecting save, one `wisdom` modifier on
the roll. [ADR 0006](docs/adr/0006-saving-throws.md) step 9.

**Spell immunity** (fr: immunité contre les sorts):
PHB Table 5's spells a character with Wisdom 19 or more is immune to, cumulative by score, as
catalogue spell ids. A save named against one of them rolls nothing and passes; a spell received
that is one of them applies nothing. [ADR 0006](docs/adr/0006-saving-throws.md) step 10.
_Code_: Immunity, SpellImmunities

### Rules and the table

**Ruleset** (fr: ensemble de règles):
The set of rules a game plays by: the book rules plus its house rules. [ADR 0041](docs/adr/0041-ruleset.md).

**House rule** (fr: règle maison):
A rule the table decided that differs from the books (the crit table, multi-class for every race).
_Avoid_: homebrew.

**House rule setting** (fr: réglage de règle maison):
One typed field of a ruleset's house rules (crit table, racial lift, level limits, success effects).
[ADR 0041](docs/adr/0041-ruleset.md) step 2.

**Racial lift** (fr: levée des restrictions raciales):
The house rule setting that waives the race's class restriction for a multi-class or dual-class
character (two switches, both on by default). [ADR 0041](docs/adr/0041-ruleset.md) step 2.2.
_Code_: RacialLift

**Level limits mode** (fr: mode des limites de niveau):
How the racial level limits apply; `warn` (a warning on the character, never a refusal or a cap)
is the only value. [ADR 0041](docs/adr/0041-ruleset.md) step 2.3.
_Code_: LevelLimitsMode

**Success effect** (fr: effet de réussite):
The house rule setting that says which buff a successful check of a proficiency puts on the
character (Tumbling: AC -4 for the round). [ADR 0041](docs/adr/0041-ruleset.md) step 2.4.
_Code_: SuccessEffect

**Game** / **Campaign** (fr: partie / campagne) _(future)_:
One running game of the table, with a ruleset, a party and sessions. The two words are not
settled apart.

**Game session** (fr: séance de jeu) _(future)_:
One evening of play inside a game. _Avoid_: session alone (that is a login, below).

**Party** (fr: groupe d'aventuriers) _(future)_:
The characters adventuring together in a game.

### Psionics

**Psionicist** (fr: psioniste) _(planned)_:
The psionic class of the Complete Psionics Handbook, whose powers cost PSP. [ADR 0043](docs/adr/0043-psionics.md) step 1. _Avoid_: psioniciste (not the book's word).

**PSP** (fr: PFP, points de force psionique) _(planned)_:
Psionic strength points, the pool a psionicist spends on powers: a maximum, and a ledger of deltas on the action log. [ADR 0043](docs/adr/0043-psionics.md) steps 4 to 7.

**Discipline** (fr: discipline) _(planned)_:
One of the six fields a psionic power belongs to (telepathy, say). [ADR 0043](docs/adr/0043-psionics.md) steps 8 and 9. _Avoid_: for the magic school (that is a **school**).

**Primary discipline** (fr: discipline primaire) _(planned)_:
The first discipline a psionicist picks; no other discipline may hold as many of his sciences or devotions. [ADR 0043](docs/adr/0043-psionics.md) step 9.

**Science** (fr: science) _(planned)_:
A major psionic power, learned within a discipline. [ADR 0043](docs/adr/0043-psionics.md) step 9.

**Devotion** (fr: dévotion) _(planned)_:
A minor psionic power, learned within a discipline; at least twice as many as the sciences in it. [ADR 0043](docs/adr/0043-psionics.md) step 9.

**Power score** (fr: score de pouvoir) _(planned)_:
A power's target: an ability score plus or minus the power's number. [ADR 0043](docs/adr/0043-psionics.md) step 10.

**Power check** (fr: test de pouvoir) _(planned)_:
The d20 roll at or under the power score that decides whether a psionic power works. [ADR 0043](docs/adr/0043-psionics.md) step 10. _Avoid_: check (alone).

**Initial cost** (fr: coût initial) _(planned)_:
The PSPs a power costs when it is used; half on a failed check. [ADR 0043](docs/adr/0043-psionics.md) step 11.

**Maintenance cost** (fr: coût de maintien) _(planned)_:
The PSPs a maintained power costs each round (or its period) to keep it working, charged on Next Round. [ADR 0043](docs/adr/0043-psionics.md) steps 13 to 16.

**Defence mode** (fr: mode de défense) _(future)_:
A telepathic power that defends against an attack mode in psionic combat. [ADR 0043](docs/adr/0043-psionics.md) step 20.

**Attack mode** (fr: mode d'attaque) _(future)_:
A telepathic power that attacks a mind in psionic combat. [ADR 0043](docs/adr/0043-psionics.md) step 21.

### Rounds and time

**Round** (fr: tour):
The combat clock's unit: **Next round** (fr: Tour suivant), **End combat** (fr: Fin du combat),
**Set round**. [ADR 0005](docs/adr/0005-rounds-and-time.md).
_Avoid_: turn (in English a turn is ten rounds, and "turn" means turning undead here).

**Initiative** (fr: initiative):
The round's d10, lowest first; display only. [ADR 0013](docs/adr/0013-initiative.md), [ADR 0035](docs/adr/0035-equipment-screens.md) item 6.
_Code_: InitiativeItems, InitiativeModifier, InitiativeSource

**Pass time** (fr: heures écoulées):
Jumping ahead by hours outside combat. [ADR 0005](docs/adr/0005-rounds-and-time.md) steps 5 to 8.

**Long rest** (fr: repos long):
The night's rest: slots unused again, daily items refilled. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) steps 11 and 12.

### Rolls

**Breakdown**:
The complete explanation a roll answers with: dice, every named **modifier**, target, verdict.
_Code_: BreakdownAmmo, BreakdownDie, BreakdownModifier, BreakdownRolled, BreakdownTerm

**Attack** (fr: attaque):
The weapon attack: to-hit, **critical hit**, damage. [ADR 0002](docs/adr/0002-attack-resolution.md), [ADR 0012](docs/adr/0012-attack-against-unknown-ac.md),
[ADR 0001](docs/adr/0001-critical-hits.md), [ADR 0003](docs/adr/0003-damage.md).
_Code_: AttackBreakdown, AttackForm, AttackFormPatch, AttackFormView, AttackPreview, AttackRate, AttackRateView, AttackRequest, Attacks

**Target AC**, **Unknown AC**:
The AC the DM gives for the target, or none yet, settled by the **DM's verdict**.
[ADR 0012](docs/adr/0012-attack-against-unknown-ac.md).

**Crit table**:
The house table a critical hit is read on. [ADR 0001](docs/adr/0001-critical-hits.md).

**Weapon in hand** (fr: arme en main):
The weapon the attack form attacks with. [ADR 0033](docs/adr/0033-equipment.md) step 8.
_Code_: HeldWeapon

**Verdict** (fr: verdict):
Hit or miss.

**Check** (fr: jet):
A roll judged at or under a target: **proficiency check**, **bard skill**, **thief skill**,
**ability check**. [ADR 0014](docs/adr/0014-skill-checks.md), [ADR 0028](docs/adr/0028-ability-checks.md). _Avoid_: test (fr).
_Code_: AbilityCheckResult, CheckResult

**Skills** (fr: compétences):
Umbrella for what a character knows how to do, owned by one module. [ADR 0037](docs/adr/0037-proficiencies.md) amendment of 2026-10-20.
_Avoid_: spends.

**Proficiency** (fr: compétence diverse):
A non-weapon proficiency with its ability and modifier. [ADR 0014](docs/adr/0014-skill-checks.md). _Avoid_: RogueSkill.
_Code_: Proficiencies, ProficiencyDescription

**Bard skills**, **Thief skills** (fr: talents de barde, talents de voleur), **Skill points**:
The rogue percentages and the points entered for them, read through the class's `rogue_skills` feature: its skill set, budget and armour entries. [ADR 0030](docs/adr/0030-classes-and-races.md) steps 17 and 18, [ADR 0040](docs/adr/0040-classes-are-data.md) step 3.
_Code_: RogueSkills, SkillBudget

**Skill set** (fr: jeu de talents):
One `rogue_skills` feature a class grants and what comes of it on a character: its skill list, its own budget, skill armour and stored points, apart from any other set the character has (a thief/bard has two, and both list pick pockets). The client renders the list of sets, never a class. [ADR 0040](docs/adr/0040-classes-are-data.md) step 3.5.
_Code_: SkillSet, SetPoints, SetSkill

**Proficiency slot** (fr: emplacement de compétence):
One unit of the proficiency budget, **weapon** or **non-weapon**; **over budget** or **to spend**.
[ADR 0037](docs/adr/0037-proficiencies.md) steps 3, 10 and 11.
_Code_: ProficiencySlots

**Pick** (fr: choix):
One proficiency the character has, with the slots spent on it. [ADR 0037](docs/adr/0037-proficiencies.md) steps 2, 6, 7 and 12.
_Code_: Picks, PickFigures

**Weapon proficiency** (fr: compétence martiale):
A pick of a weapon type, with the **non-proficiency penalty** (fr: malus de non-maîtrise) otherwise;
**specialization** (fr: spécialisation) is more slots. [ADR 0037](docs/adr/0037-proficiencies.md) steps 7 to 9.

**Adjustment** (fr: ajustement):
The DM's correction of a save or check after the roll. [ADR 0006](docs/adr/0006-saving-throws.md) steps 5 to 8, [ADR 0014](docs/adr/0014-skill-checks.md) steps 5 to 8.

**Dice composer** (fr: dés), **Formula**, **Favourite**:
The free roll of the dice footer, its `NdM±K` formula and the saved formulas.

### Effects

**Buff** (UI: Effect, fr: effet):
An active effect on the character, **timed**, **charged**, both, or **until dismissed**. [ADR 0004](docs/adr/0004-buffs.md).
_Avoid_: bonus, buff (in the UI); effect (in prose, for a buff).
_Code_: AppliedBuff, BuffEdit, BuffEffect, BuffEffects, BuffModifiers, Buffs, BuffStatus, CastBuff, SuccessBuff

**Saved buff** (UI: Saved effect, fr: effet enregistré):
A buff kept as a template.

**Item power** (fr: pouvoir d'objet):
An effect an item carries while its trigger is met. [ADR 0016](docs/adr/0016-inventory.md) steps 9 to 16.

**Aggregate**:
The one combination of buffs and item powers that feeds every roll. [ADR 0004](docs/adr/0004-buffs.md) step 11.

**Received spell** (fr: sort reçu):
A spell someone else cast on the character. [ADR 0004](docs/adr/0004-buffs.md) step 19. _Avoid_: sort lancé sur moi.

**Spell effect data**:
What a spell does mechanically, read from `seeds/spell_effects.json`. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) step 8.
_Avoid_: "effect" alone.

### Spells

**Spell** (fr: sort):
A catalogue spell. [ADR 0021](docs/adr/0021-spell-catalogue.md).
_Code_: Spells, SpellCatalogue, SpellFailure, SpellLevelCount, SpellName, SpellOption

**Spellbook** (fr: grimoire):
The spells a character knows. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) step 3, [ADR 0030](docs/adr/0030-classes-and-races.md) step 20.
_Avoid_: livre de sorts.
_Code_: SpellbookEntry, SpellbookOption, SpellbookRules, StartingBook, StartingSpellbook

**Learn** (fr: apprendre) / **Add** (fr: ajouter):
Write a spell into the spellbook with the Intelligence roll, or with none; each entry keeps its
**origin**. [ADR 0030](docs/adr/0030-classes-and-races.md) step 20.

**Spell slot** (fr: emplacement de sort):
One prepared spell, cast once until the next long rest; the **preparation** (fr: préparation) is
the slot list, the **slot allotment** the class's number per level, an **extra slot** a granted one.
[ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) steps 1, 2 and 9.
_Code_: Slots, GrantSlots

**Cast** (fr: lancer):
Spending a slot; **spell failure** is the low-Wisdom d100. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) steps 6 to 16,
[ADR 0030](docs/adr/0030-classes-and-races.md) step 14.
_Code_: CastAttack, CastBreakdown, CastDamage, CastDamagePart, CastFailure, CastInputs, CastItem, CastOn, CastRequest, CastResult, CastSave, CastTiming, CastUntracked, CastingTime

**Exposure** (fr: exposition):
One round of damage to whoever stands in a spell's area. [ADR 0008](docs/adr/0008-spell-slots-and-long-rest.md) step 16.
_Code_: ExposureAt, ExposureAtWire, ExposureBreakdown, ExposureRequest, ExposureResult

**Turn undead** (fr: renvoi des morts-vivants):
A cleric's turning roll against an undead type. [ADR 0030](docs/adr/0030-classes-and-races.md) step 16. _Avoid_: turn (alone).
_Code_: TurningTarget

**Sphere** (fr: sphère), **School** (fr: école):
A priest spell's sphere of access; a wizard spell's school. [ADR 0030](docs/adr/0030-classes-and-races.md) step 15.
_Code_: SphereAccess, Spheres

### Possessions

**Item** (fr: objet):
One possession of any kind. [ADR 0016](docs/adr/0016-inventory.md), [ADR 0033](docs/adr/0033-equipment.md).
_Avoid_: piece, possession, equipment (for one item), combat item.
_Code_: Items, ItemInput, ItemPatch, ConsumedItem, ConsumeItem

**Inventory** (fr: inventaire):
The character's list of items, and its screen. [ADR 0016](docs/adr/0016-inventory.md), [ADR 0035](docs/adr/0035-equipment-screens.md) items 2 and 7.

**Behaviour** (fr: comportement):
How an item's count moves: `stock`, `charges`, `daily`; **use** and **restock**. [ADR 0016](docs/adr/0016-inventory.md) steps 1 to 3.

**Ammunition** (fr: munition):
An item a launcher fires, by **ammo kind**. [ADR 0016](docs/adr/0016-inventory.md) step 6.
_Code_: AmmoChoice, AmmoUse

**Weapon** (fr: arme):
A possession that attacks, by **weapon kind** (melee or missile; fr: mêlée / tir) and **weapon
type**. [ADR 0030](docs/adr/0030-classes-and-races.md) step 23, [ADR 0037](docs/adr/0037-proficiencies.md) step 9.
_Code_: Weapons, WeaponKind, WeaponPool, WeaponTypeOption

**Equipment** (fr: équipement):
The topic of [ADR 0033](docs/adr/0033-equipment.md) and the catalogue's reference data for gear. Not a synonym of inventory or item.

**Equipped** (fr: équipé), **Equip** / **Unequip** (fr: équiper / retirer):
The UI's word for a worn or wielded item. [ADR 0033](docs/adr/0033-equipment.md) step 6, [ADR 0035](docs/adr/0035-equipment-screens.md) item 3.

**Skill armour** (fr: armure des talents):
The Table 29 column of the body armour worn. [ADR 0030](docs/adr/0030-classes-and-races.md) step 17, [ADR 0033](docs/adr/0033-equipment.md) step 14.
_Code_: SkillArmorRow

### The action log

**Action log** (UI: Journal, fr: journal):
The append-only log of everything that happens at the table. [ADR 0007](docs/adr/0007-hp-ledger.md), [ADR 0010](docs/adr/0010-action-stack-and-undo.md).
_Avoid_: combat log, HP log.

**Entry** (fr: entrée):
One logged action: a **kind**, a **payload**, an **HP delta**, **snapshots**; a **roll kind** shows the result card.
[ADR 0018](docs/adr/0018-transactions-and-undo.md) step 4.

**Snapshot**, **State key**, **Restorer**:
The before and after of one piece of state, its key, and what writes the before back.
[ADR 0010](docs/adr/0010-action-stack-and-undo.md) step 2.

**Transaction** (fr: action):
Everything one request logged, popped whole by one undo. [ADR 0018](docs/adr/0018-transactions-and-undo.md) step 1.

**Undo** (fr: annuler), **Redo** (fr: rétablir):
Pop the newest transaction (the **top**), or replay the newest popped one.
[ADR 0010](docs/adr/0010-action-stack-and-undo.md), [ADR 0018](docs/adr/0018-transactions-and-undo.md), [ADR 0019](docs/adr/0019-redo.md).
_Avoid_: void, pop (in the UI).

**Correction** (fr: correction):
An audited event on an entry, never an edit: **void**, **amend**, **correction step**.
[ADR 0007](docs/adr/0007-hp-ledger.md) step 7, [ADR 0010](docs/adr/0010-action-stack-and-undo.md) step 6.

### Reference data

**Catalogue** (fr: catalogue):
The one JSON artifact of reference data built from the rulebooks. [ADR 0021](docs/adr/0021-spell-catalogue.md).
_Avoid_: catalog.

**Seeds**:
The JSON files under `seeds/` that fill an empty database. [ADR 0021](docs/adr/0021-spell-catalogue.md) step 2.

**Rule table** (fr: table):
A PHB or DMG table carried by the catalogue. [ADR 0021](docs/adr/0021-spell-catalogue.md) steps 9 to 11.
_Code_: ClassTables, SkillTables, Tables

**Localized text** (fr: texte localisé):
A text in a human language, served as a language map. [ADR 0023](docs/adr/0023-localized-text.md).

### Users and access

**User** (fr: utilisateur):
A person who logs in with Google; role `player` or `admin`. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 1.
_Avoid_: account (except for the cog menu's account group).

**Admin** (fr: administrateur), **Root admin** (fr: administrateur principal):
A user who manages users and invites; the one admin the app cannot disable. [ADR 0029](docs/adr/0029-users-login-and-access.md) steps 1 and 5.

**Invite** (fr: invitation):
An email an admin authorises to log in. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 2.

**Session** (fr: session):
A logged-in browser, 30 days. [ADR 0029](docs/adr/0029-users-login-and-access.md) step 4. Not a **game session**.

**Current character** (fr: personnage joué):
The one a request acts on; **switch** and **retire** (fr: retirer). [ADR 0029](docs/adr/0029-users-login-and-access.md) step 7.

**Roster** (fr: vos personnages):
The user's list of characters. [ADR 0034](docs/adr/0034-onboarding-screens.md) step 7.

### The screen

**Dashboard** (fr: tableau de bord):
The play screen. [ADR 0022](docs/adr/0022-activity-focus-layout.md) step 2. _Avoid_: "the sheet" for the screen.

**Header card**, **Character card** (fr: Personnage), **Rail**, **Stage**, **Dice footer**:
The regions of the dashboard. [ADR 0022](docs/adr/0022-activity-focus-layout.md) steps 2.2 to 2.6.

**Activity** (fr: activité):
What the stage shows: **Act** (fr: Agir), **Spells**, **Experience**, **Inventory**.
[ADR 0022](docs/adr/0022-activity-focus-layout.md) steps 2.4 and 2.5.

**Now band** and **Result card** (fr: dernier résultat):
The Act stage's top, and the card of the latest roll; **Last result** is its header echo.
[ADR 0022](docs/adr/0022-activity-focus-layout.md) step 2.5, [ADR 0027](docs/adr/0027-space-follows-frequency-of-use.md).

**Journal** (fr: journal):
The action log on screen. [ADR 0022](docs/adr/0022-activity-focus-layout.md) step 5.

**Cog menu** (fr: réglages):
The settings popover of the dice footer. [ADR 0034](docs/adr/0034-onboarding-screens.md) step 8.

**Intro** and **Creation flow** (fr: création):
The first screen of a user without a character, and the stepped creation. [ADR 0034](docs/adr/0034-onboarding-screens.md) steps 1 to 6.

**Theme** (fr: thème):
One of the four colour themes. [ADR 0026](docs/adr/0026-contrast-ladder-raised-stage-four-themes.md).

**Fil**, **Moi** (retired):
The two columns of [ADR 0009](docs/adr/0009-dashboard-layout-and-theme-system.md)'s dashboard, replaced by
[ADR 0022](docs/adr/0022-activity-focus-layout.md). _Avoid_ in anything new.
