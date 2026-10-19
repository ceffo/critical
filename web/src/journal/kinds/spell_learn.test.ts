// The spell_learn entry's view (kinds/character.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/spell_learn.json";

describeKind("spell_learn", fixtures, {
  learned: {
    en: {
      text: "Learned: sleep (d100 37 ≤ 65%)",
    },
    fr: {
      text: "Appris : sleep (d100 37 ≤ 65 %)",
    },
    value: null,
    rolled: false,
  },
  failed: {
    en: {
      text: "Failed: sleep (d100 80 > 65%), try again at level 2",
    },
    fr: {
      text: "Échec : sleep (d100 80 > 65 %), à retenter au niveau 2",
    },
  },
});
