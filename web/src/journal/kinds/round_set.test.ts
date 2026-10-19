// The round_set entry's view (kinds/buff.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/round_set.json";

describeKind("round_set", fixtures, {
  setRound: {
    en: {
      text: "Round set: 4 → 9",
    },
    fr: {
      text: "Tour réglé : 4 → 9",
    },
  },
});
