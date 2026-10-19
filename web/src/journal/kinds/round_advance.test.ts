// The round_advance entry's view (kinds/buff.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/round_advance.json";

describeKind("round_advance", fixtures, {
  nextRound: {
    en: {
      text: "Round 2 → 3 — expired: Bless",
    },
    fr: {
      text: "Tour 2 → 3 — expirés : Bless",
    },
    value: null,
    rolled: false,
  },
});
