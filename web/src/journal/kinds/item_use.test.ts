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
});
