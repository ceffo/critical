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
  // ADR 0037 step 20: a buff from a proficiency check is named from the proficiency.
  tumblingExpired: {
    en: {
      text: "Round 3 → 4 — expired: Bless, Tumbling",
    },
    fr: {
      text: "Tour 3 → 4 — expirés : Bless, Acrobatie",
    },
    value: null,
    rolled: false,
  },
});
