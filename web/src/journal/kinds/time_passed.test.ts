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
});
