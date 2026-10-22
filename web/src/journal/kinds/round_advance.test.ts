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
  // ADR 0043 step 14: the maintenance each power was charged.
  maintained: {
    en: {
      text: "Round 4 → 5 — maintenance (PSPs): Mind thrust 3, Body equilibrium 2",
    },
    fr: {
      text: "Tour 4 → 5 — maintien (PFP) : Mind thrust 3, Body equilibrium 2",
    },
  },
  // The power the PSPs left could not pay ends.
  maintenanceUnpaid: {
    en: {
      text: "Round 5 → 6 — maintenance (PSPs): Mind Thrust 2 — powers ended, not enough PSPs: Body Equilibrium",
    },
    fr: {
      text: "Tour 5 → 6 — maintien (PFP) : Poussée mentale 2 — pouvoirs terminés, PFP insuffisants : Équilibre corporel",
    },
  },
});
