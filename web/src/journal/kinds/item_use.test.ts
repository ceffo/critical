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
});
