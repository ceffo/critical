// The time_passed entry's view (kinds/buff.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/time_passed.json";

describeKind("time_passed", fixtures, {
  passTime: {
    en: {
      text: "Time passed: 1 h (60 rd), round 3 → 0",
    },
    fr: {
      text: "Temps écoulé : 1 h (60 tr.), tour 3 → 0",
    },
    rolled: false,
  },
  // ADR 0043 step 15: the maintenance of the hour.
  passTimeMaintained: {
    en: {
      text: "Time passed: 1 h (60 rd), round 0 → 0 — maintenance (PSPs): Mind thrust 60",
    },
    fr: {
      text: "Temps écoulé : 1 h (60 tr.), tour 0 → 0 — maintien (PFP) : Mind thrust 60",
    },
  },
});
