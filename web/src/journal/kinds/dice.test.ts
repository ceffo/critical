// The dice entry's view (kinds/dice.tsx) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/dice.json";

describeKind("dice", fixtures, {
  free: {
    en: {
      text: "Dice 2d6-d4+3 = 7",
      short: "2d6-d4+3",
      subtitle: "2d6-d4+3",
      card: ['data-testid="dice-total"'],
    },
    fr: {
      text: "Dés 2d6-d4+3 = 7",
      short: "2d6-d4+3",
      subtitle: "2d6-d4+3",
      card: ['data-testid="dice-group"'],
    },
    value: {
      text: "7",
      tone: "ink",
    },
    rolled: true,
  },
});
