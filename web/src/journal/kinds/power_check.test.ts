// The power_check entry's view (kinds/powerCheck.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/power_check.json";

describeKind("power_check", fixtures, {
  success: {
    en: {
      text: "Power check (Mind Thrust): d20 9 vs 13, success, PSPs −7",
      short: "Power check · success",
    },
    fr: {
      text: "Jet de pouvoir (Poussée mentale) : d20 9 contre 13, réussi, PFP −7",
      short: "Jet de pouvoir · réussi",
    },
    value: { text: "9", tone: "pass" },
    rolled: true,
  },
  failure: {
    en: {
      text: "Power check (Mind Thrust): d20 17 vs 14, failed, PSPs −4",
      short: "Power check · failed",
    },
    fr: {
      text: "Jet de pouvoir (Poussée mentale) : d20 17 contre 14, échec, PFP −4",
      short: "Jet de pouvoir · échec",
    },
    value: { text: "17", tone: "fail" },
  },
  enteredScore: {
    en: {
      text: "Power check (Teleport): d20 11 vs 11, power score, PSPs −12",
      short: "Power check · power score",
    },
    fr: {
      text: "Jet de pouvoir (Téléportation) : d20 11 contre 11, score de pouvoir, PFP −12",
      short: "Jet de pouvoir · score de pouvoir",
    },
    value: { text: "11", tone: "pass" },
  },
});
