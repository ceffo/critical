// The correction entry's view (kinds/correction.ts) on its logged fixtures, in English and French.
import { describeKind } from "../kindCase";
import fixtures from "../fixtures/correction.json";

describeKind("correction", fixtures, {
  targetAc: {
    en: {
      text: "Correction AC 5 → 7",
    },
    fr: {
      text: "Correction CA 5 → 7",
    },
    value: null,
    rolled: false,
  },
  verdict: {
    en: {
      text: "Correction DM verdict none → Hit",
    },
    fr: {
      text: "Correction verdict du MJ aucun → Touché",
    },
  },
  saveAdjustment: {
    en: {
      text: "Correction save adjustment none → +2",
    },
    fr: {
      text: "Correction ajustement de sauvegarde aucun → +2",
    },
  },
});
