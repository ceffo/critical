// The round_reset entry's view (kinds/buff.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/round_reset.json";

describeKind("round_reset", fixtures, {
  endCombat: {
    en: {
      text: "Combat ended: round 4 → 0",
    },
    fr: {
      text: "Combat terminé : tour 4 → 0",
    },
  },
});
