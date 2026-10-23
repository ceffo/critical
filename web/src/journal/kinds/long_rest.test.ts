// The long_rest entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/long_rest.json";

describeKind("long_rest", fixtures, {
  rested: {
    en: {
      text: "Long rest: 9 spell slots and 1 items refilled",
    },
    fr: {
      text: "Repos long : 9 emplacements de sort et 1 objets rechargés",
    },
    value: null,
    rolled: false,
  },
  psionicsEnded: {
    en: {
      text: "Long rest: 0 spell slots and 0 items refilled — powers ended: Mind Blank — holds ended: Orc",
    },
    fr: {
      text: "Repos long : 0 emplacements de sort et 0 objets rechargés — pouvoirs terminés : Néant spirituel — emprises terminées : Orc",
    },
    value: null,
    rolled: false,
  },
});
