// The item_use entry's view (kinds/itemUse.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/item_use.json";

describeKind("item_use", fixtures, {
  rolled: {
    en: {
      text: "Used Wand of lightning: 6d6 = 21 (3 left)",
      short: "Wand of lightning",
      subtitle: "Wand of lightning",
      card: ['data-testid="item-rolls"', "6d6: 6 + 1 + 3 + 4 + 2 + 5 = 21"],
    },
    fr: {
      text: "Objet utilisé : Wand of lightning, 6d6 = 21 (3 restant)",
      short: "Wand of lightning",
      subtitle: "Wand of lightning",
      card: ['data-testid="item-total"'],
    },
    value: {
      text: "21",
      tone: "ink",
    },
    rolled: true,
  },
  lastCharge: {
    en: {
      text: "Used Potion of healing (last charge, removed)",
      short: "Potion of healing",
      subtitle: "Potion of healing",
      card: ['data-testid="item-left"'],
    },
    fr: {
      text: "Objet utilisé : Potion of healing (dernière charge, retiré)",
      short: "Potion of healing",
      subtitle: "Potion of healing",
    },
    value: null,
    rolled: true,
  },
  // ADR 0049 steps 7 and 14: an ability use names the item and the ability; a counter shows what is left.
  abilityCharges: {
    en: {
      text: "Used Wand of lightning · Lightning bolt: 6d6 = 21 (2 left)",
      short: "Wand of lightning · Lightning bolt",
      subtitle: "Wand of lightning · Lightning bolt",
      card: [
        "6d6: 6 + 1 + 3 + 4 + 2 + 5 = 21",
        "Wand of lightning · Lightning bolt: 2 left",
      ],
    },
    fr: {
      text: "Objet utilisé : Wand of lightning · Éclair, 6d6 = 21 (2 restant)",
      short: "Wand of lightning · Éclair",
      subtitle: "Wand of lightning · Éclair",
    },
    value: { text: "21", tone: "ink" },
    rolled: true,
  },
  // An unlimited ability has no counter: the journal words no "(N left)" and the card no left line.
  abilityUnlimited: {
    en: {
      text: "Used Ring of light",
      short: "Ring of light",
      subtitle: "Ring of light",
      card: ["Nothing to roll"],
    },
    fr: {
      text: "Objet utilisé : Ring of light",
      short: "Ring of light",
      subtitle: "Ring of light",
    },
    value: null,
    rolled: true,
  },
  // Every dice effect of the ability is drawn on the card, the journal's value is the first one's.
  abilityUnlimitedDice: {
    en: {
      text: "Used Rod: 1d6 = 4",
      short: "Rod",
      subtitle: "Rod",
      card: ["1d6: 4 = 4", "2d4+1: 2 + 3 + 1 = 6"],
    },
    fr: {
      text: "Objet utilisé : Rod, 1d6 = 4",
      short: "Rod",
      subtitle: "Rod",
    },
    value: { text: "4", tone: "ink" },
    rolled: true,
  },
  // ADR 0049 step 9: a scroll's spell effect: the use's text carries what the cast did (its damage
  // here), the value is the damage total, the card draws the cast's breakdown (the dice, the save).
  spellDamage: {
    en: {
      text: "Used Scroll of fireball (last charge, removed), cast Fireball: 41 fire damage",
      short: "Scroll of fireball",
      subtitle: "Scroll of fireball",
      card: [
        'data-testid="item-spell"',
        'data-testid="cast-entry"',
        "Cast Fireball",
        'data-testid="cast-entry-damage"',
      ],
    },
    fr: {
      text: "Objet utilisé : Scroll of fireball (dernière charge, retiré), lance Fireball: 41 dégâts (fire)",
      short: "Scroll of fireball",
      subtitle: "Scroll of fireball",
    },
    value: { text: "41", tone: "ink" },
    rolled: true,
  },
  // A spell that heals: the HP the ledger took is the use's heal phrase and value, once (the cast's own
  // heal phrase is not repeated after the spell's name).
  spellHeal: {
    en: {
      text: "Used Staff of curing (9 left), cast Cure Serious Wounds, heals 8 HP (15 rolled, capped at max HP)",
      short: "Staff of curing",
      subtitle: "Staff of curing",
      card: ['data-testid="item-spell"', "Cast Cure Serious Wounds"],
    },
    fr: {
      text: "Objet utilisé : Staff of curing (9 restant), lance Cure Serious Wounds, soigne 8 PV (15 obtenus, plafonnés aux PV max)",
      short: "Staff of curing",
      subtitle: "Staff of curing",
    },
    value: { text: "+8", tone: "heal" },
    rolled: true,
  },
});
